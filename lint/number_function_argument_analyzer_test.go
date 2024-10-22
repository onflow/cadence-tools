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

func TestCheckNumberConversionReplacementHint(t *testing.T) {

	t.Parallel()

	// to fixed point type

	//// integer literal

	t.Run("positive integer to signed fixed-point type", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			access(all) contract Test {
				access(all) fun test() {
					let x = Fix64(1)
				}
			}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 74, Line: 4, Column: 13},
						EndPos:   ast.Position{Offset: 81, Line: 4, Column: 20},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1.0 as Fix64",
				},
			},
			diagnostics,
		)
	})

	t.Run("positive integer to unsigned fixed-point type", func(t *testing.T) {

		t.Parallel()
		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = UFix64(1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 79, Line: 4, Column: 20},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1.0",
				},
			},
			diagnostics,
		)
	})

	t.Run("negative integer to signed fixed-point type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Fix64(-1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 79, Line: 4, Column: 20},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "-1.0",
				},
			},
			diagnostics,
		)
	})

	//// fixed-point literal

	t.Run("positive fixed-point to unsigned fixed-point type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = UFix64(1.2)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 81, Line: 4, Column: 22},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1.2",
				},
			},
			diagnostics,
		)
	})

	t.Run("negative fixed-point to signed fixed-point type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Fix64(-1.2)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 81, Line: 4, Column: 22},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "-1.2",
				},
			},
			diagnostics,
		)
	})

	// to integer type

	//// integer literal

	t.Run("positive integer to unsigned integer type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = UInt8(1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 78, Line: 4, Column: 19},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1 as UInt8",
				},
			},
			diagnostics,
		)
	})

	t.Run("positive integer to signed integer type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Int8(1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 77, Line: 4, Column: 18},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1 as Int8",
				},
			},
			diagnostics,
		)
	})

	t.Run("negative integer to signed integer type", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Int8(-1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 78, Line: 4, Column: 19},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "-1 as Int8",
				},
			},
			diagnostics,
		)
	})

	t.Run("positive integer to Int", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Int(1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 76, Line: 4, Column: 17},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "1",
				},
			},
			diagnostics,
		)
	})

	t.Run("negative integer to Int", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
		access(all) contract Test {
			access(all) fun test() {
				let x = Int(-1)
			}
		}`,
			lint.NumberFunctionArgumentAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 71, Line: 4, Column: 12},
						EndPos:   ast.Position{Offset: 77, Line: 4, Column: 18},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "consider replacing with:",
					SecondaryMessage: "-1",
				},
			},
			diagnostics,
		)
	})
}
