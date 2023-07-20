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

package lint

import (
	"strings"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

type MemberMeta struct {
	Identifier    string
	Declaration   string
	TypeArguments []string
	Arguments     []string
	ReturnType    *string
}

type InvocationMeta struct {
	MemberName string
	Position   ast.HasPosition
}

type UsageMeta struct {
	Position   ast.HasPosition
	MemberMeta MemberMeta
}

type CompositeMembers map[string]MemberMeta

type MemberInvocations map[string]InvocationMeta

var SuggestRestrictedType = (func() *analysis.Analyzer {

	//elementFilter := []ast.Element{
	//	(*ast.InvocationExpression)(nil),
	//}

	return &analysis.Analyzer{
		Description: "Detects unnecessary uses of the force operator",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			//store all publicly exposed members
			var publicMembers = make(map[string]CompositeMembers)

			inspector.Preorder(
				[]ast.Element{(*ast.CompositeDeclaration)(nil)},
				func(element ast.Element) {
					switch declaration := element.(type) {
					case *ast.CompositeDeclaration:
						{
							compositeID := declaration.Identifier.Identifier
							if declaration.DeclarationKind() != common.DeclarationKindResource {
								return
							}
							for _, d := range declaration.Members.Declarations() {
								if d.DeclarationAccess() != ast.AccessPublic {
									continue
								}

								switch dec := d.(type) {
								case *ast.FieldDeclaration:
									_, ok := publicMembers[compositeID]
									if !ok {
										publicMembers[compositeID] = make(CompositeMembers)
									}
									publicMembers[compositeID][dec.Identifier.Identifier] = MemberMeta{
										Identifier:  dec.Identifier.Identifier,
										Declaration: dec.TypeAnnotation.Type.String(),
									}
								case *ast.FunctionDeclaration:
									var typeParams []string
									if dec.TypeParameterList != nil && len(dec.TypeParameterList.TypeParameters) > 0 {
										for _, p := range dec.TypeParameterList.TypeParameters {
											typeParams = append(typeParams, p.Identifier.Identifier)
										}
									}

									var args []string
									for _, arg := range dec.ParameterList.Parameters {
										args = append(args, arg.Label+" "+arg.Identifier.Identifier+" "+arg.TypeAnnotation.Type.String())
									}

									var returnType string
									if dec.ReturnTypeAnnotation != nil {
										returnType = dec.ReturnTypeAnnotation.Type.String()
									}

									_, ok := publicMembers[compositeID]
									if !ok {
										publicMembers[compositeID] = make(CompositeMembers)
									}

									publicMembers[compositeID][dec.Identifier.Identifier] = MemberMeta{
										Identifier:    dec.Identifier.Identifier,
										Declaration:   "function",
										TypeArguments: typeParams,
										Arguments:     args,
										ReturnType:    &returnType,
									}
								}
							}
						}
					}
				},
			)

			var compositeVariables = make(map[string]string)
			inspector.Preorder(
				[]ast.Element{
					(*ast.VariableDeclaration)(nil),
				},
				func(element ast.Element) {
					v, ok := element.(*ast.VariableDeclaration)
					if !ok {
						return
					}
					ce, ok := v.Value.(*ast.CreateExpression)
					if !ok {
						return
					}

					idex, ok := ce.InvocationExpression.InvokedExpression.(*ast.IdentifierExpression)
					if !ok {
						return
					}

					_, ok = publicMembers[idex.Identifier.Identifier]
					if !ok {
						return
					}
					compositeVariables[v.Identifier.Identifier] = idex.Identifier.Identifier
				})

			invocations := make(map[string]InvocationMeta)
			inspector.Preorder([]ast.Element{
				(*ast.InvocationExpression)(nil),
			},
				func(element ast.Element) {
					inv, ok := element.(*ast.InvocationExpression)
					if !ok {
						return
					}
					exp, ok := inv.InvokedExpression.(*ast.MemberExpression)
					if !ok {
						return
					}

					idex, ok := exp.Expression.(*ast.IdentifierExpression)
					if !ok {
						return
					}

					if _, ok := compositeVariables[idex.Identifier.Identifier]; !ok {
						return
					}

					invocations[idex.Identifier.Identifier] = InvocationMeta{
						MemberName: exp.Identifier.Identifier,
						Position:   element,
					}
				},
			)

			var usedMembers = make(map[string]map[string]UsageMeta)
			for variable, meta := range invocations {
				compositeType := compositeVariables[variable]
				members := publicMembers[compositeType]
				mem := members[meta.MemberName]

				if _, ok := usedMembers[compositeType]; !ok {
					usedMembers[compositeType] = make(map[string]UsageMeta)
				}
				usedMembers[compositeType][variable] = UsageMeta{
					Position:   meta.Position,
					MemberMeta: mem,
				}
			}

			for resource, members := range usedMembers {

				location := pass.Program.Location
				report := pass.Report

				if len(members) < len(publicMembers[resource]) {
					var memberIDs []string

					for _, meta := range members {
						memberIDs = append(memberIDs, meta.MemberMeta.Identifier)
					}
					msg := "Consider creating restricted type variable with members " + strings.Join(memberIDs, ",")
					for k, meta := range members {
						report(
							analysis.Diagnostic{
								Location:         location,
								Range:            ast.NewRangeFromPositioned(nil, meta.Position),
								Category:         ReplacementCategory,
								Message:          "unsafe use of resource type for " + k,
								SecondaryMessage: msg,
							})
					}
				}
			}

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"suggest-restricted-type",
		SuggestRestrictedType,
	)
}
