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
	"encoding/json"
	"fmt"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

// enrichment holds type information to add to a JSON node.
type enrichment struct {
	*expressionEnrichment
	*castingExpressionEnrichment
	*forceExpressionEnrichment
}

type expressionEnrichment struct {
	ActualType string
}

type castingExpressionEnrichment struct {
	SourceType string
	TargetType string
	Operation  string
}

type forceExpressionEnrichment struct {
	TargetType string
}

// EnrichProgram converts a type-checked program into JSON which contains the AST enriched with type information.
// Returns a Go value (map/slice/string/etc.) suitable for gojq queries.
func EnrichProgram(program *analysis.Program) (any, error) {

	// Marshal AST to JSON, then unmarshal into generic map tree
	jsonBytes, err := json.Marshal(program.Program)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal AST: %w", err)
	}

	var tree any
	if err := json.Unmarshal(jsonBytes, &tree); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AST JSON: %w", err)
	}

	// Walk AST to collect enrichments, indexed by position
	enrichments := collectEnrichments(program)

	// Walk JSON tree, enrich
	enrichTree(tree, enrichments)

	return tree, nil
}

// posKey creates a position key for indexing enrichment data.
// Uses both start and end positions to disambiguate nodes that share a start position
// (e.g., CastingExpression and its child Expression both start at the same column).
func posKey(startLine, startCol, endLine, endCol int) string {
	return fmt.Sprintf("%d:%d-%d:%d", startLine, startCol, endLine, endCol)
}

// collectEnrichments walks the AST and collects type information for each node.
func collectEnrichments(program *analysis.Program) map[string]enrichment {
	result := map[string]enrichment{}

	elaboration := program.Checker.Elaboration

	inspector := ast.NewInspector(program.Program)

	inspector.Preorder(
		nil,
		func(element ast.Element) {

			startPos := element.StartPosition()
			endPos := element.EndPosition(nil)
			key := posKey(
				startPos.Line,
				startPos.Column,
				endPos.Line,
				endPos.Column,
			)

			result[key] = enrichmentForElement(element, elaboration)
		},
	)

	return result
}

func enrichmentForElement(element ast.Element, elaboration *sema.Elaboration) (result enrichment) {

	// TODO: add support for more node types
	switch element := element.(type) {
	case *ast.CastingExpression:
		result = enrichmentForCastingExpression(element, elaboration)
	case *ast.ForceExpression:
		result = enrichmentForForceExpression(element, elaboration)
	}

	expr, ok := element.(ast.Expression)
	if ok {
		addEnrichmentForExpression(expr, elaboration, &result)
	}

	return
}

func addEnrichmentForExpression(expr ast.Expression, elaboration *sema.Elaboration, result *enrichment) {
	exprTypes := elaboration.ExpressionTypes(expr)
	if exprTypes.ActualType != nil {
		info := &expressionEnrichment{}
		result.expressionEnrichment = info

		info.ActualType = string(exprTypes.ActualType.ID())
	}
}

func enrichmentForCastingExpression(element *ast.CastingExpression, elaboration *sema.Elaboration) (result enrichment) {
	info := &castingExpressionEnrichment{}
	result.castingExpressionEnrichment = info

	info.Operation = element.Operation.Symbol()

	staticTypes := elaboration.StaticCastTypes(element)
	if staticTypes.ExprActualType != nil {
		info.SourceType = string(staticTypes.ExprActualType.ID())
	}
	if staticTypes.TargetType != nil {
		info.TargetType = string(staticTypes.TargetType.ID())
	}

	// Fall back to runtime cast types
	if info.SourceType == "" || info.TargetType == "" {
		runtimeTypes := elaboration.RuntimeCastTypes(element)
		if info.SourceType == "" && runtimeTypes.Left != nil {
			info.SourceType = string(runtimeTypes.Left.ID())
		}
		if info.TargetType == "" && runtimeTypes.Right != nil {
			info.TargetType = string(runtimeTypes.Right.ID())
		}
	}

	return
}

func enrichmentForForceExpression(element *ast.ForceExpression, elaboration *sema.Elaboration) (result enrichment) {
	info := &forceExpressionEnrichment{}
	result.forceExpressionEnrichment = info

	forceType := elaboration.ForceExpressionType(element)
	if forceType != nil {
		info.TargetType = string(forceType.ID())
	}

	return
}

// enrichTree recursively walks a JSON tree and adds enrichment data.
func enrichTree(node any, enrichments map[string]enrichment) {
	switch n := node.(type) {
	case map[string]any:
		enrichMapNode(n, enrichments)
		// Recurse into all values
		for _, v := range n {
			enrichTree(v, enrichments)
		}
	case []any:
		for _, v := range n {
			enrichTree(v, enrichments)
		}
	}
}

// extractLineColumn extracts Line and Column from a JSON position object.
func extractLineColumn(pos map[string]any) (line, col int, ok bool) {
	lineF, ok := pos["Line"].(float64)
	if !ok {
		return 0, 0, false
	}

	colF, ok := pos["Column"].(float64)
	if !ok {
		return 0, 0, false
	}

	return int(lineF), int(colF), true
}

// enrichMapNode adds type info into a single map node if it has a matching position.
func enrichMapNode(node map[string]any, enrichments map[string]enrichment) {
	startPos, ok := node["StartPos"].(map[string]any)
	if !ok {
		return
	}
	startLine, startCol, ok := extractLineColumn(startPos)
	if !ok {
		return
	}

	endPos, ok := node["EndPos"].(map[string]any)
	if !ok {
		return
	}
	endLine, endCol, ok := extractLineColumn(endPos)
	if !ok {
		return
	}

	key := posKey(startLine, startCol, endLine, endCol)
	info, found := enrichments[key]
	if !found {
		return
	}

	// TODO: add support for more node types
	nodeType, _ := node["Type"].(string)
	switch nodeType {
	case "CastingExpression":
		enrichCastingExpression(node, info)

	case "ForceExpression":
		enrichForceExpression(node, info)
	}

	if info.expressionEnrichment != nil {
		node["ActualType"] = info.ActualType
	}
}

func enrichForceExpression(node map[string]any, enrichment enrichment) {
	info := enrichment.forceExpressionEnrichment
	if info == nil {
		return
	}

	node["TargetType"] = info.TargetType
}

func enrichCastingExpression(node map[string]any, enrichment enrichment) {
	info := enrichment.castingExpressionEnrichment
	if info == nil {
		return
	}

	node["Operation"] = info.Operation
	node["SourceType"] = info.SourceType
	node["TargetType"] = info.TargetType
}
