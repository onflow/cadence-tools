/*
 * Cadence-lint - The Cadence linter
 *
 * Copyright 2019-2023 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lint

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

func DetectCapabilityType(typeToCheck ast.Type, compositesWithPubCapabilities map[string]struct{}) bool {
	const capabilityTypeName = "Capability"
	switch downcastedType := typeToCheck.(type) {
	case *ast.NominalType:
		_, found := compositesWithPubCapabilities[downcastedType.Identifier.Identifier]
		return found || downcastedType.Identifier.Identifier == capabilityTypeName
	case *ast.OptionalType:
		return DetectCapabilityType(downcastedType.Type, compositesWithPubCapabilities)
	case *ast.VariableSizedType:
		return DetectCapabilityType(downcastedType.Type, compositesWithPubCapabilities)
	case *ast.ConstantSizedType:
		return DetectCapabilityType(downcastedType.Type, compositesWithPubCapabilities)
	case *ast.DictionaryType:
		return DetectCapabilityType(downcastedType.KeyType, compositesWithPubCapabilities) || DetectCapabilityType(downcastedType.ValueType, compositesWithPubCapabilities)
	case *ast.FunctionType:
		return false
	case *ast.ReferenceType:
		return DetectCapabilityType(downcastedType.Type, compositesWithPubCapabilities)
	case *ast.RestrictedType:
		return false
	case *ast.InstantiationType:
		return DetectCapabilityType(downcastedType.Type, compositesWithPubCapabilities)
	default:
		panic("Unknown type")
	}
}

func CollectCompositesWithPublicCapabilities(inspector *ast.Inspector) (map[string]struct{}, map[ast.Identifier]struct{}) {
	compositesWithPubCapabilities := make(map[string]struct{})
	fieldsInComposite := make(map[ast.Identifier]struct{})
	inspector.Preorder(
		[]ast.Element{(*ast.CompositeDeclaration)(nil)},
		func(element ast.Element) {
			switch declaration := element.(type) {
			case *ast.CompositeDeclaration:
				{
					for _, d := range declaration.Members.Declarations() {
						field, ok := d.(*ast.FieldDeclaration)
						if !ok || field.Access != ast.AccessPublic {
							return
						}
						if DetectCapabilityType(field.TypeAnnotation.Type, compositesWithPubCapabilities) {
							if declaration.CompositeKind != common.CompositeKindContract {
								// public capability fields in contracts are not included in this set as later
								// on it is used to exclude false positive. And while a struct with a public capability
								// can be a false positive a contract with a public capability is always an anti pattern.
								fieldsInComposite[field.Identifier] = struct{}{}
							}

							compositesWithPubCapabilities[declaration.Identifier.Identifier] = struct{}{}
						}
					}
				}
			}
		},
	)
	return compositesWithPubCapabilities, fieldsInComposite
}

var CapabilityFieldAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.FieldDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects public fields with Capability type",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)
			location := pass.Program.Location
			report := pass.Report
			structTypesPublicCapability, fieldsInStruct := CollectCompositesWithPublicCapabilities(inspector)

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					field, ok := element.(*ast.FieldDeclaration)
					if !ok {
						return
					}
					_, found := fieldsInStruct[field.Identifier]
					if found {
						return
					}
					if field.Access == ast.AccessPublic && DetectCapabilityType(field.TypeAnnotation.Type, structTypesPublicCapability) {
						report(
							analysis.Diagnostic{
								Location:         location,
								Range:            ast.NewRangeFromPositioned(nil, element),
								Category:         UpdateCategory,
								Message:          "It is an anti-pattern to have public Capability fields.",
								SecondaryMessage: "Consider restricting access.",
							},
						)

					}

				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"public-capability-field",
		CapabilityFieldAnalyzer,
	)
}
