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

package query

import (
	"fmt"

	"github.com/itchyny/gojq"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/tools/analysis"
)

// Match represents a single query result.
type Match struct {
	Location common.Location
	Value    any
	Code     []byte
}

// OutputFormat controls how query results are printed.
type OutputFormat int

const (
	OutputText OutputFormat = iota
	OutputJSON
)

// compileQuery parses and compiles a jq query string.
func compileQuery(queryStr string) (*gojq.Code, error) {
	parsed, err := gojq.Parse(queryStr)
	if err != nil {
		return nil, fmt.Errorf("invalid jq query: %w", err)
	}

	code, err := gojq.Compile(parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq query: %w", err)
	}

	return code, nil
}

// runCompiledQuery runs a compiled jq query against an enriched AST tree
// and calls onMatch for each result.
func runCompiledQuery(code *gojq.Code, tree any, program *analysis.Program, onMatch func(Match)) {
	iter := code.Run(tree)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if _, ok := v.(error); ok {
			continue
		}

		onMatch(Match{
			Location: program.Location,
			Value:    v,
			Code:     program.Code,
		})
	}
}

// RunQuery parses a jq query string, runs it against the enriched AST of a program,
// and returns all matches.
func RunQuery(query string, program *analysis.Program) ([]Match, error) {
	compiledQuery, err := compileQuery(query)
	if err != nil {
		return nil, err
	}

	tree, err := EnrichProgram(program)
	if err != nil {
		return nil, err
	}

	var matches []Match
	runCompiledQuery(compiledQuery, tree, program, func(m Match) {
		matches = append(matches, m)
	})

	return matches, nil
}

// NewQueryAnalyzer creates an analysis.Analyzer that runs a jq query
// and collects matches via the callback.
func NewQueryAnalyzer(query string, onMatch func(Match)) (*analysis.Analyzer, error) {
	compiledQuery, err := compileQuery(query)
	if err != nil {
		return nil, err
	}

	return &analysis.Analyzer{
		Description: "AST query",
		Run: func(pass *analysis.Pass) any {

			tree, err := EnrichProgram(pass.Program)
			if err != nil {
				return nil
			}

			runCompiledQuery(compiledQuery, tree, pass.Program, onMatch)

			return nil
		},
	}, nil
}
