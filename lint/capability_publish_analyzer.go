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

var CapabilityPublishAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.InvocationExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects capability publish calls — verify that proper entitlements guard the capability",
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
					invocation, ok := element.(*ast.InvocationExpression)
					if !ok {
						return
					}

					memberExpr, ok := invocation.InvokedExpression.(*ast.MemberExpression)
					if !ok {
						return
					}

					if memberExpr.Identifier.Identifier != "publish" {
						return
					}

					// Use type information to verify the receiver is an
					// Account.Capabilities value, avoiding false positives
					// on unrelated .publish() methods.
					memberInfo, ok := elaboration.MemberExpressionMemberAccessInfo(memberExpr)
					if !ok {
						return
					}

					receiverType := memberInfo.AccessedType

					// Unwrap reference types
					if refType, ok := receiverType.(*sema.ReferenceType); ok {
						receiverType = refType.Type
					}

					if receiverType != sema.Account_CapabilitiesType {
						return
					}

					report(
						analysis.Diagnostic{
							Location: location,
							Range:    ast.NewRangeFromPositioned(nil, invocation),
							Category: SecurityCategory,
							Message:  "capability published — verify that proper entitlements guard this capability",
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
		"capability-publish",
		CapabilityPublishAnalyzer,
	)
}
