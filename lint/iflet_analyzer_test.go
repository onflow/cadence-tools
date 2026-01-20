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

	"github.com/onflow/cadence-tools/lint"
)

func TestIfLetAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("identifier expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if opt != nil {
                        let val = opt!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 161, Line: 5, Column: 34},
						EndPos:   ast.Position{Offset: 164, Line: 5, Column: 37},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("member expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) struct Foo {
                    access(all) var field: Int?
                    init() {
                        self.field = nil
                    }
                }

                access(all) fun test(obj: Foo) {
                    if obj.field != nil {
                        let val = obj.field!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 366, Line: 12, Column: 34},
						EndPos:   ast.Position{Offset: 375, Line: 12, Column: 43},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("index expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(arr: [Int?]) {
                    if arr[0] != nil {
                        let val = arr[0]!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 166, Line: 5, Column: 34},
						EndPos:   ast.Position{Offset: 172, Line: 5, Column: 40},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("nil on left side", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if nil != opt {
                        let val = opt!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 161, Line: 5, Column: 34},
						EndPos:   ast.Position{Offset: 164, Line: 5, Column: 37},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("other expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun foo(): Int? {
                    return 42
                }

                access(all) fun test() {
                    if self.foo() != nil {
                        let val = self.foo()!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("nil", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if opt == nil {
                        let val = opt!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("no force-unwrap", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if opt != nil {
                        let x = opt
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("different expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?, other: Int?) {
                    if opt != nil {
                        let val = other!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("force-unwrap in function expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if opt != nil {
                        let f = fun(opt: Int): Int {
                            return opt + 1
                        }
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("in function expression", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    let f = fun(): Int {
                        if opt != nil {
                            return opt!
                        }
                        return 0
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		// Should report because both check and unwrap are in the same scope (inside function expression)
		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 207, Line: 6, Column: 35},
						EndPos:   ast.Position{Offset: 210, Line: 6, Column: 38},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("only report first", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
            access(all) contract Test {
                access(all) fun test(opt: Int?) {
                    if opt != nil {
                        let val1 = opt!
                        let val2 = opt!
                    }
                }
            }
            `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 162, Line: 5, Column: 35},
						EndPos:   ast.Position{Offset: 165, Line: 5, Column: 38},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("nested", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
           access(all) contract Test {
               access(all) fun test(x: Int?, y: Int?) {
                   if x != nil {
                       if y != nil {
                           let a = x!
                           let b = y!
                       }
                   }
               }
           }
           `,
			lint.IfLetAnalyzer,
		)

		assert.Equal(t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 201, Line: 6, Column: 35},
						EndPos:   ast.Position{Offset: 202, Line: 6, Column: 36},
					},
				},
				{
					Location: testLocation,
					Category: lint.IfLetHintCategory,
					Message:  "consider using if-let instead of nil check followed by force-unwrap",
					URL:      "https://cadence-lang.org/docs/language/control-flow#optional-binding",
					Range: ast.Range{
						StartPos: ast.Position{Offset: 239, Line: 7, Column: 35},
						EndPos:   ast.Position{Offset: 240, Line: 7, Column: 36},
					},
				},
			},
			diagnostics,
		)
	})
}
