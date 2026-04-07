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

	"github.com/onflow/cadence-tools/lint/query"
)

func runQuery(t *testing.T, code string, queryStr string) []query.Match {
	t.Helper()
	program := loadProgram(t, code)
	matches, err := query.RunQuery(queryStr, program)
	require.NoError(t, err)
	return matches
}

func TestQueryFindAllCastingExpressions(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as? Int
          let z = x as! String
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(.Type? == "CastingExpression")
      | .Operation
    `)
	assert.ElementsMatch(t,
		[]query.Match{
			{Location: testLocation, Value: "as?", Code: []byte(code)},
			{Location: testLocation, Value: "as!", Code: []byte(code)},
		},
		matches,
	)
}

func TestQueryFindForceCasts(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as? Int
          let z = x as! String
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and .Operation == "as!"
        )
      | {Operation, SourceType, TargetType, ActualType}
    `)
	assert.Equal(t,
		[]query.Match{
			{
				Location: testLocation,
				Value: map[string]any{
					"Operation":  "as!",
					"SourceType": "AnyStruct",
					"TargetType": "String",
					"ActualType": "String",
				},
				Code: []byte(code),
			},
		},
		matches,
	)
}

func TestQueryFindFailableCasts(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as? Int
          let z = x as! String
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and .Operation == "as?"
        )
      | {Operation, SourceType, TargetType, ActualType}
    `)
	assert.Equal(t,
		[]query.Match{
			{
				Location: testLocation,
				Value: map[string]any{
					"Operation":  "as?",
					"SourceType": "AnyStruct",
					"TargetType": "Int",
					"ActualType": "Int?",
				},
				Code: []byte(code),
			},
		},
		matches,
	)
}

func TestQueryForceCastWithExpressionType(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as! Int
          let a: Int = 1
          let b = a as! Int
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and .Operation == "as!"
          and .Expression.ActualType == "AnyStruct"
        )
      | {Operation, SourceType, TargetType, ActualType}
    `)
	assert.Equal(t,
		[]query.Match{
			{
				Location: testLocation,
				Value: map[string]any{
					"Operation":  "as!",
					"SourceType": "AnyStruct",
					"TargetType": "Int",
					"ActualType": "Int",
				},
				Code: []byte(code),
			},
		},
		matches,
	)
}

func TestQueryNoMatches(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x = 42
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(.Type? == "CastingExpression")
    `)
	assert.Empty(t, matches)
}

func TestQueryForceCastOrFailableCast(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as? Int
          let z = x as! String
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and (.Operation == "as!" or .Operation == "as?")
        )
      | .Operation
    `)
	assert.ElementsMatch(t,
		[]query.Match{
			{
				Location: testLocation,
				Value:    "as?",
				Code:     []byte(code),
			},
			{
				Location: testLocation,
				Value:    "as!",
				Code:     []byte(code),
			},
		},
		matches,
	)
}

func TestQueryTargetType(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as! Int
          let z = x as! String
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and .TargetType == "Int"
        )
      | {Operation, SourceType, TargetType, ActualType}
    `)
	assert.Equal(t,
		[]query.Match{
			{
				Location: testLocation,
				Value: map[string]any{
					"Operation":  "as!",
					"SourceType": "AnyStruct",
					"TargetType": "Int",
					"ActualType": "Int",
				},
				Code: []byte(code),
			},
		},
		matches,
	)
}

func TestQuerySourceType(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x: AnyStruct = 42
          let y = x as! Int
      }
    `
	matches := runQuery(t, code, `
      ..
      | select(
          .Type? == "CastingExpression"
          and .SourceType == "AnyStruct"
        )
      | {Operation, SourceType, TargetType, ActualType}`,
	)
	assert.Equal(t,
		[]query.Match{
			{
				Location: testLocation,
				Value: map[string]any{
					"Operation":  "as!",
					"SourceType": "AnyStruct",
					"TargetType": "Int",
					"ActualType": "Int",
				},
				Code: []byte(code),
			},
		},
		matches,
	)
}

func TestQueryInvalidQuery(t *testing.T) {
	t.Parallel()

	code := `
      access(all) fun main() {
          let x = 42
      }
    `
	program := loadProgram(t, code)
	_, err := query.RunQuery(`invalid jq [[[ query`, program)
	assert.Error(t, err)
}
