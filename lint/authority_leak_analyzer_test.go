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

func TestAuthorityLeakAnalyzer(t *testing.T) {

	t.Parallel()

	t.Run("public unentitled reference", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicUnentitledReference: &Int?
                    init() {
                        self.publicUnentitledReference = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("public entitled reference", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicEntitledReference: (auth(Mutate) &Int)?
                    init() {
                        self.publicEntitledReference = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message: "field 'publicEntitledReference' of structure 'S' exposes " +
						"(a capability of) an entitled reference, which may lead to authority leaks",
					SecondaryMessage: "consider restricting access to the field " +
						"or changing its type to avoid authority leaks",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 60,
							Line:   3,
							Column: 20,
						},
						EndPos: ast.Position{
							Offset: 120,
							Line:   3,
							Column: 80,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("public entitled reference container", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicEntitledReferences: [auth(Mutate) &Int]
                    init() {
                        self.publicEntitledReferences = []
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message: "field 'publicEntitledReferences' of structure 'S' exposes " +
						"(a capability of) an entitled reference, which may lead to authority leaks",
					SecondaryMessage: "consider restricting access to the field " +
						"or changing its type to avoid authority leaks",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 60,
							Line:   3,
							Column: 20,
						},
						EndPos: ast.Position{
							Offset: 120,
							Line:   3,
							Column: 80,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("private entitled reference", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(self) let privateEntitledReference: (auth(Mutate) &Int)?
                    init() {
                        self.privateEntitledReference = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("public unentitled capability", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicUnentitledCapability: Capability<&Int>?
                    init() {
                        self.publicUnentitledCapability = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

	t.Run("public entitled capability", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicEntitledCapability: Capability<auth(Mutate) &Int>?
                    init() {
                        self.publicEntitledCapability = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message: "field 'publicEntitledCapability' of structure 'S' exposes " +
						"(a capability of) an entitled reference, which may lead to authority leaks",
					SecondaryMessage: "consider restricting access to the field " +
						"or changing its type to avoid authority leaks",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 60,
							Line:   3,
							Column: 20,
						},
						EndPos: ast.Position{
							Offset: 131,
							Line:   3,
							Column: 91,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("public entitled capability container", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(all) let publicEntitledCapabilities: [Capability<auth(Mutate) &Int>]
                    init() {
                        self.publicEntitledCapabilities = []
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Location: testLocation,
					Category: lint.SecurityCategory,
					Message: "field 'publicEntitledCapabilities' of structure 'S' exposes " +
						"(a capability of) an entitled reference, which may lead to authority leaks",
					SecondaryMessage: "consider restricting access to the field " +
						"or changing its type to avoid authority leaks",
					Range: ast.Range{
						StartPos: ast.Position{
							Offset: 60,
							Line:   3,
							Column: 20,
						},
						EndPos: ast.Position{
							Offset: 134,
							Line:   3,
							Column: 94,
						},
					},
				},
			},
			diagnostics,
		)
	})

	t.Run("private entitled capability", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
                access(all) struct S {
                    access(self) let privateEntitledCapability: Capability<auth(Mutate) &Int>?
                    init() {
                        self.privateEntitledCapability = nil
                    }
                }
            `,
			lint.AuthorityLeakAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})

}
