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
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

func reportComplexDestructor(
	destructor *ast.SpecialFunctionDeclaration,
	location common.Location,
) *analysis.Diagnostic {
	return &analysis.Diagnostic{
		Location:         location,
		Range:            ast.NewRangeFromPositioned(nil, destructor),
		Category:         UpdateCategory,
		Message:          "complex destructor found",
		SecondaryMessage: destructor.String(),
	}
}

func isComplexDestructor(
	statements []ast.Statement,
) bool {

	for _, statement := range statements {
		switch statement := statement.(type) {
		case *ast.ReturnStatement:
			continue
		case *ast.ExpressionStatement:
			switch statement.Expression.(type) {
			case *ast.DestroyExpression:
				continue
			default:
				return true
			}
		default:
			return true
		}
	}

	return false
}

var ComplexDestructorAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.SpecialFunctionDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects complex destructors.",
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

					var diagnostic *analysis.Diagnostic

					switch expr := element.(type) {
					case *ast.SpecialFunctionDeclaration:
						function := expr.FunctionDeclaration
						if function.Identifier.Identifier != "destroy" {
							return
						}

						// find any non-destroy statements
						if isComplexDestructor(expr.FunctionDeclaration.FunctionBlock.Block.Statements) {
							diagnostic = reportComplexDestructor(expr, location)
						}

					default:
						return
					}

					if diagnostic != nil {
						report(*diagnostic)
					}
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"complex-destructor",
		ComplexDestructorAnalyzer,
	)
}
