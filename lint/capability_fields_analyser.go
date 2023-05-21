/*
 * Cadence-lint - The Cadence linter
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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
	"github.com/onflow/cadence/tools/analysis"
)

func DetectCapabilityType(typeToCheck ast.Type) bool {
	const capabilityTypeLiteral = "Capability"
	switch upcastedType := typeToCheck.(type) {
	case *ast.NominalType:
		return upcastedType.Identifier.Identifier == capabilityTypeLiteral
	case *ast.OptionalType:
		return DetectCapabilityType(upcastedType.Type)
	case *ast.VariableSizedType:
		return DetectCapabilityType(upcastedType.Type)
	case *ast.ConstantSizedType:
		return DetectCapabilityType(upcastedType.Type)
	case *ast.DictionaryType:
		return DetectCapabilityType(upcastedType.KeyType) || DetectCapabilityType(upcastedType.ValueType)
	case *ast.FunctionType:
		return false
	case *ast.ReferenceType:
		return DetectCapabilityType(upcastedType.Type)
	case *ast.RestrictedType:
		return false
	case *ast.InstantiationType:
		return DetectCapabilityType(upcastedType.Type)
	default:
		panic("Unknown type")
	}
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

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {

					field, ok := element.(*ast.FieldDeclaration)
					if !ok {
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
