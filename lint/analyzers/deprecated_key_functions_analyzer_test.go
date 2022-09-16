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

package analyzers_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-lint/analyzers"
)

func TestDeprecatedKeyFunctionsAnalyzer(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
          pub contract Test {
              pub fun test(account: AuthAccount) {
                  account.addPublicKey([])
                  account.removePublicKey(0)
              }
          }
        `,
		analyzers.DeprecatedKeyFunctionsAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 108, Line: 4, Column: 26},
					EndPos:   ast.Position{Offset: 119, Line: 4, Column: 37},
				},
				Location:         testLocation,
				Category:         analyzers.UpdateCategory,
				Message:          "deprecated function 'addPublicKey' will get removed",
				SecondaryMessage: "replace with 'keys.add'",
			},
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 151, Line: 5, Column: 26},
					EndPos:   ast.Position{Offset: 165, Line: 5, Column: 40},
				},
				Location:         testLocation,
				Category:         analyzers.UpdateCategory,
				Message:          "deprecated function 'removePublicKey' will get removed",
				SecondaryMessage: "replace with 'keys.revoke'",
			},
		},
		diagnostics,
	)
}
