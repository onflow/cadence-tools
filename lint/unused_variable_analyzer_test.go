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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestUnusedVariableAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("unused local variable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let unused = 5
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
						EndPos:   ast.Position{Offset: 67, Line: 3, Column: 27},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "variable 'unused' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_unused",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
										EndPos:   ast.Position{Offset: 67, Line: 3, Column: 27},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("used local variable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let used = 5
                  log(used)
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("underscore-prefixed variable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let _intentionallyUnused = 5
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("blank identifier", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let _ = 5
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("unused inner function parameter", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun outer() {
                  fun inner(used: Int, unused: String) {
                      log(used)
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 80, Line: 3, Column: 39},
						EndPos:   ast.Position{Offset: 85, Line: 3, Column: 44},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'unused' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Add parameter name with underscore prefix",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "unused _unused",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 80, Line: 3, Column: 39},
										EndPos:   ast.Position{Offset: 85, Line: 3, Column: 44},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("top-level function parameter", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun topLevel(unused: String) {}
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 40, Line: 2, Column: 39},
						EndPos:   ast.Position{Offset: 45, Line: 2, Column: 44},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'unused' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Add parameter name with underscore prefix",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "unused _unused",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 40, Line: 2, Column: 39},
										EndPos:   ast.Position{Offset: 45, Line: 2, Column: 44},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("underscore-prefixed parameter", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun outer() {
                  fun inner(_intentionallyUnused: Int) {}
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("parameter with argument label", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun outer() {
                  fun inner(foo bar: Int) {}
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 73, Line: 3, Column: 32},
						EndPos:   ast.Position{Offset: 75, Line: 3, Column: 34},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'bar' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix parameter name with underscore",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_bar",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 73, Line: 3, Column: 32},
										EndPos:   ast.Position{Offset: 75, Line: 3, Column: 34},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("parameter without argument label", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun outer() {
                  fun inner(foo: Int) {}
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 69, Line: 3, Column: 28},
						EndPos:   ast.Position{Offset: 71, Line: 3, Column: 30},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'foo' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Add parameter name with underscore prefix",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "foo _foo",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 69, Line: 3, Column: 28},
										EndPos:   ast.Position{Offset: 71, Line: 3, Column: 30},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("variable used multiple times", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let x = 5
                  log(x)
                  log(x)
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("variable shadowing", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let x = 5
                  if true {
                      let x = 10
                      log(x)
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
						EndPos:   ast.Position{Offset: 62, Line: 3, Column: 22},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "variable 'x' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_x",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
										EndPos:   ast.Position{Offset: 62, Line: 3, Column: 22},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("if-let binding unused", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test(opt: Int?) {
                  if let x = opt {
                      log("has value")
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 74, Line: 3, Column: 25},
						EndPos:   ast.Position{Offset: 74, Line: 3, Column: 25},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "variable 'x' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_x",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 74, Line: 3, Column: 25},
										EndPos:   ast.Position{Offset: 74, Line: 3, Column: 25},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("if-let binding used", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test(opt: Int?) {
                  if let x = opt {
                      log(x)
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("if-let binding with underscore", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test(opt: Int?) {
                  if let _x = opt {
                      log("has value")
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("multiple unused variables", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  let unused1 = 5
                  let unused2 = "hello"
                  let used = 10
                  log(used)
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
						EndPos:   ast.Position{Offset: 68, Line: 3, Column: 28},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "variable 'unused1' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_unused1",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 62, Line: 3, Column: 22},
										EndPos:   ast.Position{Offset: 68, Line: 3, Column: 28},
									},
								},
							},
						},
					},
				},
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 96, Line: 4, Column: 22},
						EndPos:   ast.Position{Offset: 102, Line: 4, Column: 28},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "variable 'unused2' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_unused2",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 96, Line: 4, Column: 22},
										EndPos:   ast.Position{Offset: 102, Line: 4, Column: 28},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("nested inner functions", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun outer() {
                  fun inner1(unused1: Int) {
                      fun inner2(unused2: String) {}
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 70, Line: 3, Column: 29},
						EndPos:   ast.Position{Offset: 76, Line: 3, Column: 35},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'unused1' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Add parameter name with underscore prefix",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "unused1 _unused1",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 70, Line: 3, Column: 29},
										EndPos:   ast.Position{Offset: 76, Line: 3, Column: 35},
									},
								},
							},
						},
					},
				},
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 119, Line: 4, Column: 33},
						EndPos:   ast.Position{Offset: 125, Line: 4, Column: 39},
					},
					Location: testLocation,
					Category: lint.UnusedVariableCategory,
					Message:  "parameter 'unused2' is declared but never used",
					SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
						{
							Message: "Add parameter name with underscore prefix",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "unused2 _unused2",
									Range: ast.Range{
										StartPos: ast.Position{Offset: 119, Line: 4, Column: 33},
										EndPos:   ast.Position{Offset: 125, Line: 4, Column: 39},
									},
								},
							},
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("variable in loop", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  for i in [1, 2, 3] {
                      log(i)
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("unused loop variable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
              access(all) fun test() {
                  for i in [1, 2, 3] {
                      log("iteration")
                  }
              }
            `,
			lint.UnusedVariableAnalyzer,
		)

		require.Equal(t,
			// TODO:
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("apply fix for unused variable", func(t *testing.T) {

		t.Parallel()

		code := `
              access(all) fun test() {
                  let unused = 5
              }
            `

		diagnostics := testAnalyzers(t, code, lint.UnusedVariableAnalyzer)

		require.Len(t, diagnostics, 1)

		expectedFixedCode := `
              access(all) fun test() {
                  let _unused = 5
              }
            `

		assert.Equal(t,
			expectedFixedCode,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("apply fix for parameter with argument label", func(t *testing.T) {

		t.Parallel()

		code := `
              access(all) fun outer() {
                  fun inner(foo bar: Int) {}
              }
            `

		diagnostics := testAnalyzers(t, code, lint.UnusedVariableAnalyzer)

		require.Len(t, diagnostics, 1)

		expectedFixedCode := `
              access(all) fun outer() {
                  fun inner(foo _bar: Int) {}
              }
            `

		assert.Equal(t,
			expectedFixedCode,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("apply fix for parameter without argument label", func(t *testing.T) {

		t.Parallel()

		code := `
              access(all) fun outer() {
                  fun inner(foo: Int) {}
              }
            `

		diagnostics := testAnalyzers(t, code, lint.UnusedVariableAnalyzer)

		require.Len(t, diagnostics, 1)

		expectedFixedCode := `
              access(all) fun outer() {
                  fun inner(foo _foo: Int) {}
              }
            `

		assert.Equal(t,
			expectedFixedCode,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})

	t.Run("apply fix for if-let binding", func(t *testing.T) {

		t.Parallel()

		code := `
              access(all) fun test(opt: Int?) {
                  if let x = opt {
                      log("has value")
                  }
              }
            `

		diagnostics := testAnalyzers(t, code, lint.UnusedVariableAnalyzer)

		require.Len(t, diagnostics, 1)

		expectedFixedCode := `
              access(all) fun test(opt: Int?) {
                  if let _x = opt {
                      log("has value")
                  }
              }
            `

		assert.Equal(t,
			expectedFixedCode,
			diagnostics[0].SuggestedFixes[0].TextEdits[0].ApplyTo(code),
		)
	})
}
