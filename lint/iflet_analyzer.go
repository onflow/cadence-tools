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
	"github.com/onflow/cadence/tools/analysis"
)

var IfLetAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.IfStatement)(nil),
		(*ast.ForceExpression)(nil),
		(*ast.FunctionExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects if statements with nil checks followed by force-unwraps that should use if-let",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			report := pass.Report

			// Track nil-checked expressions per scope
			// Each function expression creates a new scope to handle parameter shadowing
			type scope struct {
				// Track nil-checked expressions in scope.
				// Use the string representation as the key for easy comparison.
				// Use a counter to handle nested if statements checking the same expression.
				nilCheckedExprs map[string]int
				// Track which nil-checked expressions we've already reported
				// to avoid duplicate diagnostics (only report the first occurrence)
				reportedExprs map[string]struct{}
			}

			var scopeStack []scope
			pushScope := func() {
				scopeStack = append(
					scopeStack,
					scope{
						nilCheckedExprs: map[string]int{},
						reportedExprs:   map[string]struct{}{},
					},
				)
			}
			pushScope()

			inspector.Elements(
				elementFilter,
				func(element ast.Element, push bool) (cont bool) {
					// By default, continue traversing the AST
					cont = true

					switch elem := element.(type) {
					case *ast.FunctionExpression:
						if push {
							pushScope()
						} else {
							scopeStack = scopeStack[:len(scopeStack)-1]
						}

					case *ast.IfStatement:
						currentScope := &scopeStack[len(scopeStack)-1]

						if push {
							// Entering an if statement.
							// Track the nil-checked expression, if any
							nilCheckedExpr := getNilCheckedExpression(elem)
							if nilCheckedExpr != nil {
								key := nilCheckedExpr.String()
								currentScope.nilCheckedExprs[key] += 1
							}
						} else {
							// Exiting an if statement.
							// Remove the nil-checked expression
							if len(currentScope.nilCheckedExprs) > 0 {
								nilCheckedExpr := getNilCheckedExpression(elem)
								if nilCheckedExpr != nil {
									key := nilCheckedExpr.String()
									currentScope.nilCheckedExprs[key] -= 1
								}
							}
						}

					case *ast.ForceExpression:
						if !push {
							return
						}

						// Check only the current scope
						currentScope := &scopeStack[len(scopeStack)-1]

						// Check if this force-unwrap's expression matches any nil-checked expression
						key := elem.Expression.String()
						if currentScope.nilCheckedExprs[key] > 0 {
							// Only report if we haven't already reported this expression in this scope
							if _, ok := currentScope.reportedExprs[key]; !ok {
								report(
									analysis.Diagnostic{
										Location: location,
										Range:    ast.NewRangeFromPositioned(nil, elem),
										Category: IfLetHintCategory,
										Message:  "consider using if-let instead of nil check followed by force-unwrap",
										URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
									},
								)
								currentScope.reportedExprs[key] = struct{}{}
							}
						}
					}

					return
				},
			)

			return nil
		},
	}
})()

func getNilCheckedExpression(ifStmt *ast.IfStatement) ast.Expression {
	binaryExpr, ok := ifStmt.Test.(*ast.BinaryExpression)
	if !ok || binaryExpr.Operation != ast.OperationNotEqual {
		return nil
	}

	var checkedExpr ast.Expression

	if _, ok := binaryExpr.Right.(*ast.NilExpression); ok {
		checkedExpr = binaryExpr.Left
	} else if _, ok := binaryExpr.Left.(*ast.NilExpression); ok {
		checkedExpr = binaryExpr.Right
	} else {
		return nil
	}

	// Only consider some expressions that are unlikely to have changed for now
	// TODO: improve analysis to track writes and ensure the expression hasn't changed
	switch checkedExpr.(type) {
	case *ast.IdentifierExpression,
		*ast.MemberExpression,
		*ast.IndexExpression:

		return checkedExpr
	}

	return nil
}

func init() {
	RegisterAnalyzer(
		"if-let",
		IfLetAnalyzer,
	)
}
