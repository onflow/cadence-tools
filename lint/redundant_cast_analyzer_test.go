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

package lint_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestRedundantCastAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("redundant", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub contract Test {
				pub fun test() {
					let x = true as Bool
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
						StartPos: ast.Position{Offset: 66, Line: 4, Column: 21},
						EndPos:   ast.Position{Offset: 69, Line: 4, Column: 24},
					},
					Location: testLocation,
					Category: lint.UnnecessaryCastCategory,
					Message:  "cast to `Bool` is redundant",
				},
			},
			diagnostics,
		)
	})

	t.Run("always succeeding force", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub contract Test {
				pub fun test() {
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
						StartPos: ast.Position{Offset: 58, Line: 4, Column: 13},
						EndPos:   ast.Position{Offset: 70, Line: 4, Column: 25},
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
			pub contract Test {
				pub fun test() {
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
						StartPos: ast.Position{Offset: 58, Line: 4, Column: 13},
						EndPos:   ast.Position{Offset: 70, Line: 4, Column: 25},
					},
					Location: testLocation,
					Category: lint.UnnecessaryCastCategory,
					Message:  "failable cast ('as?') from `Bool` to `Bool` always succeeds",
				},
			},
			diagnostics,
		)
	})

}
