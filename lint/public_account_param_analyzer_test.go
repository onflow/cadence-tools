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

		require.Len(t, diagnostics, 1)
		require.Equal(t, lint.SecurityCategory, diagnostics[0].Category)
		require.Contains(t, diagnostics[0].Message, "dangerousFunction")
		require.Contains(t, diagnostics[0].Message, "authorized Account reference")
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

		require.Len(t, diagnostics, 1)
		require.Contains(t, diagnostics[0].Message, "setup")
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

		require.Len(t, diagnostics, 1)
		require.Contains(t, diagnostics[0].Message, "mixedParams")
	})
}
