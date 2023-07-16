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

var ImplicitCapabilityLeak = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.FieldDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects potential capability leak via public fields",
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
					fieldDeclaration, ok := element.(*ast.FieldDeclaration)
					if !ok {
						return
					}
					if fieldDeclaration.DeclarationAccess() != ast.AccessPublic {
						return
					}

					arr, ok := fieldDeclaration.TypeAnnotation.Type.(*ast.VariableSizedType)
					if !ok {
						return
					}
					nominalType, ok := arr.Type.(*ast.NominalType)
					if !ok {
						return
					}
					if nominalType.Identifier.Identifier != "Capability" {
						return
					}

					report(
						analysis.Diagnostic{
							Location: location,
							Range:    ast.NewRangeFromPositioned(nil, element),
							Category: ReplacementCategory,
							Message:  "capability might be leaking",
						},
					)
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"implicit-capability-leak",
		ImplicitCapabilityLeak,
	)
}
