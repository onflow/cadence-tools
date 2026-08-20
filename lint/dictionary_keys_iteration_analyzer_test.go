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

func TestDictionaryKeysIterationAnalyzer(t *testing.T) {
	t.Parallel()

	t.Run("for key in dict.keys", func(t *testing.T) {
		t.Parallel()

		code := `
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					for key in dict.keys {
						log(key)
					}
				}
			}
			`

		diagnostics := testAnalyzers(t,
			code,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Range: ast.Range{
						StartPos: ast.Position{Offset: 129, Line: 5, Column: 20},
						EndPos:   ast.Position{Offset: 133, Line: 5, Column: 24},
					},
					Category: lint.ReplacementCategory,
					Message:  "unnecessary '.keys': iterating over a dictionary directly yields its keys",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove code",
							TextEdits: []ast.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 129, Line: 5, Column: 20},
										EndPos:   ast.Position{Offset: 133, Line: 5, Column: 24},
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

		fixedCode := diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code)

		expectedFixedCode := `
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					for key in dict {
						log(key)
					}
				}
			}
			`

		require.Equal(t, expectedFixedCode, fixedCode)
	})

	t.Run("for key in dict (no .keys)", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					for key in dict {
						log(key)
					}
				}
			}
			`,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run("for i, key in dict.keys (indexed iteration)", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					for i, key in dict.keys {
						log(i)
						log(key)
					}
				}
			}
			`,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run("for val in dict.values", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					for val in dict.values {
						log(val)
					}
				}
			}
			`,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run(".keys on non-dictionary type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) struct Foo {
					access(all) let keys: [String]
					init() {
						self.keys = ["a", "b"]
					}
				}
				access(all) fun test() {
					let foo = Foo()
					for key in foo.keys {
						log(key)
					}
				}
			}
			`,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})

	t.Run(".keys outside for loop", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let dict: {String: Int} = {"a": 1, "b": 2}
					let k = dict.keys
				}
			}
			`,
			lint.DictionaryKeysIterationAnalyzer,
		)

		require.Equal(t, []analysis.Diagnostic(nil), diagnostics)
	})
}
