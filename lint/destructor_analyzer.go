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
	"strings"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

type ComplexDestructorKind uint

const (
	EventEmission ComplexDestructorKind = iota
	TotalSupplyDecrement
	AssertOrCondition
	LoggingCall
	PanicCall
	IfStatement
	LoopStatement
	OtherComplexOperation
)

func stringOfKind(
	kind ComplexDestructorKind,
) string {
	switch kind {
	case EventEmission:
		return "EventEmission"
	case TotalSupplyDecrement:
		return "TotalSupplyDecrement"
	case AssertOrCondition:
		return "AssertOrCondition"
	case OtherComplexOperation:
		return "OtherComplexOperation"
	case PanicCall:
		return "PanicCall"
	case IfStatement:
		return "IfStatement"
	case LoopStatement:
		return "LoopStatement"
	case LoggingCall:
		return "LoggingCall"
	}
	return "UnknownKind"
}

func reportComplexDestructor(
	destructor *ast.SpecialFunctionDeclaration,
	location common.Location,
	kinds []ComplexDestructorKind,
) *analysis.Diagnostic {
	kindString := "kinds: "

	for _, kind := range kinds {
		k := stringOfKind(kind)

		if !strings.Contains(kindString, k) {
			kindString = kindString + k + ", "
		}
	}

	return &analysis.Diagnostic{
		Location:         location,
		Range:            ast.NewRangeFromPositioned(nil, destructor),
		Category:         UpdateCategory,
		Message:          "complex destructor found",
		SecondaryMessage: kindString,
	}
}

func isComplexDestructor(
	statements []ast.Statement,
) (kinds []ComplexDestructorKind) {

	for _, statement := range statements {
		switch statement := statement.(type) {
		case *ast.ReturnStatement:
			continue
		case *ast.EmitStatement:
			kinds = append(kinds, EventEmission)
		case *ast.IfStatement:
			kinds = append(kinds, IfStatement)
			kinds = append(kinds, isComplexDestructor(statement.Then.Statements)...)
			if statement.Else != nil {
				kinds = append(kinds, isComplexDestructor(statement.Else.Statements)...)
			}
		case *ast.ForStatement:
			kinds = append(kinds, LoopStatement)
			kinds = append(kinds, isComplexDestructor(statement.Block.Statements)...)
		case *ast.WhileStatement:
			kinds = append(kinds, LoopStatement)
			kinds = append(kinds, isComplexDestructor(statement.Block.Statements)...)
		case *ast.AssignmentStatement:
			switch target := statement.Target.(type) {
			case *ast.MemberExpression:
				if target.Identifier.Identifier == "totalSupply" || target.Identifier.Identifier == "TotalSupply" {
					kinds = append(kinds, TotalSupplyDecrement)
				} else {
					kinds = append(kinds, OtherComplexOperation)
				}
			default:
				kinds = append(kinds, OtherComplexOperation)
			}
		case *ast.ExpressionStatement:
			switch expr := statement.Expression.(type) {
			case *ast.DestroyExpression:
				continue
			case *ast.InvocationExpression:
				switch invoked := expr.InvokedExpression.(type) {
				case *ast.IdentifierExpression:
					if invoked.Identifier.Identifier == "assert" {
						kinds = append(kinds, AssertOrCondition)
					} else if invoked.Identifier.Identifier == "log" {
						kinds = append(kinds, LoggingCall)
					} else if invoked.Identifier.Identifier == "panic" {
						kinds = append(kinds, PanicCall)
					} else {
						kinds = append(kinds, OtherComplexOperation)
					}
				default:
					kinds = append(kinds, OtherComplexOperation)
				}
			default:
				kinds = append(kinds, OtherComplexOperation)
			}
		default:
			kinds = append(kinds, OtherComplexOperation)
		}
	}

	return kinds
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
						complexKinds := isComplexDestructor(expr.FunctionDeclaration.FunctionBlock.Block.Statements)

						if expr.FunctionDeclaration.FunctionBlock.PostConditions != nil ||
							expr.FunctionDeclaration.FunctionBlock.PreConditions != nil {
							complexKinds = append(complexKinds, AssertOrCondition)
						}

						if complexKinds != nil {
							diagnostic = reportComplexDestructor(expr, location, complexKinds)
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
