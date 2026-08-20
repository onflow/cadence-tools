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
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

var DictionaryKeysIterationAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.ForStatement)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects unnecessary '.keys' when iterating over a dictionary",
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
					forStmt, ok := element.(*ast.ForStatement)
					if !ok {
						return
					}

					// Indexed iteration (for i, key in ...) requires .keys
					if forStmt.Index != nil {
						return
					}

					memberExpr, ok := forStmt.Value.(*ast.MemberExpression)
					if !ok {
						return
					}

					if memberExpr.Identifier.Identifier != "keys" {
						return
					}

					if memberExpr.Optional {
						return
					}

					memberInfo, ok := elaboration.MemberExpressionMemberAccessInfo(memberExpr)
					if !ok {
						return
					}

					if _, isDictType := memberInfo.AccessedType.(*sema.DictionaryType); !isDictType {
						return
					}

					dotKeysRange := ast.Range{
						StartPos: memberExpr.Expression.EndPosition(nil).Shifted(nil, 1),
						EndPos:   memberExpr.EndPosition(nil),
					}

					newDiagnostic(
						location,
						report,
						"unnecessary '.keys': iterating over a dictionary directly yields its keys",
						dotKeysRange,
					).
						WithCategory(ReplacementCategory).
						WithSimpleReplacement("").
						Report()
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"dictionary-keys-iteration",
		DictionaryKeysIterationAnalyzer,
	)
}
