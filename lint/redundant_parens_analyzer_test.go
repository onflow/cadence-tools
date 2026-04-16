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

package lint_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestRedundantParensAnalyzer(t *testing.T) {
	t.Parallel()

	t.Run("if with redundant parens", func(t *testing.T) {
		t.Parallel()

		code := `
			access(all) contract Test {
				access(all) fun test() {
					if (true) {}
				}
			}
			`

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Range: ast.Range{
						StartPos: ast.Position{Offset: 69, Line: 4, Column: 8},
						EndPos:   ast.Position{Offset: 74, Line: 4, Column: 13},
					},
					Category: lint.ReplacementCategory,
					Message:  "unnecessary parentheses around condition",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove parentheses",
							TextEdits: []ast.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 69, Line: 4, Column: 8},
										EndPos:   ast.Position{Offset: 69, Line: 4, Column: 8},
									},
									Replacement: "",
								},
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 74, Line: 4, Column: 13},
										EndPos:   ast.Position{Offset: 74, Line: 4, Column: 13},
									},
									Replacement: "",
								},
							},
						},
					},
				},
			},
			diagnostics,
		)

		// Apply fixes in reverse order so offsets stay valid
		fixedCode := diagnostics[0].SuggestedFixes[0].TextEdits[1].ApplyTo(code)
		fixedCode = diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(fixedCode)

		expectedFixedCode := `
			access(all) contract Test {
				access(all) fun test() {
					if true {}
				}
			}
			`

		require.Equal(t, expectedFixedCode, fixedCode)
	})

	t.Run("if without parens", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					if true {}
				}
			}
			`,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run("while with redundant parens", func(t *testing.T) {
		t.Parallel()

		code := `
			access(all) contract Test {
				access(all) fun test() {
					while (true) {}
				}
			}
			`

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Range: ast.Range{
						StartPos: ast.Position{Offset: 72, Line: 4, Column: 11},
						EndPos:   ast.Position{Offset: 77, Line: 4, Column: 16},
					},
					Category: lint.ReplacementCategory,
					Message:  "unnecessary parentheses around condition",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove parentheses",
							TextEdits: []ast.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 72, Line: 4, Column: 11},
										EndPos:   ast.Position{Offset: 72, Line: 4, Column: 11},
									},
									Replacement: "",
								},
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 77, Line: 4, Column: 16},
										EndPos:   ast.Position{Offset: 77, Line: 4, Column: 16},
									},
									Replacement: "",
								},
							},
						},
					},
				},
			},
			diagnostics,
		)

		fixedCode := diagnostics[0].SuggestedFixes[0].TextEdits[1].ApplyTo(code)
		fixedCode = diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(fixedCode)

		expectedFixedCode := `
			access(all) contract Test {
				access(all) fun test() {
					while true {}
				}
			}
			`

		require.Equal(t, expectedFixedCode, fixedCode)
	})

	t.Run("while without parens", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					while true {}
				}
			}
			`,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run("if with binary expression in parens", func(t *testing.T) {
		t.Parallel()

		code := `
			access(all) contract Test {
				access(all) fun test() {
					if (1 == 1) {}
				}
			}
			`

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(t, 1, len(diagnostics))
		require.Equal(t, "unnecessary parentheses around condition", diagnostics[0].Message)

		fixedCode := diagnostics[0].SuggestedFixes[0].TextEdits[1].ApplyTo(code)
		fixedCode = diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(fixedCode)

		expectedFixedCode := `
			access(all) contract Test {
				access(all) fun test() {
					if 1 == 1 {}
				}
			}
			`

		require.Equal(t, expectedFixedCode, fixedCode)
	})

	t.Run("if with sub-expression parens only", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					if (1 == 1) && (2 == 2) {}
				}
			}
			`,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run("if let (optional binding)", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x: Int? = 1
					if let y = x {}
				}
			}
			`,
			lint.RedundantParensAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})
}
