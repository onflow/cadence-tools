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

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestPublicCapabilityFieldInContract(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    pub let my_capability : Capability?
			    init() {
				    self.my_capability = nil
			    }
		    }
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 48, Line: 3, Column: 7},
					EndPos:   ast.Position{Offset: 82, Line: 3, Column: 41},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to have public Capability fields.",
				SecondaryMessage: "Consider restricting access.",
			},
		},
		diagnostics,
	)
}

func TestPublicDictionaryFieldWithCapabilityValueType(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    pub let my_capability : {String: Capability?}
			    init() {
				    self.my_capability = {"key": nil}
			    }
		    }
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 48, Line: 3, Column: 7},
					EndPos:   ast.Position{Offset: 92, Line: 3, Column: 51},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to have public Capability fields.",
				SecondaryMessage: "Consider restricting access.",
			},
		},
		diagnostics,
	)
}

func TestPublicArrayFieldWithConcreteCapabilityType(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    pub resource interface MySecretStuffInterface {}
			    pub let array_capability : [Capability<&{MySecretStuffInterface}>]
			    init() {
				    self.array_capability = [self.account.link<&{MySecretStuffInterface}>(/public/Stuff, target: /storage/Stuff)!]
			    }
		}
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 104, Line: 4, Column: 7},
					EndPos:   ast.Position{Offset: 169, Line: 4, Column: 72},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to have public Capability fields.",
				SecondaryMessage: "Consider restricting access.",
			},
		},
		diagnostics,
	)
}

func TestPrivateCapabilityAndNonCapabilityTypeFieldInContract(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    priv let my_capability : Capability?
				pub let data: Int
			    init() {
				    self.my_capability = nil
					self.data = 42
			    }
		    }
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic(nil),
		diagnostics,
	)
}
