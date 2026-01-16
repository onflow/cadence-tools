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

func TestCyclomaticComplexityAnalyzer_SimpleFunction(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun simple() {
            let x = 1
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "simple" has cyclomatic complexity 1 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 30, Line: 2, Column: 29},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_SingleIf(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withIf() {
            if true {
                let x = 1
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withIf" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 30, Line: 2, Column: 29},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_IfElseIfChain(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withIfElseIf(x: Int) {
            if x == 1 {
                let a = 1
            } else if x == 2 {
                let b = 2
            } else {
                let c = 3
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withIfElseIf" has cyclomatic complexity 3 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 36, Line: 2, Column: 35},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_NestedIf(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun nestedIf() {
            if true {
                if false {
                    let x = 1
                }
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "nestedIf" has cyclomatic complexity 3 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 32, Line: 2, Column: 31},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ForLoop(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withFor() {
            let items = [1, 2, 3]
            for item in items {
                let x = item
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withFor" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 31, Line: 2, Column: 30},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_WhileLoop(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withWhile() {
            var x = 0
            while x < 10 {
                x = x + 1
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withWhile" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 33, Line: 2, Column: 32},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_SwitchStatement(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withSwitch(x: Int) {
            switch x {
            case 1:
                let a = 1
            case 2:
                let b = 2
            case 3:
                let c = 3
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withSwitch" has cyclomatic complexity 5 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 34, Line: 2, Column: 33},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_LogicalOperatorsEnabled(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withLogicalOperators(a: Bool, b: Bool, c: Bool) {
            if a && b || c {
                let x = 1
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withLogicalOperators" has cyclomatic complexity 4 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 44, Line: 2, Column: 43},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_LogicalOperatorsDisabled(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun withLogicalOperators(a: Bool, b: Bool, c: Bool) {
            if a && b || c {
                let x = 1
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "withLogicalOperators" has cyclomatic complexity 2 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 44, Line: 2, Column: 43},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_SpecialFunctionInit(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) resource R {
            access(all) let value: Int

            init(value: Int) {
                if value > 0 {
                    self.value = value
                } else {
                    self.value = 0
                }
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "init" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 86, Line: 5, Column: 12},
					EndPos:   ast.Position{Offset: 89, Line: 5, Column: 15},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ComplexFunction(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun complex(x: Int): Int {
            var result = 0

            if x > 0 {
                var i = 0
                while i < x {
                    for j in [1, 2, 3] {
                        if j > 1 {
                            result = result + j
                        }
                    }
                    i = i + 1
                }
            }

            switch result {
            case 0:
                return 0
            case 1:
                return 1
            }

            return result
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "complex" has cyclomatic complexity 8 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 31, Line: 2, Column: 30},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ThresholdFiltering(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(2, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun simple() {
            if true {
                let x = 1
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic(nil),
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ThresholdExceeded(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(2, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun moreThanThreshold() {
            if true {
                let x = 1
            }
            if false {
                let y = 2
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "moreThanThreshold" has cyclomatic complexity 3 (threshold: 2)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 41, Line: 2, Column: 40},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_MultipleFunctions(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(2, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun simple() {
            let x = 1
        }

        access(all) fun complex() {
            if true {
                let x = 1
            }
            if false {
                let y = 2
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "complex" has cyclomatic complexity 3 (threshold: 2)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 93, Line: 6, Column: 24},
					EndPos:   ast.Position{Offset: 99, Line: 6, Column: 30},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_EmptyFunction(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun empty() {
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "empty" has cyclomatic complexity 1 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 29, Line: 2, Column: 28},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_NestedFunctionsAnalyzedSeparately(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun outer() {
            if true {
                let x = 1
            }

            fun inner() {
                if false {
                    let y = 2
                }
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "outer" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 29, Line: 2, Column: 28},
				},
			},
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "inner" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 114, Line: 7, Column: 16},
					EndPos:   ast.Position{Offset: 118, Line: 7, Column: 20},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_LogicalOperatorsInReturnStatement(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun checkCondition(a: Bool, b: Bool, c: Bool): Bool {
            return a && b || c
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "checkCondition" has cyclomatic complexity 3 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 38, Line: 2, Column: 37},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_MultipleLogicalOperators(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, true)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun multipleLogical(a: Bool, b: Bool, c: Bool, d: Bool): Bool {
            if a && b && c || d {
                return true
            }
            return false
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "multipleLogical" has cyclomatic complexity 5 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 39, Line: 2, Column: 38},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_SwitchWithComplexCases(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun switchWithIf(x: Int) {
            switch x {
            case 1:
                if true {
                    let a = 1
                }
            case 2:
                let b = 2
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "switchWithIf" has cyclomatic complexity 5 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 36, Line: 2, Column: 35},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ForLoopWithIf(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun forWithIf() {
            for i in [1, 2, 3] {
                if i > 1 {
                    let x = i
                }
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "forWithIf" has cyclomatic complexity 3 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 33, Line: 2, Column: 32},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_ContractFunction(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(1, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) contract MyContract {
            access(all) fun complexMethod() {
                if true {
                    let x = 1
                }
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "complexMethod" has cyclomatic complexity 2 (threshold: 1)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 71, Line: 3, Column: 28},
					EndPos:   ast.Position{Offset: 83, Line: 3, Column: 40},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_FunctionExpression(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(0, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun createHandler(): fun(Int): Int {
            return fun(x: Int): Int {
                if x > 0 {
                    return x + 1
                }
                return 0
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "createHandler" has cyclomatic complexity 2 (threshold: 0)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 37, Line: 2, Column: 36},
				},
			},
		},
		diagnostics,
	)
}

func TestCyclomaticComplexityAnalyzer_FunctionExpressionWithComplexity(t *testing.T) {
	t.Parallel()

	analyzer := lint.NewCyclomaticComplexityAnalyzer(2, false)

	diagnostics := testAnalyzers(t,
		`
        access(all) fun test() {
            let complexClosure = fun(x: Int): Int {
                if x > 0 {
                    if x > 10 {
                        return x * 2
                    }
                }
                return x
            }
        }
        `,
		analyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Location: testLocation,
				Category: lint.ComplexityCategory,
				Message:  `function "test" has cyclomatic complexity 3 (threshold: 2)`,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 25, Line: 2, Column: 24},
					EndPos:   ast.Position{Offset: 28, Line: 2, Column: 27},
				},
			},
		},
		diagnostics,
	)
}
