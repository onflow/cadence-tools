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

// CapabilityPublishAnalyzer detects calls to Account.Capabilities.publish() where the
// capability being published has an entitled borrow type.
// Publishing an entitled capability makes the entitlements publicly accessible,
// which may grant more authority than intended. Verify that the entitlements
// on the published capability are intentional and scoped appropriately.
var CapabilityPublishAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.InvocationExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects capability publish calls where the capability has entitled access",
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

					if memberExpr.Identifier.Identifier != sema.Account_CapabilitiesTypePublishFunctionName {
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

					// Check the type of the capability argument.
					// Only flag when the capability's borrow type is entitled,
					// as that grants more authority than a plain reference.
					invocationTypes := elaboration.InvocationExpressionTypes(invocation)
					if len(invocationTypes.ArgumentTypes) == 0 {
						return
					}

					capType, ok := invocationTypes.ArgumentTypes[0].(*sema.CapabilityType)
					if !ok {
						return
					}

					// Check if the borrow type is an entitled reference
					borrowRef, ok := capType.BorrowType.(*sema.ReferenceType)
					if !ok {
						return
					}

					switch borrowRef.Authorization.(type) {
					case *sema.EntitlementMapAccess,
						sema.EntitlementSetAccess:
						// Entitled — flag it
					default:
						return
					}

					report(
						analysis.Diagnostic{
							Location: location,
							Range:    ast.NewRangeFromPositioned(nil, invocation),
							Category: SecurityCategory,
							Message:  "entitled capability published — verify that the entitlements are intentional and scoped appropriately",
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
