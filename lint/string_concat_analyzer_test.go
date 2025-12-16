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

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestStringConcatAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run(`"a" "b"`, func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  access(all) let x = "a".concat("b")
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 26, Line: 2, Column: 25},
						EndPos:   ast.Position{Offset: 40, Line: 2, Column: 39},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"ab\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 26, Line: 2, Column: 25},
										EndPos:   ast.Position{Offset: 40, Line: 2, Column: 39},
									},
									Replacement: `"ab"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run(`a "b"`, func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) let a = "a"
			  access(all) let x = a.concat("b")
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 64, Line: 3, Column: 25},
						EndPos:   ast.Position{Offset: 76, Line: 3, Column: 37},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"\\(a)b\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 64, Line: 3, Column: 25},
										EndPos:   ast.Position{Offset: 76, Line: 3, Column: 37},
									},
									Replacement: `"\(a)b"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run(`"a" b`, func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) let b = "b"
			  access(all) let x = "a".concat(b)
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 64, Line: 3, Column: 25},
						EndPos:   ast.Position{Offset: 76, Line: 3, Column: 37},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"a\\(b)\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 64, Line: 3, Column: 25},
										EndPos:   ast.Position{Offset: 76, Line: 3, Column: 37},
									},
									Replacement: `"a\(b)"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run(`a b`, func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) let a = "a"
              access(all) let b = "b"
			  access(all) let x = a.concat(b)
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 102, Line: 4, Column: 25},
						EndPos:   ast.Position{Offset: 112, Line: 4, Column: 35},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"\\(a)\\(b)\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 102, Line: 4, Column: 25},
										EndPos:   ast.Position{Offset: 112, Line: 4, Column: 35},
									},
									Replacement: `"\(a)\(b)"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run(`a "b" c`, func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  access(all) let a = "a"
			  access(all) let c = "c"
			  access(all) let x = a.concat("b").concat(c)
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 84, Line: 4, Column: 25},
						EndPos:   ast.Position{Offset: 106, Line: 4, Column: 47},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"\\(a)b\\(c)\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 84, Line: 4, Column: 25},
										EndPos:   ast.Position{Offset: 106, Line: 4, Column: 47},
									},
									Replacement: `"\(a)b\(c)"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run(`"a" b "c"`, func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  access(all) let b = "b"
			  access(all) let x = "a".concat(b).concat("c")
			`,
			lint.StringConcatAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 55, Line: 3, Column: 25},
						EndPos:   ast.Position{Offset: 79, Line: 3, Column: 49},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "string concatenation can be replaced with string template",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `\"a\\(b)c\"`",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 55, Line: 3, Column: 25},
										EndPos:   ast.Position{Offset: 79, Line: 3, Column: 49},
									},
									Replacement: `"a\(b)c"`,
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})
}
