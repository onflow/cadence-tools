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

func TestHardcodedAddressAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("hardcoded address in variable", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main() {
                    let addr: Address = 0x1234567890abcdef
                }
            `,
			lint.HardcodedAddressAnalyzer,
		)

		require.Len(t, diagnostics, 1)
		require.Equal(t, lint.SecurityCategory, diagnostics[0].Category)
		require.Contains(t, diagnostics[0].Message, "hardcoded address")
	})

	t.Run("short hex literal not flagged", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main() {
                    let x: UInt64 = 0xff
                }
            `,
			lint.HardcodedAddressAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("decimal integer not flagged", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main() {
                    let x: Int = 1234567890
                }
            `,
			lint.HardcodedAddressAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("multiple hardcoded addresses", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main() {
                    let a: Address = 0x1234567890abcdef
                    let b: Address = 0xabcdef1234567890
                }
            `,
			lint.HardcodedAddressAnalyzer,
		)

		require.Len(t, diagnostics, 2)
	})
}
