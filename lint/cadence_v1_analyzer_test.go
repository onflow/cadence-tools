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

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

func TestCadenceV1Analyzer(t *testing.T) {

	t.Parallel()

	t.Run("account.save()", func(t *testing.T) {

		t.Parallel()

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.save()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`save` has been replaced by the new Storage API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.linkAccount()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "`linkAccount` has been replaced by the Capability Controller API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.link()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "`link` has been replaced by the Capability Controller API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.unlink()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "`unlink` has been replaced by the Capability Controller API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.getCapability()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`getCapability` has been replaced by the Capability Controller API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.getLinkTarget()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "`getLinkTarget` has been replaced by the Capability Controller API.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.addPublicKey()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location:       testLocation,
					Category:       lint.CadenceV1Category,
					Message:        "`addPublicKey` has been removed in favour of the new Key Management API. Please use `keys.add` instead.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) contract Test {
				init() {
					self.account.removePublicKey()
				}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredMemberError *sema.NotDeclaredMemberError
		require.ErrorAs(t, err, &notDeclaredMemberError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`removePublicKey` has been removed in favour of the new Key Management API.\nPlease use `keys.revoke` instead.",
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

		var checkerErr *sema.CheckerError
		var parserError errors.ParentError
		diagnostics := testAnalyzersAdvanced(t,
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
			func(config *analysis.Config) {
				config.HandleCheckerError = func(err analysis.ParsingCheckingError, checker *sema.Checker) error {
					require.Equal(t, err.ImportLocation(), testLocation)
					require.NotNil(t, checker)
					require.ErrorAs(t, err, &checkerErr)
					return nil
				}
				config.HandleParserError = func(err analysis.ParsingCheckingError, program *ast.Program) error {
					require.Equal(t, err.ImportLocation(), testLocation)
					require.ErrorAs(t, err, &parserError)
					return nil
				}
			},
			lint.CadenceV1Analyzer,
		)

		// Ensure that the checker error and parser error exist
		require.NotNil(t, checkerErr)
		require.NotNil(t, parserError)

		// Ensure that the checker error is of the correct type
		var unknownSpecialFunctionError *sema.UnknownSpecialFunctionError
		require.Len(t, checkerErr.ChildErrors(), 1)
		require.ErrorAs(t, checkerErr, &unknownSpecialFunctionError)

		// Ensure that the parser error is of the correct type
		var invalidDestructorError *parser.CustomDestructorError
		require.Len(t, parserError.ChildErrors(), 1)
		require.ErrorAs(t, parserError, &invalidDestructorError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`destroy` keyword has been removed.  Nested resources will now be implicitly destroyed with their parent.  A `ResourceDestroyed` event can be configured to be emitted to notify clients of the destruction.",
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

		var notDeclaredError *sema.NotDeclaredError
		diagnostics, checkerErr := testAnalyzersWithCheckerError(t,
			`
			transaction () {
				prepare(signer: AuthAccount) {}
			}
			`,
			lint.CadenceV1Analyzer,
		)
		require.ErrorAs(t, checkerErr, &notDeclaredError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`AuthAccount` has been removed in Cadence 1.0.  Please use an authorized `&Account` reference with necessary entitlements instead.",
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

		diagnostics, err := testAnalyzersWithCheckerError(t,
			`
			access(all) fun main(addr: Address) {
				let account: PublicAccount = getAccount(addr)
			}
			`,
			lint.CadenceV1Analyzer,
		)

		var notDeclaredError *sema.NotDeclaredError
		require.ErrorAs(t, err, &notDeclaredError)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.CadenceV1Category,
					Message:  "`PublicAccount` has been removed in Cadence 1.0.  Please use an `&Account` reference instead.",
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
											Offset: 59,
											Line:   3,
											Column: 17,
										},
										EndPos: ast.Position{
											Offset: 71,
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
							Offset: 59,
							Line:   3,
							Column: 17,
						},
						EndPos: ast.Position{
							Offset: 71,
							Line:   3,
							Column: 29,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("no error with empty transaction", func(t *testing.T) {
		t.Parallel()

		// Ensure that no error is reported/no panic for an empty transaction
		// I.e. does not throw with an empty function parameter list
		diagnostics := testAnalyzers(t,
			`
			transaction {
				prepare(signer: &Account) {}
				execute {}
			}
			`,
			lint.CadenceV1Analyzer,
		)

		require.Len(t, diagnostics, 0)
	})

	t.Run("no error when no type annotation", func(t *testing.T) {
		t.Parallel()

		// Ensure that no error is reported/no panic when there is no type annotation
		// I.e. does not throw when type for declaration is inferred
		diagnostics := testAnalyzers(t,
			`
			access(all) fun main() {
				let foo = 1
			}
			`,
			lint.CadenceV1Analyzer,
		)

		require.Len(t, diagnostics, 0)
	})
}
