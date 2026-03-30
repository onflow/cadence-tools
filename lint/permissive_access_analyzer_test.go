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

		require.Len(t, diagnostics, 1)
		require.Equal(t, testLocation, diagnostics[0].Location)
		require.Equal(t, lint.SecurityCategory, diagnostics[0].Category)
		require.Contains(t, diagnostics[0].Message, "balance")
		require.Contains(t, diagnostics[0].Message, "access(all)")
		require.Equal(t, 3, diagnostics[0].StartPos.Line)
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

		require.Len(t, diagnostics, 2)
		require.Contains(t, diagnostics[0].Message, "balance")
		require.Contains(t, diagnostics[1].Message, "owner")
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

		require.Len(t, diagnostics, 1)
		require.Contains(t, diagnostics[0].Message, "balance")
		require.Contains(t, diagnostics[0].Message, "resource")
	})
}
