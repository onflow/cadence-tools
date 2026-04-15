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

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestDisableNextLineAll(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Empty(t, diagnostics)
	})

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 1)
	})
}

func TestDisableNextLineSpecificMatch(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next unnecessary-force
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Empty(t, diagnostics)
	})

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 1)
	})
}

func TestDisableNextLineSpecificNoMatch(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next redundant-cast
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 1)
	})

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 1)
	})
}

func TestDisableNextLineMultipleAnalyzers(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next redundant-cast, unnecessary-force
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Empty(t, diagnostics)
	})

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 1)
	})
}

func TestDisableNextLinePermissiveAccess(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract MyContract {
				// lint-disable-next permissive-access
				access(all) var balance: UFix64

				init() {
					self.balance = 0.0
				}
			}
			`,
			lint.PermissiveAccessAnalyzer,
		)

		require.Empty(t, diagnostics)
	})

	t.Run("not disabled", func(t *testing.T) {
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
	})
}

func TestDisableNextLineSelectiveFiltering(t *testing.T) {

	t.Parallel()

	// `let y = x! as Int` triggers both unnecessary-force (x is non-optional)
	// and redundant-cast (Int as Int). Disabling one must not suppress the other.

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x! as Int
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
			lint.RedundantCastAnalyzer,
		)

		require.Len(t, diagnostics, 2)
	})

	t.Run("disable one", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next unnecessary-force
					let y = x! as Int
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
			lint.RedundantCastAnalyzer,
		)

		require.Len(t, diagnostics, 1)
		require.Equal(t, lint.UnnecessaryCastCategory, diagnostics[0].Category)
	})

	t.Run("disable other", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next redundant-cast
					let y = x! as Int
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
			lint.RedundantCastAnalyzer,
		)

		require.Len(t, diagnostics, 1)
		require.Equal(t, lint.RemovalCategory, diagnostics[0].Category)
	})

	t.Run("disable both", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next unnecessary-force, redundant-cast
					let y = x! as Int
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
			lint.RedundantCastAnalyzer,
		)

		require.Empty(t, diagnostics)
	})

	t.Run("disable all", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next
					let y = x! as Int
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
			lint.RedundantCastAnalyzer,
		)

		require.Empty(t, diagnostics)
	})
}

func TestDisableNextLineDoesNotAffectOtherLines(t *testing.T) {

	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					// lint-disable-next
					let y = x!
					let z = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		// Only the second force (z = x!) should produce a diagnostic
		require.Len(t, diagnostics, 1)
	})

	t.Run("not disabled", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) fun test() {
					let x = 3
					let y = x!
					let z = x!
				}
			}
			`,
			lint.UnnecessaryForceAnalyzer,
		)

		require.Len(t, diagnostics, 2)
	})
}
