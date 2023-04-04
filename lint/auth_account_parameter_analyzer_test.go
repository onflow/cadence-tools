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

func TestAuthAccountParameterAnalyzerInContractFunction(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract BalanceChecker {
		        pub fun getBalance(account: AuthAccount): UFix64 {
		            return account.balance
		        }
		    }
		`,
		lint.AuthAccountParameterAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 47, Line: 3, Column: 10},
					EndPos:   ast.Position{Offset: 145, Line: 5, Column: 10},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to pass AuthAccount to functions.",
				SecondaryMessage: "Consider using Capabilities instead.",
			},
		},
		diagnostics,
	)
}

func TestAuthAccountParameterAnalyzerInContractInitFunction(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract BalanceChecker {
		        pub let balance: UFix64
		        init(account: AuthAccount) {
		            self.balance = account.balance
		        }
		    }
		`,
		lint.AuthAccountParameterAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 81, Line: 4, Column: 10},
					EndPos:   ast.Position{Offset: 165, Line: 6, Column: 10},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to pass AuthAccount to functions.",
				SecondaryMessage: "Consider using Capabilities instead.",
			},
		},
		diagnostics,
	)
}

func TestAuthAccountParameterAnalyzerInStructInitFunction(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub struct Balance {
		        pub let balance: UFix64
		        init(account: AuthAccount) {
		            self.balance = account.balance
		        }
		    }
		`,
		lint.AuthAccountParameterAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 72, Line: 4, Column: 10},
					EndPos:   ast.Position{Offset: 156, Line: 6, Column: 10},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to pass AuthAccount to functions.",
				SecondaryMessage: "Consider using Capabilities instead.",
			},
		},
		diagnostics,
	)
}

func TestAuthAccountParameterAnalyzerWithPublicAccount(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract BalanceChecker {
		        pub fun getBalance(account: PublicAccount): UFix64 {
		            return account.balance
		        }
		    }
		`,
		lint.AuthAccountParameterAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic(nil),
		diagnostics,
	)
}
