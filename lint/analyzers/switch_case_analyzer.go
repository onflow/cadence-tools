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

package analyzers

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
)

var _ ast.ExpressionVisitor = &DuplicateCaseChecker{}
var _ ast.TypeEqualityChecker = &DuplicateCaseChecker{}

type DuplicateCaseChecker struct {
	runtime.TypeComparator
	expr ast.Expression
}

func NewDuplicateCaseChecker(program *ast.Program) DuplicateCaseChecker {
	rootDeclIdentifier := getRootDeclarationIdentifier(program)

	return DuplicateCaseChecker{
		TypeComparator: runtime.TypeComparator{
			RootDeclIdentifier: rootDeclIdentifier,
		},
	}
}

func getRootDeclarationIdentifier(program *ast.Program) *ast.Identifier {
	compositeDecl := program.SoleContractDeclaration()
	if compositeDecl != nil {
		return compositeDecl.DeclarationIdentifier()
	}

	interfaceDecl := program.SoleContractInterfaceDeclaration()
	if interfaceDecl != nil {
		return interfaceDecl.DeclarationIdentifier()
	}

	return nil
}

func (d *DuplicateCaseChecker) isDuplicate(this ast.Expression, other ast.Expression) bool {
	if this == nil || other == nil {
		return false
	}

	tempExpr := d.expr
	d.expr = this
	defer func() {
		d.expr = tempExpr
	}()

	return other.AcceptExp(d).(bool)
}

func (d *DuplicateCaseChecker) VisitBoolExpression(otherExpr *ast.BoolExpression) ast.Repr {
	expr, ok := d.expr.(*ast.BoolExpression)
	if !ok {
		return false
	}

	return otherExpr.Value == expr.Value
}

func (d *DuplicateCaseChecker) VisitNilExpression(_ *ast.NilExpression) ast.Repr {
	_, ok := d.expr.(*ast.NilExpression)
	return ok
}

func (d *DuplicateCaseChecker) VisitIntegerExpression(otherExpr *ast.IntegerExpression) ast.Repr {
	expr, ok := d.expr.(*ast.IntegerExpression)
	if !ok {
		return false
	}

	return expr.Value.Cmp(otherExpr.Value) == 0
}

func (d *DuplicateCaseChecker) VisitFixedPointExpression(otherExpr *ast.FixedPointExpression) ast.Repr {
	expr, ok := d.expr.(*ast.FixedPointExpression)
	if !ok {
		return false
	}

	return expr.Negative == otherExpr.Negative &&
		expr.Fractional.Cmp(otherExpr.Fractional) == 0 &&
		expr.UnsignedInteger.Cmp(otherExpr.UnsignedInteger) == 0 &&
		expr.Scale == otherExpr.Scale
}

func (d *DuplicateCaseChecker) VisitArrayExpression(otherExpr *ast.ArrayExpression) ast.Repr {
	expr, ok := d.expr.(*ast.ArrayExpression)
	if !ok || len(expr.Values) != len(otherExpr.Values) {
		return false
	}

	for index, value := range expr.Values {
		if !d.isDuplicate(value, otherExpr.Values[index]) {
			return false
		}
	}

	return true
}

func (d *DuplicateCaseChecker) VisitDictionaryExpression(otherExpr *ast.DictionaryExpression) ast.Repr {
	expr, ok := d.expr.(*ast.DictionaryExpression)
	if !ok || len(expr.Entries) != len(otherExpr.Entries) {
		return false
	}

	for index, entry := range expr.Entries {
		otherEntry := otherExpr.Entries[index]

		if !d.isDuplicate(entry.Key, otherEntry.Key) ||
			!d.isDuplicate(entry.Value, otherEntry.Value) {
			return false
		}
	}

	return true
}

func (d *DuplicateCaseChecker) VisitIdentifierExpression(otherExpr *ast.IdentifierExpression) ast.Repr {
	expr, ok := d.expr.(*ast.IdentifierExpression)
	if !ok {
		return false
	}

	return expr.Identifier.Identifier == otherExpr.Identifier.Identifier
}

func (d *DuplicateCaseChecker) VisitInvocationExpression(_ *ast.InvocationExpression) ast.Repr {
	// Invocations can be stateful. Thus, it's not possible to determine if
	// invoking the same function in two cases would produce the same results.
	return false
}

func (d *DuplicateCaseChecker) VisitMemberExpression(otherExpr *ast.MemberExpression) ast.Repr {
	expr, ok := d.expr.(*ast.MemberExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Expression, otherExpr.Expression) &&
		expr.Optional == otherExpr.Optional &&
		expr.Identifier.Identifier == otherExpr.Identifier.Identifier
}

