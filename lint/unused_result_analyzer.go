/*
 * Cadence lint - The Cadence linter
 *
 * Copyright Flow Foundation
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
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/tools/analysis"
)

var UnusedResultAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.ExpressionStatement)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects unused results of expressions",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			elaboration := program.Checker.Elaboration
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {

					expressionStatement, ok := element.(*ast.ExpressionStatement)
					if !ok {
						return
					}

					ty := elaboration.ExpressionTypes(expressionStatement.Expression).ActualType
					switch ty {
					case nil, sema.VoidType, sema.NeverType:
						// NO-OP
					default:
						report(
							analysis.Diagnostic{
								Location: location,
								Range:    ast.NewRangeFromPositioned(nil, element),
								Category: UnusedResultCategory,
								Message:  "unused result",
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
		"unused-result",
		UnusedResultAnalyzer,
	)
}
