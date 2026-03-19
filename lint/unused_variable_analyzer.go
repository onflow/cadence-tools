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

package lint

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

var UnusedVariableAnalyzer = (func() *analysis.Analyzer {
	elementFilter := []ast.Element{
		(*ast.VariableDeclaration)(nil),
		(*ast.FunctionDeclaration)(nil),
		(*ast.InterfaceDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects variables and parameters in inner functions that are declared but never used",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)
			program := pass.Program
			checker := program.Checker
			location := program.Location
			report := pass.Report

			// Collect identifiers to check,
			// from function parameters and variable declarations

			type identifierKind int

			const (
				identifierKindVariable identifierKind = iota
				identifierKindParameter
			)

			type identifierToCheck struct {
				identifier ast.Identifier
				kind       identifierKind
				parameter  *ast.Parameter
			}

			var identifiers []identifierToCheck

			var inInterfaceDepth int
			var inFunctionDepth int

			inspector.Elements(elementFilter, func(element ast.Element, push bool) bool {
				switch decl := element.(type) {
				case *ast.InterfaceDeclaration:
					if push {
						inInterfaceDepth++
					} else {
						inInterfaceDepth--
					}

				case *ast.FunctionDeclaration:
					if push {
						inFunctionDepth++
					} else {
						inFunctionDepth--
					}

					// Collect all parameter identifiers,
					// but ignore non-default interface functions
					if push && (inInterfaceDepth == 0 ||
						(inInterfaceDepth > 0 && decl.FunctionBlock.HasStatements())) {

						// Collect all parameter identifiers
						for _, param := range decl.ParameterList.Parameters {
							identifiers = append(
								identifiers,
								identifierToCheck{
									identifier: param.Identifier,
									kind:       identifierKindParameter,
									parameter:  param,
								},
							)
						}
					}

				case *ast.VariableDeclaration:
					// Ignore global/member variables with access(all),
					// as they may be used externally.
					if push && (inFunctionDepth > 0 || decl.Access != ast.AccessAll) {
						identifiers = append(
							identifiers,
							identifierToCheck{
								identifier: decl.Identifier,
								kind:       identifierKindVariable,
							},
						)
					}
				}
				return true
			})

			// Analyze each collected identifier for usage

			for _, item := range identifiers {
				identifier := item.identifier
				name := identifier.Identifier

				// Skip underscore-prefixed and blank identifiers
				if name == "_" || strings.HasPrefix(name, "_") {
					continue
				}

				// Get position and find occurrences
				astPosition := identifier.StartPosition()
				position := sema.ASTToSemaPosition(astPosition)
				occurrence := checker.PositionInfo.Occurrences.Find(position)

				if occurrence == nil || occurrence.Origin == nil {
					continue
				}

				// Check if unused (only declaration, no usages)
				if len(occurrence.Origin.Occurrences) == 1 {
					var kindStr string
					var suggestedFix errors.SuggestedFix[ast.TextEdit]

					switch item.kind {
					case identifierKindVariable:
						kindStr = "variable"
						// For variables: just add underscore prefix
						suggestedFix = errors.SuggestedFix[ast.TextEdit]{
							Message: "Prefix with underscore to mark as intentionally unused",
							TextEdits: []ast.TextEdit{
								{
									Replacement: "_" + name,
									Range:       ast.NewRangeFromPositioned(nil, identifier),
								},
							},
						}

					case identifierKindParameter:
						kindStr = "parameter"
						param := item.parameter

						// For parameters: handle argument label vs parameter name
						if param.Label != "" {
							// Has argument label: only rename parameter (foo bar: Int -> foo _bar: Int)
							suggestedFix = errors.SuggestedFix[ast.TextEdit]{
								Message: "Prefix parameter name with underscore",
								TextEdits: []ast.TextEdit{
									{
										Replacement: "_" + name,
										Range:       ast.NewRangeFromPositioned(nil, identifier),
									},
								},
							}
						} else {
							// No argument label: add parameter name (foo: Int -> foo _foo: Int)
							suggestedFix = errors.SuggestedFix[ast.TextEdit]{
								Message: "Add parameter name with underscore prefix",
								TextEdits: []ast.TextEdit{
									{
										Replacement: fmt.Sprintf("%[1]s _%[1]s", name),
										Range:       ast.NewRangeFromPositioned(nil, identifier),
									},
								},
							}
						}
					}

					message := fmt.Sprintf("%s '%s' is declared but never used", kindStr, name)

					report(analysis.Diagnostic{
						Location: location,
						Range:    ast.NewRangeFromPositioned(nil, identifier),
						Category: UnusedVariableCategory,
						Message:  message,
						SuggestedFixes: []errors.SuggestedFix[ast.TextEdit]{
							suggestedFix,
						},
					})
				}
			}

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"unused-variable",
		UnusedVariableAnalyzer,
	)
}
