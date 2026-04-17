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
	"sort"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/tools/analysis"
)

var UnusedImportAnalyzer = (func() *analysis.Analyzer {

	return &analysis.Analyzer{
		Description: "Detects unused imports",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			elaboration := program.Checker.Elaboration
			report := pass.Report

			type importInfo struct {
				importDecl *ast.ImportDeclaration
				// For explicit imports (e.g., import A from 0x1), this is the specific Import.
				// For implicit imports (e.g., import 0x1), this is nil.
				explicitImport *ast.Import
			}

			importedNames := map[string]importInfo{}
			usedImports := map[string]struct{}{}

			// Collect all imports using resolved locations

			inspector.Preorder(
				[]ast.Element{
					(*ast.ImportDeclaration)(nil),
				},
				func(element ast.Element) {
					importDecl := element.(*ast.ImportDeclaration)

					// If there are explicit imports,
					// build a map from original name to import for them

					var explicitImportMap map[string]*ast.Import
					if len(importDecl.Imports) > 0 {
						explicitImportMap = make(map[string]*ast.Import)
						for i := range importDecl.Imports {
							imp := &importDecl.Imports[i]
							explicitImportMap[imp.Identifier.Identifier] = imp
						}
					}

					resolvedLocations := elaboration.ImportDeclarationResolvedLocations(importDecl)
					aliases := elaboration.ImportDeclarationAliases(importDecl)

					for _, resolved := range resolvedLocations {
						for _, identifier := range resolved.Identifiers {
							originalName := identifier.Identifier
							name := originalName

							if alias, hasAlias := aliases[originalName]; hasAlias {
								name = alias
							}

							importedNames[name] = importInfo{
								importDecl:     importDecl,
								explicitImport: explicitImportMap[originalName],
							}
						}
					}
				},
			)

			// Track usage in identifier expressions and nominal types

			inspector.Preorder(
				[]ast.Element{
					(*ast.IdentifierExpression)(nil),
					(*ast.NominalType)(nil),
				},
				func(element ast.Element) {
					var name string

					switch element := element.(type) {
					case *ast.IdentifierExpression:
						name = element.Identifier.Identifier

					case *ast.NominalType:
						name = element.Identifier.Identifier

					default:
						return
					}

					if _, isImported := importedNames[name]; isImported {
						usedImports[name] = struct{}{}
					}
				},
			)

			// Collect unused imports.
			// For implicit imports, we only report if ALL imports from that declaration are unused

			type unusedImport struct {
				name string
				info importInfo
			}
			var unused []unusedImport

			// First, for implicit imports, check if any name from each declaration is used
			implicitDeclsWithSomeUsed := map[*ast.ImportDeclaration]struct{}{}
			for name, info := range importedNames {
				_, used := usedImports[name]
				if info.explicitImport == nil && used {
					implicitDeclsWithSomeUsed[info.importDecl] = struct{}{}
				}
			}

			// Now collect unused imports.
			// For implicit imports, we only want to report the declaration once,
			// not once per unused name, so track which declarations we've already added.
			implicitDeclsReported := map[*ast.ImportDeclaration]struct{}{}
			for name, info := range importedNames {
				if _, used := usedImports[name]; !used {
					// For implicit imports, skip if any name from the same declaration is used
					_, ok := implicitDeclsWithSomeUsed[info.importDecl]
					if info.explicitImport == nil && ok {
						continue
					}

					// For implicit imports, deduplicate by declaration
					if info.explicitImport == nil {
						if _, reported := implicitDeclsReported[info.importDecl]; reported {
							continue
						}
						implicitDeclsReported[info.importDecl] = struct{}{}
					}

					unused = append(
						unused,
						unusedImport{
							name: name,
							info: info,
						},
					)
				}
			}

			// Sort by source position for deterministic ordering.
			// If positions are equal (e.g., implicit imports), sort by name

			sort.Slice(unused, func(i, j int) bool {
				var posA, posB ast.Position

				unusedA := unused[i].info
				if unusedA.explicitImport != nil {
					posA = unusedA.explicitImport.Identifier.StartPosition()
				} else {
					posA = unusedA.importDecl.StartPosition()
				}

				unusedB := unused[j].info
				if unusedB.explicitImport != nil {
					posB = unusedB.explicitImport.Identifier.StartPosition()
				} else {
					posB = unusedB.importDecl.StartPosition()
				}

				cmp := posA.Compare(posB)
				if cmp != 0 {
					return cmp < 0
				}
				return unused[i].name < unused[j].name
			})

			// Report in sorted order
			for _, u := range unused {
				var (
					diagRange      ast.Range
					message        string
					suggestedFixes []analysis.SuggestedFix
				)

				importDecl := u.info.importDecl
				explicitImport := u.info.explicitImport

				if explicitImport != nil {
					// For explicit imports, report just the identifier
					diagRange = ast.NewRangeFromPositioned(nil, explicitImport.Identifier)
					message = fmt.Sprintf("unused import '%s'", u.name)

					// Only provide a fix if this is the only import in the declaration.
					// For multiple imports, removing one is complex due to comma handling.
					if len(importDecl.Imports) == 1 {
						// Remove entire declaration including leading whitespace and trailing newline
						declRange := ast.NewRangeFromPositioned(nil, importDecl)

						startPos := declRange.StartPos
						endPos := declRange.EndPos

						suggestedFixes = []analysis.SuggestedFix{
							{
								Message: "Remove unused import",
								TextEdits: []analysis.TextEdit{
									{
										Range: ast.Range{
											StartPos: ast.Position{
												Offset: startPos.Offset - startPos.Column,
												Line:   startPos.Line,
												Column: 0,
											},
											EndPos: ast.Position{
												// +1 to include newline
												Offset: endPos.Offset + 1,
												Line:   endPos.Line + 1,
												Column: 0,
											},
										},
										Replacement: "",
									},
								},
							},
						}
					}
				} else {
					declRange := ast.NewRangeFromPositioned(nil, importDecl)

					// For implicit imports, report the whole declaration
					diagRange = declRange
					message = "unused import"

					// Remove entire declaration including leading whitespace and trailing newline
					startPos := declRange.StartPos
					endPos := declRange.EndPos

					suggestedFixes = []analysis.SuggestedFix{
						{
							Message: "Remove unused import",
							TextEdits: []analysis.TextEdit{
								{
									Range: ast.Range{
										StartPos: ast.Position{
											Offset: startPos.Offset - startPos.Column,
											Line:   startPos.Line,
											Column: 0,
										},
										EndPos: ast.Position{
											// +1 to include newline
											Offset: endPos.Offset + 1,
											Line:   endPos.Line + 1,
											Column: 0,
										},
									},
									Replacement: "",
								},
							},
						},
					}
				}

				report(
					analysis.Diagnostic{
						Location:       location,
						Range:          diagRange,
						Category:       UnusedImportCategory,
						Message:        message,
						SuggestedFixes: suggestedFixes,
					},
				)
			}

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"unused-import",
		UnusedImportAnalyzer,
	)
}