func (d *DuplicateCaseChecker) VisitIndexExpression(otherExpr *ast.IndexExpression) ast.Repr {
	expr, ok := d.expr.(*ast.IndexExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.TargetExpression, otherExpr.TargetExpression) &&
		d.isDuplicate(expr.IndexingExpression, otherExpr.IndexingExpression)
}

func (d *DuplicateCaseChecker) VisitConditionalExpression(otherExpr *ast.ConditionalExpression) ast.Repr {
	expr, ok := d.expr.(*ast.ConditionalExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Test, otherExpr.Test) &&
		d.isDuplicate(expr.Then, otherExpr.Then) &&
		d.isDuplicate(expr.Else, otherExpr.Else)
}

func (d *DuplicateCaseChecker) VisitUnaryExpression(otherExpr *ast.UnaryExpression) ast.Repr {
	expr, ok := d.expr.(*ast.UnaryExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Expression, otherExpr.Expression) &&
		expr.Operation == otherExpr.Operation
}

func (d *DuplicateCaseChecker) VisitBinaryExpression(otherExpr *ast.BinaryExpression) ast.Repr {
	expr, ok := d.expr.(*ast.BinaryExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Left, otherExpr.Left) &&
		d.isDuplicate(expr.Right, otherExpr.Right) &&
		expr.Operation == otherExpr.Operation
}

func (d *DuplicateCaseChecker) VisitFunctionExpression(_ *ast.FunctionExpression) ast.Repr {
	// Not a valid expression for switch-case. Hence, skip.
	return false
}

func (d *DuplicateCaseChecker) VisitStringExpression(otherExpr *ast.StringExpression) ast.Repr {
	expr, ok := d.expr.(*ast.StringExpression)
	if !ok {
		return false
	}

	return expr.Value == otherExpr.Value
}

func (d *DuplicateCaseChecker) VisitCastingExpression(otherExpr *ast.CastingExpression) ast.Repr {
	expr, ok := d.expr.(*ast.CastingExpression)
	if !ok {
		return false
	}

	if !d.isDuplicate(expr.Expression, otherExpr.Expression) ||
		expr.Operation != otherExpr.Operation {
		return false
	}

	typ := expr.TypeAnnotation.Type
	otherType := otherExpr.TypeAnnotation.Type
	return typ.CheckEqual(otherType, d) == nil
}

func (d *DuplicateCaseChecker) VisitCreateExpression(_ *ast.CreateExpression) ast.Repr {
	// Not a valid expression for switch-case. Hence, skip.
	return false
}

func (d *DuplicateCaseChecker) VisitDestroyExpression(_ *ast.DestroyExpression) ast.Repr {
	// Not a valid expression for switch-case. Hence, skip.
	return false
}

func (d *DuplicateCaseChecker) VisitReferenceExpression(otherExpr *ast.ReferenceExpression) ast.Repr {
	expr, ok := d.expr.(*ast.ReferenceExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Expression, otherExpr.Expression)
}

func (d *DuplicateCaseChecker) VisitForceExpression(otherExpr *ast.ForceExpression) ast.Repr {
	expr, ok := d.expr.(*ast.ForceExpression)
	if !ok {
		return false
	}

	return d.isDuplicate(expr.Expression, otherExpr.Expression)
}

func (d *DuplicateCaseChecker) VisitPathExpression(otherExpr *ast.PathExpression) ast.Repr {
	expr, ok := d.expr.(*ast.PathExpression)
	if !ok {
		return false
	}

	return expr.Domain == otherExpr.Domain &&
		expr.Identifier.Identifier == otherExpr.Identifier.Identifier
}

var SwitchCaseAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.SwitchStatement)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects duplicate switch cases",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			location := pass.Program.Location
			program := pass.Program.Program
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {

					switchStatement, ok := element.(*ast.SwitchStatement)
					if !ok {
						return
					}

					duplicates := make(map[*ast.SwitchCase]bool)

					duplicateChecker := NewDuplicateCaseChecker(program)

					cases := switchStatement.Cases
					for i, switchCase := range cases {

						// If the current case is already identified as a duplicate,
						// then no need to check it again. Can simply skip.
						if _, isDuplicate := duplicates[switchCase]; isDuplicate {
							continue
						}

						for j := i + 1; j < len(cases); j++ {
							otherCase := cases[j]
							if !duplicateChecker.isDuplicate(switchCase.Expression, otherCase.Expression) {
								continue
							}

							duplicates[otherCase] = true

							report(
								analysis.Diagnostic{
									Location: location,
									Range:    ast.NewRangeFromPositioned(nil, otherCase.Expression),
									Category: "lint",
									Message:  "duplicate switch case",
								},
							)
						}
					}
				},
			)

			return nil
		},
	}
})()

func init() {
	registerAnalyzer(
		"switch-case-analyzer",
		SwitchCaseAnalyzer,
	)
}
