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

func TestDeprecatedMemberAnalyzer(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
          pub contract Test {
              /// **DEPRECATED**
              pub fun foo() {}

              /// Deprecated: No good
              pub fun bar() {}

              pub fun test(account: AuthAccount) {
                  account.addPublicKey([])
                  account.removePublicKey(0)
                  self.foo()
                  self.bar()
              }
          }
        `,
		lint.DeprecatedMemberAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 247, Line: 10, Column: 26},
					EndPos:   ast.Position{Offset: 258, Line: 10, Column: 37},
				},
				Location:         testLocation,
				Category:         lint.DeprecatedCategory,
				Message:          "function 'addPublicKey' is deprecated",
				SecondaryMessage: "Use `keys.add` instead.",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "replace",
						TextEdits: []analysis.TextEdit{
							{
								Replacement: "keys.add",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 247, Line: 10, Column: 26},
									EndPos:   ast.Position{Offset: 258, Line: 10, Column: 37},
								},
							},
						},
					},
				},
			},
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 290, Line: 11, Column: 26},
					EndPos:   ast.Position{Offset: 304, Line: 11, Column: 40},
				},
				Location:         testLocation,
				Category:         lint.DeprecatedCategory,
				Message:          "function 'removePublicKey' is deprecated",
				SecondaryMessage: "Use `keys.revoke` instead.",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "replace",
						TextEdits: []analysis.TextEdit{
							{
								Replacement: "keys.revoke",
								Range: ast.Range{
									StartPos: ast.Position{Offset: 290, Line: 11, Column: 26},
									EndPos:   ast.Position{Offset: 304, Line: 11, Column: 40},
								},
							},
						},
					},
				},
			},
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 332, Line: 12, Column: 23},
					EndPos:   ast.Position{Offset: 334, Line: 12, Column: 25},
				},
				Location: testLocation,
				Category: lint.DeprecatedCategory,
				Message:  "function 'foo' is deprecated",
			},
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 361, Line: 13, Column: 23},
					EndPos:   ast.Position{Offset: 363, Line: 13, Column: 25},
				},
				Location:         testLocation,
				Category:         lint.DeprecatedCategory,
				Message:          "function 'bar' is deprecated",
				SecondaryMessage: "No good",
			},
		},
		diagnostics,
	)
}
