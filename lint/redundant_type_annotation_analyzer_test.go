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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestRedundantTypeAnnotationAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("redundant", func(t *testing.T) {

		t.Parallel()

		const code = `access(all) let x: Bool = true`

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantTypeAnnotationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 17, Line: 1, Column: 17},
						EndPos:   ast.Position{Offset: 22, Line: 1, Column: 22},
					},
					Location: testLocation,
					Category: lint.UnnecessaryTypeAnnotationCategory,
					Message:  "type annotation is redundant, type can be inferred",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove redundant type annotation",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 17, Line: 1, Column: 17},
										EndPos:   ast.Position{Offset: 22, Line: 1, Column: 22},
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

		assert.Equal(t,
			`access(all) let x = true`,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("redundant", func(t *testing.T) {

		t.Parallel()

		const code = `access(all) let x: Int = 1`

		diagnostics := testAnalyzers(t,
			code,
			lint.RedundantTypeAnnotationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 17, Line: 1, Column: 17},
						EndPos:   ast.Position{Offset: 21, Line: 1, Column: 21},
					},
					Location: testLocation,
					Category: lint.UnnecessaryTypeAnnotationCategory,
					Message:  "type annotation is redundant, type can be inferred",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove redundant type annotation",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 17, Line: 1, Column: 17},
										EndPos:   ast.Position{Offset: 21, Line: 1, Column: 21},
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

		assert.Equal(t,
			`access(all) let x = 1`,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("not redundant", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  access(all) let x: Bool? = true
			`,
			lint.RedundantTypeAnnotationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("not redundant", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  access(all) let x: UInt64 = 1
			`,
			lint.RedundantTypeAnnotationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("type annotation needed", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) let xs: [UInt8] = []
              access(all) let ys: {String: Int} = {}
            `,
			lint.RedundantTypeAnnotationAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
