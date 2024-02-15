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

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestCadenceV1Analyzer(t *testing.T) {

	t.Parallel()

	t.Run("account.save()", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.save()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `save` has been replaced by the new Storage API.",
					Code:     "C1.0-StorageAPI-Save",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `storage.save`",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "storage.save",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 63,
											Line:   4,
											Column: 18,
										},
										EndPos: ast.Position{
											Offset: 66,
											Line:   4,
											Column: 21,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 66,
							Line:   4,
							Column: 21,
						},
					},
				}},
			diagnostics,
		)
	})

	t.Run("account.linkAccount()", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.linkAccount()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "[Cadence 1.0] `linkAccount` has been replaced by the Capability Controller API.",
					Code:           "C1.0-StorageAPI-LinkAccount",
					URL:            "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 73,
							Line:   4,
							Column: 28,
						},
					},
				}},
			diagnostics,
		)
	})

	t.Run("account.link()", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.link()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "[Cadence 1.0] `link` has been replaced by the Capability Controller API.",
					Code:           "C1.0-StorageAPI-Link",
					URL:            "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 66,
							Line:   4,
							Column: 21,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("account.unlink()", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.unlink()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "[Cadence 1.0] `unlink` has been replaced by the Capability Controller API.",
					Code:           "C1.0-StorageAPI-Unlink",
					URL:            "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 68,
							Line:   4,
							Column: 23,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("account.getCapability()", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.getCapability()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `getCapability` has been replaced by the Capability Controller API.",
					Code:     "C1.0-CapabilityAPI-GetCapability",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `capabilities.get`",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "capabilities.get",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 63,
											Line:   4,
											Column: 18,
										},
										EndPos: ast.Position{
											Offset: 75,
											Line:   4,
											Column: 30,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 75,
							Line:   4,
							Column: 30,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("account.getLinkTarget()", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.getLinkTarget()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "[Cadence 1.0] `getLinkTarget` has been replaced by the Capability Controller API.",
					Code:           "C1.0-CapabilityAPI-GetLinkTarget",
					URL:            "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 75,
							Line:   4,
							Column: 30,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("account.addPublicKey()", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.addPublicKey()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "[Cadence 1.0] `addPublicKey` has been removed in favour of the new Key Management API. Please use `keys.add` instead.",
					Code:           "C1.0-KeyAPI-AddPublicKey",
					URL:            "https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
					SuggestedFixes: []analysis.SuggestedFix{},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 74,
							Line:   4,
							Column: 29,
						},
					},
				},
			}, diagnostics,
		)
	})

	t.Run("account.removePublicKey()", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				init() {
					self.account.removePublicKey()
				}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `removePublicKey` has been removed in favour of the new Key Management API.\nPlease use `keys.revoke` instead.",
					Code:     "C1.0-KeyAPI-RemovePublicKey",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#deprecated-key-management-api-got-removed-60",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `keys.revoke`",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "keys.revoke",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 63,
											Line:   4,
											Column: 18,
										},
										EndPos: ast.Position{
											Offset: 77,
											Line:   4,
											Column: 32,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 63,
							Line:   4,
							Column: 18,
						},
						EndPos: ast.Position{
							Offset: 77,
							Line:   4,
							Column: 32,
						},
					},
				}},
			diagnostics,
		)
	})

	t.Run("resource destruction", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			access(all) contract Test {
				access(all) resource R {
					init() {}
					destroy() {
						// i'm bad :)
					}
				}
				init() {}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `destroy` keyword has been removed.  Sub-resources will now be implicitly destroyed with their parent.  A `ResourceDestroyed` event can be configured to be emitted to notify clients of the destruction.",
					Code:     "C1.0-ResourceDestruction",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#force-destruction-of-resources-101",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Remove code",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 81,
											Line:   5,
											Column: 5,
										},
										EndPos: ast.Position{
											Offset: 118,
											Line:   7,
											Column: 5,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 81,
							Line:   5,
							Column: 5,
						},
						EndPos: ast.Position{
							Offset: 118,
							Line:   7,
							Column: 5,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("AuthAccount type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			transaction () {
				prepare(signer: AuthAccount) {}
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `AuthAccount` has been removed in Cadence 1.0.  Please use an authorized `&Account` reference with necessary entitlements instead.",
					Code:     "C1.0-AuthAccount",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `&Account`",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "&Account",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 41,
											Line:   3,
											Column: 20,
										},
										EndPos: ast.Position{
											Offset: 51,
											Line:   3,
											Column: 30,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 41,
							Line:   3,
							Column: 20,
						},
						EndPos: ast.Position{
							Offset: 51,
							Line:   3,
							Column: 30,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("PublicAccount type", func(t *testing.T) {
		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub fun main(addr: Address) {
				let account: PublicAccount = getAccount(addr)
			}
			`,
			true,
			lint.CadenceV1Analyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "[Cadence 1.0] `PublicAccount` has been removed in Cadence 1.0.  Please use an `&Account` reference instead.",
					Code:     "C1.0-PublicAccount",
					URL:      "https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "Replace with `&Account`",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "&Account",
									Insertion:   "",
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: 51,
											Line:   3,
											Column: 17,
										},
										EndPos: ast.Position{
											Offset: 63,
											Line:   3,
											Column: 29,
										},
									},
								},
							},
						},
					},
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 51,
							Line:   3,
							Column: 17,
						},
						EndPos: ast.Position{
							Offset: 63,
							Line:   3,
							Column: 29,
						},
					},
				},
			},
			diagnostics,
		)
	})
}
