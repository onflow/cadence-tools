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

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-lint/analyzers"
)

func TestReferenceOperatorAnalyzer(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
          pub contract Test {
              pub fun test() {
                  let ref = &1 as! &Int
              }
          }
        `,
		analyzers.ReferenceOperatorAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 90, Line: 4, Column: 28},
					EndPos:   ast.Position{Offset: 100, Line: 4, Column: 38},
				},
				Location:         testLocation,
				Category:         analyzers.UpdateCategory,
				Message:          "incorrect reference operator used",
				SecondaryMessage: "use the 'as' operator",
			},
		},
		diagnostics,
	)
}
