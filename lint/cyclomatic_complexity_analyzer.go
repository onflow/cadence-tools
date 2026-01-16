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
	"fmt"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/tools/analysis"
)

// NewCyclomaticComplexityAnalyzer creates an analyzer that detects functions
// with high cyclomatic complexity.
//
// Cyclomatic complexity is calculated as:
// - Base complexity: 1
// - +1 for each: if, else-if, for, while, switch, case
// - +1 for each: && and || (if countLogicalOperators is true)
//
// Functions with complexity exceeding the threshold are reported.
func NewCyclomaticComplexityAnalyzer(threshold int, countLogicalOperators bool) *analysis.Analyzer {

	calculator := complexityCalculator{
		countLogicalOperators: countLogicalOperators,
	}

	elementFilter := []ast.Element{
		(*ast.FunctionDeclaration)(nil),
		(*ast.SpecialFunctionDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects functions with high cyclomatic complexity",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					var (
						identifier    ast.Identifier
						functionBlock *ast.FunctionBlock
					)

					switch decl := element.(type) {
					case *ast.FunctionDeclaration:
						identifier = decl.Identifier
						functionBlock = decl.FunctionBlock

					case *ast.SpecialFunctionDeclaration:
						if decl.FunctionDeclaration == nil {
							return
						}
						identifier = decl.FunctionDeclaration.Identifier
						functionBlock = decl.FunctionDeclaration.FunctionBlock

					default:
						return
					}

					if functionBlock == nil || functionBlock.Block == nil {
						return
					}

					complexity := 1 + calculator.blockComplexity(functionBlock.Block)

					if complexity > threshold {
						report(
							analysis.Diagnostic{
								Location: location,
								Range:    ast.NewRangeFromPositioned(nil, identifier),
								Category: ComplexityCategory,
								Message: fmt.Sprintf(
									"function %q has cyclomatic complexity %d (threshold: %d)",
									identifier.Identifier,
									complexity,
									threshold,
								),
							},
						)
					}
				},
			)

			return nil
		},
	}
}

type complexityCalculator struct {
	countLogicalOperators bool
}

func (c complexityCalculator) blockComplexity(block *ast.Block) int {
	if block == nil {
		return 0
	}

	var complexity int

	for _, statement := range block.Statements {
		complexity += c.statementComplexity(statement)
	}

	return complexity
}

func (c complexityCalculator) statementComplexity(statement ast.Statement) int {
	var complexity int

	switch stmt := statement.(type) {
	case *ast.IfStatement:
		complexity += 1 +
			c.countIfStatementTestComplexity(stmt.Test) +
			c.blockComplexity(stmt.Then) +
			c.blockComplexity(stmt.Else)

	case *ast.WhileStatement:
		complexity += 1 +
			c.countExpressionComplexity(stmt.Test) +
			c.blockComplexity(stmt.Block)

	case *ast.ForStatement:
		complexity += 1 +
			c.blockComplexity(stmt.Block)

	case *ast.SwitchStatement:
		complexity += 1

		for _, switchCase := range stmt.Cases {
			complexity += 1

			if switchCase.Statements != nil {
				for _, caseStmt := range switchCase.Statements {
					complexity += c.statementComplexity(caseStmt)
				}
			}
		}

	case *ast.ExpressionStatement:
		complexity += c.countExpressionComplexity(stmt.Expression)

	case *ast.ReturnStatement:
		complexity += c.countExpressionComplexity(stmt.Expression)

	case *ast.AssignmentStatement:
		complexity += c.countExpressionComplexity(stmt.Value)

	case *ast.VariableDeclaration:
		complexity += c.countExpressionComplexity(stmt.Value)
		complexity += c.countExpressionComplexity(stmt.SecondValue)

	case *ast.EmitStatement:
		complexity += c.countExpressionComplexity(stmt.InvocationExpression)

	case *ast.SwapStatement:
		complexity += c.countExpressionComplexity(stmt.Left)
		complexity += c.countExpressionComplexity(stmt.Right)

	case *ast.RemoveStatement:
		complexity += c.countExpressionComplexity(stmt.Value)
	}

	return complexity
}

func (c complexityCalculator) countIfStatementTestComplexity(test ast.IfStatementTest) int {
	switch test := test.(type) {
	case *ast.VariableDeclaration:
		return c.countExpressionComplexity(test.Value)
	case ast.Expression:
		return c.countExpressionComplexity(test)
	default:
		panic(errors.NewUnreachableError())
	}
}

func (c complexityCalculator) countExpressionComplexity(expr ast.Expression) int {
	if expr == nil {
		return 0
	}

	// Function expressions always contribute complexity (control flow)
	// regardless of the countLogicalOperators setting
	if funcExpr, ok := expr.(*ast.FunctionExpression); ok {
		return c.blockComplexity(funcExpr.FunctionBlock.Block)
	}

	// For other expressions, only count if logical operators are enabled
	if !c.countLogicalOperators {
		return 0
	}

	complexity := 0

	switch e := expr.(type) {
	case *ast.BinaryExpression:
		if e.Operation == ast.OperationAnd || e.Operation == ast.OperationOr {
			complexity += 1
		}

		complexity += c.countExpressionComplexity(e.Left)
		complexity += c.countExpressionComplexity(e.Right)

	case *ast.ConditionalExpression:
		complexity += c.countExpressionComplexity(e.Test)
		complexity += c.countExpressionComplexity(e.Then)
		complexity += c.countExpressionComplexity(e.Else)

	case *ast.InvocationExpression:
		complexity += c.countExpressionComplexity(e.InvokedExpression)
		for _, arg := range e.Arguments {
			complexity += c.countExpressionComplexity(arg.Expression)
		}

	case *ast.IndexExpression:
		complexity += c.countExpressionComplexity(e.TargetExpression)
		complexity += c.countExpressionComplexity(e.IndexingExpression)

	case *ast.MemberExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.UnaryExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.CastingExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.CreateExpression:
		complexity += c.countExpressionComplexity(e.InvocationExpression)

	case *ast.DestroyExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.ReferenceExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.ForceExpression:
		complexity += c.countExpressionComplexity(e.Expression)

	case *ast.ArrayExpression:
		for _, element := range e.Values {
			complexity += c.countExpressionComplexity(element)
		}

	case *ast.DictionaryExpression:
		for _, entry := range e.Entries {
			complexity += c.countExpressionComplexity(entry.Key)
			complexity += c.countExpressionComplexity(entry.Value)
		}

	case *ast.StringTemplateExpression:
		for _, part := range e.Expressions {
			complexity += c.countExpressionComplexity(part)
		}
	}

	return complexity
}
