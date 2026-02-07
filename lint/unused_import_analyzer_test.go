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
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestUnusedImportAnalyzer(t *testing.T) {
	t.Parallel()

	t.Run("explicit import, used", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				Foo.doSomething()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("explicit import, unused", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				// Foo is not used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Len(t, diagnostics, 1)

		expected := analysis.Diagnostic{
			Range: ast.Range{
				StartPos: ast.Position{Offset: 11, Line: 2, Column: 10},
				EndPos:   ast.Position{Offset: 13, Line: 2, Column: 12},
			},
			Location: testLocation,
			Category: lint.UnusedImportCategory,
			Message:  "unused import 'Foo'",
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "Remove unused import",
					TextEdits: []analysis.TextEdit{
						{
							Range: ast.Range{
								StartPos: ast.Position{Offset: 1, Line: 2, Column: 0},
								EndPos:   ast.Position{Offset: 24, Line: 3, Column: 0},
							},
							Replacement: "",
						},
					},
				},
			},
		}
		require.Equal(t, expected, diagnostics[0])

		// Apply the fix and verify
		result := diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code)
		require.Equal(t, `

			access(all) fun main() {
				// Foo is not used
			}
		`, result)
	})

	t.Run("multiple explicit imports, some unused", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo, Bar, Baz from 0x01

			access(all) fun main() {
				Foo.doSomething()
				// Bar is not used
				Baz.doSomething()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 16, Line: 2, Column: 15},
						EndPos:   ast.Position{Offset: 18, Line: 2, Column: 17},
					},
					Location: testLocation,
					Category: lint.UnusedImportCategory,
					Message:  "unused import 'Bar'",
					// No suggested fix for multiple imports (comma handling is complex)
					SuggestedFixes: nil,
				},
			},
			diagnostics,
		)
	})

	t.Run("implicit import, all used", func(t *testing.T) {
		t.Parallel()

		const code = `
			import 0x01

			access(all) fun main() {
				Foo.doSomething()
				Bar.doSomething()
				Baz.doSomething()
				ContractA.funcA()
				ContractB.funcB()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("implicit import, some unused", func(t *testing.T) {
		t.Parallel()

		const code = `
			import 0x01

			access(all) fun main() {
				ContractA.funcA()
				// ContractB, Foo, Bar, Baz are not used, but that's OK
				// because at least one import (ContractA) is used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		// For implicit imports, we don't report unused individual imports
		// as long as at least one import from the declaration is used
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("implicit import, all unused", func(t *testing.T) {
		t.Parallel()

		const code = `
			import 0x01

			access(all) fun main() {
				// None of the imports are used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		// When ALL imports from an implicit import are unused, we report it
		require.Len(t, diagnostics, 1)

		expected := analysis.Diagnostic{
			Range: ast.Range{
				StartPos: ast.Position{Offset: 4, Line: 2, Column: 3},
				EndPos:   ast.Position{Offset: 14, Line: 2, Column: 13},
			},
			Location: testLocation,
			Category: lint.UnusedImportCategory,
			Message:  "unused import",
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "Remove unused import",
					TextEdits: []analysis.TextEdit{
						{
							Range: ast.Range{
								StartPos: ast.Position{Offset: 1, Line: 2, Column: 0},
								EndPos:   ast.Position{Offset: 15, Line: 3, Column: 0},
							},
							Replacement: "",
						},
					},
				},
			},
		}
		require.Equal(t, expected, diagnostics[0])

		// Apply the fix and verify
		result := diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code)
		require.Equal(t, `

			access(all) fun main() {
				// None of the imports are used
			}
		`, result)
	})

	t.Run("aliased import, used", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo as Bar from 0x01

			access(all) fun main() {
				Bar.doSomething()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("aliased import, unused", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo as Bar from 0x01

			access(all) fun main() {
				// Bar is not used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Len(t, diagnostics, 1)

		expected := analysis.Diagnostic{
			Range: ast.Range{
				StartPos: ast.Position{Offset: 11, Line: 2, Column: 10},
				EndPos:   ast.Position{Offset: 13, Line: 2, Column: 12},
			},
			Location: testLocation,
			Category: lint.UnusedImportCategory,
			Message:  "unused import 'Bar'",
			SuggestedFixes: []analysis.SuggestedFix{
				{
					Message: "Remove unused import",
					TextEdits: []analysis.TextEdit{
						{
							Range: ast.Range{
								StartPos: ast.Position{Offset: 1, Line: 2, Column: 0},
								EndPos:   ast.Position{Offset: 31, Line: 3, Column: 0},
							},
							Replacement: "",
						},
					},
				},
			},
		}
		require.Equal(t, expected, diagnostics[0])

		// Apply the fix and verify
		result := diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code)
		require.Equal(t, `

			access(all) fun main() {
				// Bar is not used
			}
		`, result)
	})

	t.Run("used in type annotation, variable", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: Foo.SomeStruct = Foo.createStruct()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in type annotation, function parameter", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun process(value: Foo.SomeStruct) {
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in type annotation, function return", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun makeStruct(): Foo.SomeStruct {
				return Foo.createStruct()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in type annotation, field declaration", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) contract MyContract {
				access(all) var field: Foo.SomeStruct

				init() {
					self.field = Foo.createStruct()
				}
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in cast", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: AnyStruct = Foo.createStruct()
				let y = x as! Foo.SomeStruct
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in optional type", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: Foo.SomeStruct? = nil
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in array type", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: [Foo.SomeStruct] = []
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in dictionary type", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: {String: Foo.SomeStruct} = {}
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("used in reference type", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				var s = Foo.createStruct()
				let x: &Foo.SomeStruct = &s as &Foo.SomeStruct
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("multiple imports, deterministic order", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01
			import Bar from 0x01
			import Baz from 0x01

			access(all) fun main() {
				// None are used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 11, Line: 2, Column: 10},
						EndPos:   ast.Position{Offset: 13, Line: 2, Column: 12},
					},
					Location: testLocation,
					Category: lint.UnusedImportCategory,
					Message:  "unused import 'Foo'",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove unused import",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 1, Line: 2, Column: 0},
										EndPos:   ast.Position{Offset: 24, Line: 3, Column: 0},
									},
									Replacement: "",
								},
							},
						},
					},
				},
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 35, Line: 3, Column: 10},
						EndPos:   ast.Position{Offset: 37, Line: 3, Column: 12},
					},
					Location: testLocation,
					Category: lint.UnusedImportCategory,
					Message:  "unused import 'Bar'",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove unused import",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 25, Line: 3, Column: 0},
										EndPos:   ast.Position{Offset: 48, Line: 4, Column: 0},
									},
									Replacement: "",
								},
							},
						},
					},
				},
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 59, Line: 4, Column: 10},
						EndPos:   ast.Position{Offset: 61, Line: 4, Column: 12},
					},
					Location: testLocation,
					Category: lint.UnusedImportCategory,
					Message:  "unused import 'Baz'",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove unused import",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{Offset: 49, Line: 4, Column: 0},
										EndPos:   ast.Position{Offset: 72, Line: 5, Column: 0},
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
	})

	t.Run("mixed usage patterns", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01
			import Bar from 0x01
			import Baz from 0x01

			access(all) fun main() {
				// Foo used in expression
				Foo.doSomething()

				// Baz used in type annotation
				let x: Baz.SomeStruct = Baz.createStruct()

				// Bar is not used
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Len(t, diagnostics, 1)
		require.Equal(t,
			analysis.Diagnostic{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 35, Line: 3, Column: 10},
					EndPos:   ast.Position{Offset: 37, Line: 3, Column: 12},
				},
				Location: testLocation,
				Category: lint.UnusedImportCategory,
				Message:  "unused import 'Bar'",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "Remove unused import",
						TextEdits: []analysis.TextEdit{
							{
								Range: ast.Range{
									StartPos: ast.Position{Offset: 25, Line: 3, Column: 0},
									EndPos:   ast.Position{Offset: 48, Line: 4, Column: 0},
								},
								Replacement: "",
							},
						},
					},
				},
			},
			diagnostics[0],
		)
	})

	t.Run("no imports", func(t *testing.T) {
		t.Parallel()

		const code = `
			access(all) fun main() {
				// No imports
			}
		`

		diagnostics := testAnalyzers(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("nested type reference", func(t *testing.T) {
		t.Parallel()

		const code = `
			import Foo from 0x01

			access(all) fun main() {
				let x: Foo.SomeStruct = Foo.createStruct()
			}
		`

		diagnostics := testAnalyzersWithImports(t, code, lint.UnusedImportAnalyzer)
		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}

// Helper function for tests that need imports
func testAnalyzersWithImports(t *testing.T, code string, analyzers ...*analysis.Analyzer) []analysis.Diagnostic {
	address := common.MustBytesToAddress([]byte{0x01})

	// Create various contracts that tests can import
	fooContract := `
		access(all) contract Foo {
			access(all) struct SomeStruct {}

			access(all) fun doSomething() {}

			access(all) fun createStruct(): SomeStruct {
				return SomeStruct()
			}
		}
	`

	barContract := `
		access(all) contract Bar {
			access(all) struct SomeStruct {}

			access(all) fun doSomething() {}

			access(all) fun createStruct(): SomeStruct {
				return SomeStruct()
			}
		}
	`

	bazContract := `
		access(all) contract Baz {
			access(all) struct SomeStruct {}

			access(all) fun doSomething() {}

			access(all) fun createStruct(): SomeStruct {
				return SomeStruct()
			}
		}
	`

	contractACode := `
		access(all) contract ContractA {
			access(all) fun funcA() {}
		}
	`

	contractBCode := `
		access(all) contract ContractB {
			access(all) fun funcB() {}
		}
	`

	config := analysis.NewSimpleConfig(
		lint.LoadMode,
		map[common.Location][]byte{
			testLocation: []byte(code),
			common.AddressLocation{
				Address: address,
				Name:    "Foo",
			}: []byte(fooContract),
			common.AddressLocation{
				Address: address,
				Name:    "Bar",
			}: []byte(barContract),
			common.AddressLocation{
				Address: address,
				Name:    "Baz",
			}: []byte(bazContract),
			common.AddressLocation{
				Address: address,
				Name:    "ContractA",
			}: []byte(contractACode),
			common.AddressLocation{
				Address: address,
				Name:    "ContractB",
			}: []byte(contractBCode),
		},
		map[common.Address][]string{
			address: {"Foo", "Bar", "Baz", "ContractA", "ContractB"},
		},
		nil,
	)

	programs, err := analysis.Load(config, testLocation)
	require.NoError(t, err)

	var diagnostics []analysis.Diagnostic

	programs.Get(testLocation).Run(
		analyzers,
		func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	)

	return diagnostics
}
