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

func TestPermissiveAccessAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("access(all) var field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) var balance: UFix64

                    init() {
                        self.balance = 0.0
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "mutable field 'balance' of contract 'MyContract' has access(all), allowing public write access — consider restricting with entitlements",
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

	t.Run("access(all) let field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) let name: String

                    init() {
                        self.name = "test"
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		// let fields should not be flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("access(self) var field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(self) var counter: Int

                    init() {
                        self.counter = 0
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		// access(self) fields should not be flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("multiple var fields in struct", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct Token {
                    access(all) var balance: UFix64
                    access(all) var owner: Address

                    init() {
                        self.balance = 0.0
                        self.owner = 0x1
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "mutable field 'balance' of structure 'Token' has access(all), allowing public write access — consider restricting with entitlements",
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
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "mutable field 'owner' of structure 'Token' has access(all), allowing public write access — consider restricting with entitlements",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[1].StartPos.Offset,
							Line:   4,
							Column: diagnostics[1].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[1].EndPos.Offset,
							Line:   4,
							Column: diagnostics[1].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("resource var field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) resource Vault {
                    access(all) var balance: UFix64

                    init(balance: UFix64) {
                        self.balance = balance
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "mutable field 'balance' of resource 'Vault' has access(all), allowing public write access — consider restricting with entitlements",
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

	t.Run("nested composite var field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) contract MyContract {
                    access(all) struct Nested {
                        access(all) var value: Int

                        init() {
                            self.value = 0
                        }
                    }
                }
            `,
			lint.PermissiveAccessAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "mutable field 'value' of structure 'MyContract.Nested' has access(all), allowing public write access — consider restricting with entitlements",
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
