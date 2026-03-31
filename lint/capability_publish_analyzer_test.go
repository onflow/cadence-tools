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

func TestCapabilityPublishAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("entitled capability publish", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) entitlement E

                access(all) fun main(acct: auth(Storage, Capabilities) &Account) {
                    let cap = acct.capabilities.storage.issue<auth(E) &Int>(/storage/test)
                    acct.capabilities.publish(cap, at: /public/test)
                }
            `,
			lint.CapabilityPublishAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message:  "entitled capability published — verify that the entitlements are intentional and scoped appropriately",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: diagnostics[0].StartPos.Offset,
							Line:   6,
							Column: diagnostics[0].StartPos.Column,
						},
						EndPos: ast.Position{
							Offset: diagnostics[0].EndPos.Offset,
							Line:   6,
							Column: diagnostics[0].EndPos.Column,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("unentitled capability publish", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) fun main(acct: auth(Storage, Capabilities) &Account) {
                    let cap = acct.capabilities.storage.issue<&Int>(/storage/test)
                    acct.capabilities.publish(cap, at: /public/test)
                }
            `,
			lint.CapabilityPublishAnalyzer,
		)

		// Unentitled capability publish should not be flagged
		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
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
