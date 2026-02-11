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

package lint

import (
	goErrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/tools/analysis"
)

func TestEditsOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		editA    ast.TextEdit
		editB    ast.TextEdit
		expected bool
	}{
		{
			name: "non-overlapping edits",
			editA: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 5},
				},
			},
			editB: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 10},
					EndPos:   ast.Position{Offset: 15},
				},
			},
			expected: false,
		},
		{
			name: "overlapping edits",
			editA: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 10},
				},
			},
			editB: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 5},
					EndPos:   ast.Position{Offset: 15},
				},
			},
			expected: true,
		},
		{
			name: "adjacent edits (touching but not overlapping)",
			editA: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 5},
				},
			},
			editB: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 5},
					EndPos:   ast.Position{Offset: 10},
				},
			},
			expected: false,
		},
		{
			name: "nested edit (B inside A)",
			editA: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 20},
				},
			},
			editB: ast.TextEdit{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 5},
					EndPos:   ast.Position{Offset: 10},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := editsOverlap(tt.editA, tt.editB)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterOverlappingEdits(t *testing.T) {
	t.Parallel()

	t.Run("no overlapping edits", func(t *testing.T) {
		t.Parallel()

		edits := []ast.TextEdit{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 5},
				},
			},
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 10},
					EndPos:   ast.Position{Offset: 15},
				},
			},
		}

		result := filterOverlappingEdits(edits)
		assert.Equal(t, 2, len(result))
	})

	t.Run("overlapping edits - keep first", func(t *testing.T) {
		t.Parallel()

		edits := []ast.TextEdit{
			{
				Replacement: "first",
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 10},
				},
			},
			{
				Replacement: "second",
				Range: ast.Range{
					StartPos: ast.Position{Offset: 5},
					EndPos:   ast.Position{Offset: 15},
				},
			},
		}

		result := filterOverlappingEdits(edits)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, "first", result[0].Replacement)
	})

	t.Run("multiple overlapping edits", func(t *testing.T) {
		t.Parallel()

		edits := []ast.TextEdit{
			{
				Replacement: "first",
				Range: ast.Range{
					StartPos: ast.Position{Offset: 0},
					EndPos:   ast.Position{Offset: 10},
				},
			},
			{
				Replacement: "second",
				Range: ast.Range{
					StartPos: ast.Position{Offset: 5},
					EndPos:   ast.Position{Offset: 15},
				},
			},
			{
				Replacement: "third",
				Range: ast.Range{
					StartPos: ast.Position{Offset: 20},
					EndPos:   ast.Position{Offset: 25},
				},
			},
		}

		result := filterOverlappingEdits(edits)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "first", result[0].Replacement)
		assert.Equal(t, "third", result[1].Replacement)
	})
}

type testCodeManager struct {
	codes      map[common.Location]string
	readError  error
	writeError error
}

func newTestCodeManager(codes map[common.Location]string) testCodeManager {
	return testCodeManager{
		codes: codes,
	}
}

func (m testCodeManager) ReadCode(location common.Location) ([]byte, error) {
	if m.readError != nil {
		return nil, m.readError
	}
	code, ok := m.codes[location]
	if !ok {
		return nil, fmt.Errorf("location not found: %q", location.String())
	}

	return []byte(code), nil
}

func (m testCodeManager) WriteCode(location common.Location, code []byte) error {
	if m.writeError != nil {
		return m.writeError
	}
	m.codes[location] = string(code)
	return nil
}

func TestApplyFixes(t *testing.T) {
	t.Parallel()

	t.Run("single fix in single location", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let unused = 42",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "Prefix with underscore",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_unused",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 9},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _unused = 42",
			codeManager.codes[location],
		)
	})

	t.Run("multiple non-overlapping fixes in single location", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let unused = 42\nlet another = 99",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "Fix first",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_unused",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 9},
								},
							},
						},
					},
				},
			},
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "Fix second",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_another",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 20},
									EndPos:   ast.Position{Offset: 26},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _unused = 42\nlet _another = 99",
			codeManager.codes[location],
		)
	})

	t.Run("overlapping fixes - keeps first", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let value = 42",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "First fix",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_value",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 8},
								},
							},
						},
					},
				},
			},
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "Second fix (overlapping)",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "newValue",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 8},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _value = 42",
			codeManager.codes[location],
		)
	})

	t.Run("multiple locations", func(t *testing.T) {
		t.Parallel()

		location1 := common.StringLocation("test1")
		location2 := common.StringLocation("test2")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location1: "let unused1 = 1",
				location2: "let unused2 = 2",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location1,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_unused1",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 10},
								},
							},
						},
					},
				},
			},
			{
				Location: location2,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_unused2",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 10},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _unused1 = 1",
			codeManager.codes[location1],
		)
		assert.Equal(t,
			"let _unused2 = 2",
			codeManager.codes[location2],
		)
	})

	t.Run("read error", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(nil)

		const errorMessage = "read failed"
		codeManager.readError = goErrors.New(errorMessage)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							{
								Replacement: "new",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 0},
									EndPos:   ast.Position{Offset: 3},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.Error(t, err)

		assert.ErrorContains(t, err, errorMessage)
	})

	t.Run("write error", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let value = 42",
			},
		)

		const errorMessage = "write failed"
		codeManager.writeError = goErrors.New(errorMessage)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_value",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 9},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.Error(t, err)

		assert.ErrorContains(t, err, errorMessage)
	})

	t.Run("edits are applied in reverse order", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let first = 1\nlet second = 2",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							// First edit (earlier in file)
							{
								Replacement: "_first",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 8},
								},
							},
						},
					},
				},
			},
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						TextEdits: []ast.TextEdit{
							// Second edit (later in file)
							{
								Replacement: "_second",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 18},
									EndPos:   ast.Position{Offset: 23},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _first = 1\nlet _second = 2",
			codeManager.codes[location],
		)
	})

	t.Run("only use first suggested fix", func(t *testing.T) {
		t.Parallel()

		location := common.StringLocation("test")
		codeManager := newTestCodeManager(
			map[common.Location]string{
				location: "let value = 42",
			},
		)

		diagnostics := []analysis.Diagnostic{
			{
				Location: location,
				SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
					{
						Message: "First suggested fix",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "_value",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 8},
								},
							},
						},
					},
					{
						Message: "Second suggested fix (should be ignored)",
						TextEdits: []ast.TextEdit{
							{
								Replacement: "newValue",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 4},
									EndPos:   ast.Position{Offset: 8},
								},
							},
						},
					},
				},
			},
		}

		err := ApplyFixes(diagnostics, codeManager)
		require.NoError(t, err)

		assert.Equal(t,
			"let _value = 42",
			codeManager.codes[location],
		)
	})
}
