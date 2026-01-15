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
	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/tools/analysis"
)

var RedundantTypeAnnotationAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.VariableDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects unnecessary cast expressions",
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

					variableDeclaration, ok := element.(*ast.VariableDeclaration)
					if !ok {
						return
					}

					typeAnnotation := variableDeclaration.TypeAnnotation
					if typeAnnotation == nil {
						return
					}

					variableDeclarationTypes := elaboration.VariableDeclarationTypes(variableDeclaration)

					if variableDeclarationTypes.ValueActualType != nil && isRedundantCast(
						variableDeclaration.Value,
						variableDeclarationTypes.ValueActualType,
						variableDeclarationTypes.TargetType,
						// There is never "another" expected type without the type annotation
						// of the variable declaration.
						// In a casting expression, the expected type passed here is not the target type of the cast,
						// but it is the expected type of the casting expression itself (if any).
						nil,
					) {

						typeAnnotationRangeIncludingColon := ast.Range{
							StartPos: variableDeclaration.Identifier.EndPosition(nil).Shifted(nil, 1),
							EndPos:   typeAnnotation.EndPosition(nil),
						}

						report(
							analysis.Diagnostic{
								Location: location,
								Range:    typeAnnotationRangeIncludingColon,
								Category: UnnecessaryTypeAnnotationCategory,
								Message:  "type annotation is redundant, type can be inferred",
								SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
									{
										Message: "Remove redundant type annotation",
										TextEdits: []ast.TextEdit{
											{
												Replacement: "",
												Range:       typeAnnotationRangeIncludingColon,
											},
										},
									},
								},
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
		"redundant-type-annotation",
		RedundantTypeAnnotationAnalyzer,
	)
}
