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

var RedundantParensAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.IfStatement)(nil),
		(*ast.WhileStatement)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects unnecessary parentheses around conditions in if and while statements",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			code := program.Code
			codeStr := string(code)
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {

					var keywordEndOffset int
					var blockStartOffset int
					var testStartPos, testEndPos ast.Position

					switch stmt := element.(type) {
					case *ast.IfStatement:
						keywordEndOffset = stmt.StartPos.Offset + len("if")
						blockStartOffset = stmt.Then.StartPosition().Offset
						testStartPos = stmt.Test.StartPosition()
						testEndPos = stmt.Test.EndPosition(nil)
					case *ast.WhileStatement:
						keywordEndOffset = stmt.StartPos.Offset + len("while")
						blockStartOffset = stmt.Block.StartPosition().Offset
						testStartPos = stmt.Test.StartPosition()
						testEndPos = stmt.Test.EndPosition(nil)
					default:
						return
					}

					if keywordEndOffset >= blockStartOffset ||
						blockStartOffset > len(code) {
						return
					}

					region := code[keywordEndOffset:blockStartOffset]

					// Find first and last non-whitespace byte in the region
					firstNonWS := -1
					lastNonWS := -1
					for i, b := range region {
						if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
							if firstNonWS == -1 {
								firstNonWS = i
							}
							lastNonWS = i
						}
					}

					if firstNonWS == -1 ||
						region[firstNonWS] != '(' ||
						region[lastNonWS] != ')' {
						return
					}

					// Verify the opening '(' matches the closing ')'
					// by counting balanced parentheses
					depth := 0
					for i := firstNonWS; i <= lastNonWS; i++ {
						switch region[i] {
						case '(':
							depth++
						case ')':
							depth--
							if depth == 0 && i != lastNonWS {
								// The opening '(' closed before the last ')',
								// so they are not wrapping the entire condition
								return
							}
						}
					}
					if depth != 0 {
						return
					}

					openParenOffset := keywordEndOffset + firstNonWS
					closeParenOffset := keywordEndOffset + lastNonWS

					openParenPos := ast.NewPositionAtCodeOffset(nil, codeStr, openParenOffset)
					closeParenPos := ast.NewPositionAtCodeOffset(nil, codeStr, closeParenOffset)

					diagnosticRange := ast.Range{
						StartPos: openParenPos,
						EndPos:   closeParenPos,
					}

					// Fix: remove '(' and any whitespace up to the expression,
					// and remove ')' and any whitespace from the expression end
					removeOpenRange := ast.Range{
						StartPos: openParenPos,
						EndPos:   testStartPos.Shifted(nil, -1),
					}
					removeCloseRange := ast.Range{
						StartPos: testEndPos.Shifted(nil, 1),
						EndPos:   closeParenPos,
					}

					report(analysis.Diagnostic{
						Location: location,
						Range:    diagnosticRange,
						Category: ReplacementCategory,
						Message:  "unnecessary parentheses around condition",
						SuggestedFixes: []analysis.SuggestedFix{
							{
								Message: "Remove parentheses",
								TextEdits: []ast.TextEdit{
									{
										Range:       removeOpenRange,
										Replacement: "",
									},
									{
										Range:       removeCloseRange,
										Replacement: "",
									},
								},
							},
						},
					})
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"redundant-parens",
		RedundantParensAnalyzer,
	)
}
