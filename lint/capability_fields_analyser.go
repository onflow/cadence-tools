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

func DetectCapabilityType(typeToCheck ast.Type) bool {
	const capabilityTypeName = "Capability"
	switch downcastedType := typeToCheck.(type) {
	case *ast.NominalType:
		return downcastedType.Identifier.Identifier == capabilityTypeName
	case *ast.OptionalType:
		return DetectCapabilityType(downcastedType.Type)
	case *ast.VariableSizedType:
		return DetectCapabilityType(downcastedType.Type)
	case *ast.ConstantSizedType:
		return DetectCapabilityType(downcastedType.Type)
	case *ast.DictionaryType:
		return DetectCapabilityType(downcastedType.KeyType) || DetectCapabilityType(downcastedType.ValueType)
	case *ast.FunctionType:
		return false
	case *ast.ReferenceType:
		return DetectCapabilityType(downcastedType.Type)
	case *ast.RestrictedType:
		return false
	case *ast.InstantiationType:
		return DetectCapabilityType(downcastedType.Type)
	default:
		panic("Unknown type")
	}
}

func CollectCompositeIdentifiers(inspector *ast.Inspector) (map[string]bool, map[ast.Identifier]bool) {
	compositeIdsCapabilitiesPropery := make(map[string]bool)
	fieldsInStruct := make(map[ast.Identifier]bool)
	inspector.Preorder(
		[]ast.Element{(*ast.CompositeDeclaration)(nil)},
		func(element ast.Element) {
			switch declaration := element.(type) {
			case *ast.CompositeDeclaration:
				{
					if declaration.CompositeKind != common.CompositeKindStructure {
						return
					}
					for _, d := range declaration.Members.Declarations() {
						fd, ok := d.(*ast.FieldDeclaration)
						if !ok {
							return
						}
						if fd.Access != ast.AccessPublic {
							return
						}
						if fd.Access == ast.AccessPublic && DetectCapabilityType(fd.TypeAnnotation.Type) {
							fieldsInStruct[fd.Identifier] = true
							compositeIdsCapabilitiesPropery[declaration.Identifier.Identifier] = true
						}
					}
				}
			}
		},
	)
	return compositeIdsCapabilitiesPropery, fieldsInStruct
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
			compositeIdsCapabilitiesPropery, fieldsInStruct := CollectCompositeIdentifiers(inspector)

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {

					field, ok := element.(*ast.FieldDeclaration)
					if !ok {
						return
					}

					nt, ok := field.TypeAnnotation.Type.(*ast.NominalType)
					if ok {
						hasCapability, found := compositeIdsCapabilitiesPropery[nt.Identifier.Identifier]

						if found {
							if hasCapability && field.Access == ast.AccessPublic {
								report(
									analysis.Diagnostic{
										Location:         location,
										Range:            ast.NewRangeFromPositioned(nil, element),
										Category:         UpdateCategory,
										Message:          "It is an anti-pattern to have public Capability fields.",
										SecondaryMessage: "Consider restricting access.",
									},
								)
								return
							}
						}
					}
					_, found := fieldsInStruct[field.Identifier]
					if found {
						//if its in a struct it is treated by another check
						return
					}

					if field.Access == ast.AccessPublic && DetectCapabilityType(field.TypeAnnotation.Type) {
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
