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
	"strings"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

// isStringConcatMethodInvocation returns true if the given invocation expression
// is an invocation of the method String.concat
func isStringConcatMethodInvocation(
	invocation *ast.InvocationExpression,
	elaboration *sema.Elaboration,
) bool {
	memberExpr, ok := invocation.InvokedExpression.(*ast.MemberExpression)
	if !ok {
		return false
	}

	if memberExpr.Identifier.Identifier != sema.StringTypeConcatFunctionName ||
		memberExpr.Optional {

		return false
	}

	memberInfo, ok := elaboration.MemberExpressionMemberAccessInfo(memberExpr)
	if !ok {
		return false
	}

	if memberInfo.AccessedType == nil {
		return false
	}

	return memberInfo.AccessedType.Equal(sema.StringType)
}

type stringConcatPart struct {
	isLiteral bool
	code      []byte
}

// collectStringConcatParts recursively collects all parts of a String.concat chain
func collectStringConcatParts(
	expr ast.Expression,
	elaboration *sema.Elaboration,
	code []byte,
) []stringConcatPart {
	invocation, ok := expr.(*ast.InvocationExpression)
	if !ok || !isStringConcatMethodInvocation(invocation, elaboration) {
		// Base case: not a concat call, treat as an expression to interpolate
		return []stringConcatPart{
			extractStringConcatPart(expr, code),
		}
	}

	// Recursive case: this is a concat call.
	// Collect parts from the base expression
	memberExpr := invocation.InvokedExpression.(*ast.MemberExpression)
	parts := collectStringConcatParts(memberExpr.Expression, elaboration, code)

	// Add the argument as the next part
	if len(invocation.Arguments) > 0 {
		argument := invocation.Arguments[0]
		parts = append(
			parts,
			extractStringConcatPart(argument.Expression, code),
		)
	}

	return parts
}

// extractStringConcatPart extracts a stringConcatPart from an expression
func extractStringConcatPart(expr ast.Expression, code []byte) stringConcatPart {
	startPos := expr.StartPosition()
	endPos := expr.EndPosition(nil)
	if _, ok := expr.(*ast.StringExpression); ok {
		return stringConcatPart{
			isLiteral: true,
			// Exclude the surrounding quotes
			code: code[startPos.Offset+1 : endPos.Offset],
		}
	} else {
		return stringConcatPart{
			isLiteral: false,
			code:      code[startPos.Offset : endPos.Offset+1],
		}
	}
}

// buildStringTemplateFromStringConcatParts constructs a string template from collected String.concat parts
func buildStringTemplateFromStringConcatParts(parts []stringConcatPart) string {
	var result strings.Builder

	result.WriteByte('"')

	for _, part := range parts {
		if part.isLiteral {
			result.Write(part.code)
		} else {
			// For expressions, wrap in interpolation syntax
			result.WriteString(`\(`)
			result.Write(part.code)
			result.WriteByte(')')
		}
	}

	result.WriteByte('"')

	return result.String()
}

var StringConcatAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.InvocationExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects string concatenation using String.concat and suggests string templates",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			elaboration := program.Checker.Elaboration
			report := pass.Report

			inspector.Elements(
				elementFilter,
				func(element ast.Element, push bool) bool {
					// We only care about the push phase (traversal down the AST)
					if !push {
						return true
					}

					invocation, ok := element.(*ast.InvocationExpression)
					if !ok || !isStringConcatMethodInvocation(invocation, elaboration) {
						// Ignore this expression, but continue traversal down the AST
						return true
					}

					// Collect all parts of the concat chain
					parts := collectStringConcatParts(
						invocation,
						elaboration,
						program.Code,
					)

					// Only suggest if there is at least one literal part
					var hasLiteralPart bool
					for _, part := range parts {
						if part.isLiteral {
							hasLiteralPart = true
							break
						}
					}
					if !hasLiteralPart {
						// Ignore this expression, but continue traversal down the AST
						return true
					}

					// Build the string template replacement
					replacement := buildStringTemplateFromStringConcatParts(parts)

					// Report the diagnostic with a suggested fix
					newDiagnostic(
						location,
						report,
						"string concatenation can be replaced with string template",
						ast.NewRangeFromPositioned(nil, invocation),
					).
						WithCategory(ReplacementCategory).
						WithSimpleReplacement(replacement).
						Report()

					// Stop traversal down the AST
					return false
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"string-concat",
		StringConcatAnalyzer,
	)
}
