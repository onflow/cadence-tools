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

package analyzers_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-lint/analyzers"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/tests/checker"
	"github.com/onflow/cadence/tools/analysis"
)

func TestCheckSwitchStatementDuplicateCases(t *testing.T) {

	t.Parallel()

	t.Run("multiple duplicates", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let s: String? = nil
                switch s {
                    case "foo":
                        return 1
                    case "bar":
                        return 2
                    case "bar":
                        return 3
                    case "bar":
                        return 4
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		// Should only report two errors.
		// i.e: second and the third duplicate cases must not be compared with each other.
		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 254, Line: 9, Column: 25},
						EndPos:   ast.Position{Offset: 258, Line: 9, Column: 29},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 319, Line: 11, Column: 25},
						EndPos:   ast.Position{Offset: 323, Line: 11, Column: 29},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},

			diagnostics,
		)
	})

	t.Run("simple literals", func(t *testing.T) {
		t.Parallel()

		type test struct {
			name string
			expr string
		}

		expressions := []test{
			{
				name: "string",
				expr: "\"hello\"",
			},
			{
				name: "integer",
				expr: "5",
			},
			{
				name: "fixedpoint",
				expr: "4.7",
			},
			{
				name: "boolean",
				expr: "true",
			},
		}

		for _, testCase := range expressions {

			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				diagnostics := testAnalyzers(t,
					fmt.Sprintf(`
                        pub fun test(): Int {
                            let x = %[1]s
                            switch x {
                                case %[1]s:
                                    return 1
                                case %[1]s:
                                    return 2
                            }
                            return -1
                        }`,
						testCase.expr,
					),
					analyzers.SwitchCaseAnalyzer,
				)

				l := len(testCase.expr)

				require.Equal(
					t,
					[]analysis.Diagnostic{
						{
							Range: ast.Range{
								StartPos: ast.Position{Offset: 244 + 2*l, Line: 7, Column: 37},
								EndPos:   ast.Position{Offset: 244 + 3*l - 1, Line: 7, Column: 37 + l - 1},
							},
							Location: testLocation,
							Category: "lint",
							Message:  "duplicate switch case",
						},
					},
					diagnostics,
				)
			})
		}
	})

	t.Run("identifier", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x = 5
                let y = 5
                switch 4 {
                    case x:
                        return 1
                    case x:
                        return 2
                    case y:  // different identifier
                        return 3
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 200, Line: 8, Column: 25},
						EndPos:   ast.Position{Offset: 200, Line: 8, Column: 25},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("member access", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x = Foo()
                switch x.a {
                    case x.a:
                        return 1
                    case x.a:
                        return 2
                    case x.b:
                        return 3
                }
                return -1
            }
            pub struct Foo {
                pub var a: String
                pub var b: String
                init() {
                    self.a = "foo"
                    self.b = "bar"
                }
            }`,

			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 182, Line: 7, Column: 25},
						EndPos:   ast.Position{Offset: 184, Line: 7, Column: 27},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("index access", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x: [Int] = [1, 2, 3]
                let y: [Int] = [5, 6, 7]
                switch x[0] {
                    case x[1]:
                        return 1
                    case x[1]:
                        return 2
                    case x[2]:
                        return 3
                    case y[1]:
                        return 4
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 237, Line: 8, Column: 26},
						EndPos:   ast.Position{Offset: 239, Line: 8, Column: 28},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("conditional", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                switch "foo" {
                    case true ? "foo" : "bar":
                        return 1
                    case true ? "foo" : "bar":
                        return 2
                    case true ? "baz" : "bar":  // different then expr
                        return 3
                    case true ? "foo" : "baz":  // different else expr
                        return 4
                    case false ? "foo" : "bar":  // different condition expr
                        return 5
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 171, Line: 6, Column: 25},
						EndPos:   ast.Position{Offset: 190, Line: 6, Column: 44},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("unary", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x = 5
                let y = x
                switch x {
                    case -x:
                        return 1
                    case -x:
                        return 2
                    case -y:  // different rhs expr
                        return 3
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 201, Line: 8, Column: 25},
						EndPos:   ast.Position{Offset: 202, Line: 8, Column: 26},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("binary", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                switch 4 {
                    case 3+5:
                        return 1
                    case 3+5:
                        return 2
                    case 3+7:  // different rhs expr
                        return 3
                    case 7+5:  // different lhs expr
                        return 4
                    case 3-5:  // different operator
                        return 5
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 150, Line: 6, Column: 25},
						EndPos:   ast.Position{Offset: 152, Line: 6, Column: 27},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("cast", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x = 5
                let y = x as Integer
                switch y {
                    case x as Integer:
                        return 1
                    case x as Integer:
                        return 2
                    case x as! Integer:  // different operator
                        return 3
                    case y as Integer:  // different expr
                        return 4
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 222, Line: 8, Column: 25},
						EndPos:   ast.Position{Offset: 233, Line: 8, Column: 36},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("create", func(t *testing.T) {
		t.Parallel()

		_, err := checker.ParseAndCheck(t, `
            pub fun test() {
                let x <- create Foo()
                switch x {
                }
                destroy x
            }
            pub resource Foo {}
        `)

		errs := checker.ExpectCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.NotEquatableTypeError{}, errs[0])
	})

	t.Run("destroy", func(t *testing.T) {
		t.Parallel()

		_, err := checker.ParseAndCheck(t, `
          pub fun test() {
              let x <- create Foo()
              switch destroy x {
              }
          }
          pub resource Foo {}
        `)

		errs := checker.ExpectCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.NotEquatableTypeError{}, errs[0])
	})

	t.Run("reference", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x: Int = 5
                let y: Int = 7
                switch (&x as &Int) {
                    case &x as &Int:
                        return 1
                    case &x as &Int:
                        return 2
                    case &y as &Int:  // different expr
                        return 2
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 230, Line: 8, Column: 25},
						EndPos:   ast.Position{Offset: 239, Line: 8, Column: 34},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("force", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                let x: Int? = 5
                let y: Int? = 5
                switch 4 {
                    case x!:
                        return 1
                    case x!:
                        return 2
                    case y!:    // different expr
                        return 3
                }
                return -1
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 213, Line: 8, Column: 25},
						EndPos:   ast.Position{Offset: 214, Line: 8, Column: 26},
					},
					Location: testLocation,
					Category: "lint",
					Message:  "duplicate switch case",
				},
			},
			diagnostics,
		)
	})

	t.Run("path", func(t *testing.T) {
		t.Parallel()

		_, err := checker.ParseAndCheck(t, `
            pub fun test() {
                switch /public/somepath {
                }
            }
        `)

		errs := checker.ExpectCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.NotEquatableTypeError{}, errs[0])
	})

	t.Run("invocation", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                switch "hello" {
                    case foo():
                        return 1
                    case foo():
                        return 2
                }
                return -1
            }

            pub fun foo(): String {
                return "hello"
            }`,

			analyzers.SwitchCaseAnalyzer,
		)

		assert.Empty(t, diagnostics)
	})

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t, `
            pub fun test(): Int {
                switch "hello" {
                    default:
                        return -1
                }
            }`,
			analyzers.SwitchCaseAnalyzer,
		)

		assert.Empty(t, diagnostics)
	})
}
