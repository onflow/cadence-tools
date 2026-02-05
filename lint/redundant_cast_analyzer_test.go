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
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestRedundantCastAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("redundant", func(t *testing.T) {

		t.Parallel()

		const code = `
          access(all) contract Test {
              access(all) fun test() {
                  let x = true as Bool
              }
          }
        `

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 108, Line: 4, Column: 30},
						EndPos:   ast.Position{Offset: 115, Line: 4, Column: 37},
					},
					Location: testLocation,
					Category: lint.UnnecessaryCastCategory,
					Message:  "static cast is redundant",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Remove redundant cast",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 108, Line: 4, Column: 30},
										EndPos:   ast.Position{Offset: 115, Line: 4, Column: 37},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)

		const expectedFixedCode = `
          access(all) contract Test {
              access(all) fun test() {
                  let x = true
              }
          }
        `

		assert.Equal(t,
			expectedFixedCode,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("not redundant", func(t *testing.T) {

		t.Parallel()

		const code = `
          access(all) contract Test {
              access(all) fun test() {
                  let x = true as Bool?
              }
          }
        `

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("not redundant", func(t *testing.T) {

		t.Parallel()

		const code = `
          access(all) contract Test {
              access(all) fun test() {
                  let x = 1 as UInt64
              }
          }
        `

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("always succeeding force", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) contract Test {
                  access(all) fun test() {
                      let x = true as! Bool
                  }
              }
            `,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 116, Line: 4, Column: 30},
						EndPos:   ast.Position{Offset: 128, Line: 4, Column: 42},
					},
					Location: testLocation,
					Category: lint.UnnecessaryCastCategory,
					Message:  "force cast ('as!') from `Bool` to `Bool` always succeeds",
				},
			},
			diagnostics,
		)
	})

	t.Run("always succeeding failable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) contract Test {
                  access(all) fun test() {
                      let x = true as? Bool
                  }
              }
            `,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 116, Line: 4, Column: 30},
						EndPos:   ast.Position{Offset: 128, Line: 4, Column: 42},
					},
					Location: testLocation,
					Category: lint.UnnecessaryCastCategory,
					Message:  "failable cast ('as?') from `Bool` to `Bool` always succeeds",
				},
			},
			diagnostics,
		)
	})

	t.Run("type annotation needed", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) contract Test {
                  access(all) fun test() {
                      let xs = [] as [UInt8]
                      let ys = {} as {String: Int}
                  }
              }
            `,
			lint.RedundantCastAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

}
