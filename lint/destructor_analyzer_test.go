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

func TestCheckIdentifiesComplexDestructors(t *testing.T) {

	t.Parallel()

	t.Run("finds complex destructor", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) fun foo() {}
				access(all) resource R {
					destroy() {
						Test.foo()
					}
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 96, Line: 5, Column: 5},
						EndPos:   ast.Position{Offset: 130, Line: 7, Column: 5},
					},
					Location:         testLocation,
					Category:         lint.UpdateCategory,
					Message:          "complex destructor found",
					SecondaryMessage: "kinds: OtherComplexOperation, ",
				},
			},
			diagnostics,
		)
	})

	t.Run("ignores simple destructor", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) fun foo() {}
				access(all) resource Sub {}
				access(all) resource R {
					access(all) let sub: @Sub
					init() {
						self.sub <- create Sub()
					}
					destroy() {
						destroy self.sub
					}
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Nil(
			t,
			diagnostics,
		)
	})

	t.Run("finds event emission destructor", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) event Foo()
				access(all) resource R {
					destroy() {
						emit Test.Foo()
					}
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Len(t, diagnostics, 1)

		require.Equal(
			t,
			"kinds: EventEmission, ",
			diagnostics[0].SecondaryMessage,
		)
	})

	t.Run("finds assert destructor", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) resource R {
					destroy() {
						assert(1 > 2, message: "")
					}
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Len(t, diagnostics, 1)

		require.Equal(
			t,
			"kinds: AssertOrCondition, ",
			diagnostics[0].SecondaryMessage,
		)
	})

	t.Run("finds condition destructor", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) event Foo()
				access(all) resource R {
					destroy() {
						pre {
							1 > 2: ""
						}
						emit Test.Foo()
					}
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Len(t, diagnostics, 1)

		require.Equal(
			t,
			"kinds: EventEmission, AssertOrCondition, ",
			diagnostics[0].SecondaryMessage,
		)
	})

	t.Run("finds total supply", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t, `
			 access(all) contract Test {
				access(all) var totalSupply: UInt
				access(all) resource R {
					destroy() {
						Test.totalSupply = Test.totalSupply - 3
					}
				}
				init() {
					self.totalSupply = 0
				}
			 }`,
			lint.ComplexDestructorAnalyzer,
		)

		require.Len(t, diagnostics, 1)

		require.Equal(
			t,
			"kinds: TotalSupplyDecrement, ",
			diagnostics[0].SecondaryMessage,
		)
	})
}
