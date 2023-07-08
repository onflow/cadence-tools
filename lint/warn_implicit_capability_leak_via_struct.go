package lint

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

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

var ImplicitCapabilityLeakViaStruct = (func() *analysis.Analyzer {

	return &analysis.Analyzer{
		Description: "Detects potential capability leak via public struct",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			location := pass.Program.Location
			report := pass.Report

			var structIdentifier *string
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

								nd, ok := fd.TypeAnnotation.Type.(*ast.NominalType)
								if !ok {
									return
								}

								if nd.Identifier.Identifier == "Capability" {
									structIdentifier = &declaration.Identifier.Identifier
								}
							}
						}
					}
				},
			)

			if structIdentifier != nil {
				inspector.Preorder(
					[]ast.Element{(*ast.FieldDeclaration)(nil)},
					func(element ast.Element) {
						switch declaration := element.(type) {
						case *ast.FieldDeclaration:
							{
								nt, ok := declaration.TypeAnnotation.Type.(*ast.NominalType)
								if !ok {
									return
								}
								
								if declaration.Access != ast.AccessPublic {
									return
								}

								if nt.Identifier.Identifier != *structIdentifier {
									return
								}

								report(
									analysis.Diagnostic{
										Location: location,
										Range:    ast.NewRangeFromPositioned(nil, element),
										Category: ReplacementCategory,
										Message:  "capability might be leaking via public struct field",
									},
								)
							}
						}
					},
				)
			}

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"implicit-capability-leak-through-struct",
		ImplicitCapabilityLeakViaStruct,
	)
}
