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
	"fmt"
	"regexp"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/tools/analysis"
)

func memberReplacement(memberInfo sema.MemberInfo) string {
	memberName := memberInfo.Member.Identifier.Identifier
	switch memberInfo.AccessedType {
	case sema.AuthAccountType:
		switch memberName {
		case sema.AuthAccountTypeGetCapabilityFunctionName:
			return "capabilities.get"

		case sema.AuthAccountTypeLinkFunctionName:
			return "capabilities.storage.issue"

		case sema.AuthAccountTypeLinkAccountFunctionName:
			return "capabilities.account.issue"

		case sema.AuthAccountTypeUnlinkFunctionName:
			return "capabilities.unpublish"
		}
	}

	return ""
}

var docStringDeprecationWarningPattern = regexp.MustCompile(`(?i)[\t *_]*deprecated\b(?:[*_]*: (.*))?`)

func MemberIsDeprecated(docString string) bool {
	return docStringDeprecationWarningPattern.MatchString(docString)
}

var DeprecatedMemberAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.MemberExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects uses of deprecated members",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			location := pass.Program.Location
			elaboration := pass.Program.Elaboration
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					memberExpression, ok := element.(*ast.MemberExpression)
					if !ok {
						return
					}

					memberInfo, _ := elaboration.MemberExpressionMemberInfo(memberExpression)
					member := memberInfo.Member
					if member == nil {
						return
					}

					docStringMatch := docStringDeprecationWarningPattern.FindStringSubmatch(member.DocString)
					if docStringMatch == nil {
						return
					}

					memberName := memberInfo.Member.Identifier.Identifier

					identifierRange := ast.NewRangeFromPositioned(nil, memberExpression.Identifier)

					var suggestedFixes []analysis.SuggestedFix

					replacement := memberReplacement(memberInfo)
					if replacement != "" {
						suggestedFix := analysis.SuggestedFix{
							Message: "replace",
							TextEdits: []analysis.TextEdit{
								{
									Replacement: replacement,
									Range:       identifierRange,
								},
							},
						}
						suggestedFixes = append(suggestedFixes, suggestedFix)
					}

					report(
						analysis.Diagnostic{
							Location: location,
							Range:    identifierRange,
							Category: DeprecatedCategory,
							Message: fmt.Sprintf(
								"%s '%s' is deprecated",
								member.DeclarationKind.Name(),
								memberName,
							),
							SecondaryMessage: docStringMatch[1],
							SuggestedFixes:   suggestedFixes,
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
		"deprecated-member",
		DeprecatedMemberAnalyzer,
	)
}
