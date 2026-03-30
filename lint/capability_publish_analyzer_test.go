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

func TestCapabilityPublishAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("capability publish call", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main(acct: auth(Capabilities) &Account) {
                    let cap = acct.capabilities.storage.issue<&Int>(/storage/test)
                    acct.capabilities.publish(cap, at: /public/test)
                }
            `,
			lint.CapabilityPublishAnalyzer,
		)

		require.Len(t, diagnostics, 1)
		require.Equal(t, lint.SecurityCategory, diagnostics[0].Category)
		require.Contains(t, diagnostics[0].Message, "capability published")
	})

	t.Run("unrelated publish method", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct Publisher {
                    access(all) fun publish() {}
                }

                access(all) fun main() {
                    let p = Publisher()
                    p.publish()
                }
            `,
			lint.CapabilityPublishAnalyzer,
		)

		// Unrelated .publish() should not be flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
