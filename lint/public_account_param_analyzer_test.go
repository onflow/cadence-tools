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

func TestPublicAccountParamAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("public function with auth Account param", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun dangerousFunction(acct: auth(Storage) &Account) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'dangerousFunction' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("public function with multiple auth entitlements", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun setup(acct: auth(Storage, Capabilities) &Account) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'setup' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("public function with unentitled Account ref", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun safeFunction(acct: &Account) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		// Unentitled &Account references are safe — not flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("private function with auth Account param", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(self) fun internalSetup(acct: auth(Storage) &Account) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		// Private functions are not flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("access(contract) function with auth Account param", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(contract) fun contractSetup(acct: auth(Storage) &Account) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		// access(contract) functions are not flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("public function without Account param", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun normalFunction(x: Int) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("multiple params with one auth Account", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun mixedParams(x: Int, acct: auth(Storage) &Account, y: String) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'mixedParams' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("array of auth Account references", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun batchSetup(accts: [auth(Storage) &Account]) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'batchSetup' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("dictionary value with auth Account reference", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun lookup(accts: {String: auth(Storage) &Account}) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'lookup' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("optional array of auth Account references", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun maybeSetup(accts: [auth(Storage) &Account]?) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'maybeSetup' of contract 'MyContract' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   3,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   3,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("array of unentitled Account references", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) fun safeArray(accts: [&Account]) {}
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("nested composite public function with auth Account param", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) struct Nested {
                        access(all) fun setup(acct: auth(Storage) &Account) {}
                    }
                }
            `,
			lint.PublicAccountParamAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "public function 'setup' of structure 'MyContract.Nested' accepts an authorized Account reference — this grants callers broad account access, consider using capabilities instead",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   4,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   4,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})
}
