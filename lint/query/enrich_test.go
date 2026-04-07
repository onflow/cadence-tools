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

package query_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint/query"
)

const loadMode = analysis.NeedTypes | analysis.NeedExtendedElaboration | analysis.NeedPositionInfo

var testLocation = common.StringLocation("test")

func loadProgram(t *testing.T, code string) *analysis.Program {
	t.Helper()
	config := analysis.NewSimpleConfig(
		loadMode,
		map[common.Location][]byte{
			testLocation: []byte(code),
		},
		nil,
		nil,
	)
	programs, err := analysis.Load(config, testLocation)
	require.NoError(t, err)
	return programs.Get(testLocation)
}

func TestEnrichProgramHasType(t *testing.T) {
	t.Parallel()

	code := `
		access(all) fun main() {
			let x: AnyStruct = 42
			let y = x as! Int
		}
	`
	program := loadProgram(t, code)
	tree, err := query.EnrichProgram(program)
	require.NoError(t, err)
	require.NotNil(t, tree)

	// The tree should be a map with "Type": "Program"
	m, ok := tree.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Program", m["Type"])
}

func TestEnrichCastingExpressionHasOperation(t *testing.T) {
	t.Parallel()

	code := `
		access(all) fun main() {
			let x: AnyStruct = 42
			let y = x as! Int
		}
	`
	program := loadProgram(t, code)
	tree, err := query.EnrichProgram(program)
	require.NoError(t, err)

	// Find the CastingExpression in the tree
	var castNode map[string]any
	findNode(tree, "CastingExpression", &castNode)
	require.NotNil(t, castNode, "should find CastingExpression node")

	assert.Equal(t, "as!", castNode["Operation"])
	assert.Equal(t, "Int", castNode["TargetType"])
}

func TestEnrichCastingExpressionHasActualType(t *testing.T) {
	t.Parallel()

	code := `
		access(all) fun main() {
			let x: AnyStruct = 42
			let y = x as! Int
		}
	`
	program := loadProgram(t, code)
	tree, err := query.EnrichProgram(program)
	require.NoError(t, err)

	var castNode map[string]any
	findNode(tree, "CastingExpression", &castNode)
	require.NotNil(t, castNode)

	assert.Equal(t, "Int", castNode["ActualType"])
}

func TestEnrichExpressionHasActualType(t *testing.T) {
	t.Parallel()

	code := `
		access(all) fun main() {
			let x: AnyStruct = 42
			let y = x as! Int
		}
	`
	program := loadProgram(t, code)
	tree, err := query.EnrichProgram(program)
	require.NoError(t, err)

	// Find the IdentifierExpression for "x" inside the cast
	var castNode map[string]any
	findNode(tree, "CastingExpression", &castNode)
	require.NotNil(t, castNode)

	expr, ok := castNode["Expression"].(map[string]any)
	require.True(t, ok, "Expression should be a map")
	assert.Equal(t, "AnyStruct", expr["ActualType"])
}

func TestEnrichFailableCast(t *testing.T) {
	t.Parallel()

	code := `
		access(all) fun main() {
			let x: AnyStruct = 42
			let y = x as? Int
		}
	`
	program := loadProgram(t, code)
	tree, err := query.EnrichProgram(program)
	require.NoError(t, err)

	var castNode map[string]any
	findNode(tree, "CastingExpression", &castNode)
	require.NotNil(t, castNode)

	assert.Equal(t, "as?", castNode["Operation"])
}

// findNode recursively searches a JSON tree for a node with the given Type.
func findNode(node any, nodeType string, result *map[string]any) {
	if *result != nil {
		return
	}
	switch n := node.(type) {
	case map[string]any:
		if t, ok := n["Type"].(string); ok && t == nodeType {
			*result = n
			return
		}
		for _, v := range n {
			findNode(v, nodeType, result)
		}
	case []any:
		for _, v := range n {
			findNode(v, nodeType, result)
		}
	}
}
