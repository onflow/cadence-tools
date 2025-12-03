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
	"regexp"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/tools/analysis"
)

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

			program := pass.Program
			location := program.Location
			elaboration := program.Checker.Elaboration
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					memberExpression, ok := element.(*ast.MemberExpression)
					if !ok {
						return
					}

					memberInfo, _ := elaboration.MemberExpressionMemberAccessInfo(memberExpression)
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
