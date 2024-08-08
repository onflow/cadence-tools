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

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestUnusedResultAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("binary expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let x = 3
                  x + 1
              }
            `,
			lint.UnusedResultAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 86, Line: 4, Column: 18},
						EndPos:   ast.Position{Offset: 90, Line: 4, Column: 22},
					},
					Location: testLocation,
					Category: lint.UnusedResultCategory,
					Message:  "unused result",
				},
			},
			diagnostics,
		)
	})

	t.Run("non-void member function", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let string = "hello"
                  string.concat("world")
              }
            `,
			lint.UnusedResultAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 97, Line: 4, Column: 18},
						EndPos:   ast.Position{Offset: 118, Line: 4, Column: 39},
					},
					Location: testLocation,
					Category: lint.UnusedResultCategory,
					Message:  "unused result",
				},
			},
			diagnostics,
		)
	})

	t.Run("non-void function", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun answer(): Int {
                  return 42
              }

              access(all) fun test() {
                  answer()
              }
            `,
			lint.UnusedResultAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 149, Line: 7, Column: 18},
						EndPos:   ast.Position{Offset: 156, Line: 7, Column: 25},
					},
					Location: testLocation,
					Category: lint.UnusedResultCategory,
					Message:  "unused result",
				},
			},
			diagnostics,
		)
	})

	t.Run("never function", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  panic("test")
              }
            `,
			lint.UnusedResultAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("void function", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun nothing() {}

              access(all) fun test() {
                  nothing()
              }
            `,
			lint.UnusedResultAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
