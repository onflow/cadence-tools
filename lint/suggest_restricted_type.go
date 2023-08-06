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
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
	"strings"
)

type MemberMeta struct {
	Identifier    string
	Declaration   string
	TypeArguments []string
	Arguments     []string
	ReturnType    *string
}

type MemberName string

type InvocationMeta struct {
	MemberName MemberName
	Position   ast.HasPosition
}

type UsageMeta struct {
	Position   ast.HasPosition
	MemberMeta MemberMeta
}

type MemberInvocations map[MemberName]InvocationMeta
type CompositeMembers map[MemberName]MemberMeta

type CompositeTypeName string
type CompositeType struct {
	CompositeTypeName CompositeTypeName
	Restrictions      []CompositeTypeName
}

type CompositeWithPublicMembers map[CompositeTypeName]CompositeMembers

type CompositeVariable string
type CompositeVariables map[CompositeVariable]CompositeType

type CompositeVarInvocations map[CompositeVariable]InvocationMeta

type MemberUsageMeta map[CompositeVariable]UsageMeta
type CompositeTypeUsageMeta map[CompositeTypeName]MemberUsageMeta

// PublicMemberResources detect all public resources with public members (both vars and functions)
func PublicMemberResources(inspector *ast.Inspector) CompositeWithPublicMembers {
	var publicMembers = make(CompositeWithPublicMembers)

	inspector.Preorder(
		[]ast.Element{(*ast.CompositeDeclaration)(nil)},
		func(element ast.Element) {
			switch declaration := element.(type) {
			case *ast.CompositeDeclaration:
				{
					compositeID := CompositeTypeName(declaration.Identifier.Identifier)
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
							publicMembers[compositeID][MemberName(dec.Identifier.Identifier)] = MemberMeta{
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

							publicMembers[compositeID][MemberName(dec.Identifier.Identifier)] = MemberMeta{
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
	return publicMembers
}

// CompositeTypeVariables find all resource type variables and their respective resource type name
func CompositeTypeVariables(inspector *ast.Inspector, publicMembers CompositeWithPublicMembers) CompositeVariables {
	var compositeVariables = make(CompositeVariables)
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

			_, ok = publicMembers[CompositeTypeName(idex.Identifier.Identifier)]
			if !ok {
				return
			}

			var resTypes []CompositeTypeName

			resType, ok := v.TypeAnnotation.Type.(*ast.RestrictedType)
			if ok {
				for _, typename := range resType.Restrictions {
					resTypes = append(resTypes, CompositeTypeName(typename.Identifier.Identifier))
				}
			}

			variable := CompositeVariable(v.Identifier.Identifier)
			compositeType := CompositeTypeName(idex.Identifier.Identifier)
			compositeVariables[variable] = CompositeType{
				CompositeTypeName: compositeType,
				Restrictions:      resTypes,
			}
		})

	return compositeVariables
}

// CompositeVariableInvocations find all the invocations of provided resource type variables
func CompositeVariableInvocations(inspector *ast.Inspector, resourceVariables CompositeVariables) CompositeVarInvocations {
	var invocations = make(CompositeVarInvocations)
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

			variable := CompositeVariable(idex.Identifier.Identifier)
			if _, ok := resourceVariables[variable]; !ok {
				return
			}

			invocations[variable] = InvocationMeta{
				MemberName: MemberName(exp.Identifier.Identifier),
				Position:   element,
			}
		},
	)
	return invocations
}

func UsedMembersMeta(invocations CompositeVarInvocations, compositeVariables CompositeVariables, resourceTypes CompositeWithPublicMembers) CompositeTypeUsageMeta {
	var usedMembers = make(CompositeTypeUsageMeta)
	for variable, meta := range invocations {
		compositeType := compositeVariables[variable].CompositeTypeName
		members := resourceTypes[compositeType]
		mem := members[meta.MemberName]

		if _, ok := usedMembers[compositeType]; !ok {
			usedMembers[compositeType] = make(MemberUsageMeta)
		}
		usedMembers[compositeType][variable] = UsageMeta{
			Position:   meta.Position,
			MemberMeta: mem,
		}
	}
	return usedMembers
}

var SuggestRestrictedType = (func() *analysis.Analyzer {
	return &analysis.Analyzer{
		Description: "Detects unnecessary uses of the force operator",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			//save all publicly exposed members
			// e.g "Vault" -> "balance", "init", "getBalance", "rob"
			var resourceTypes = PublicMemberResources(inspector)

			//save all the resource type variables of resourceTypes
			// e.g "res"-> ["Vault", ["VaultBalance"]]
			var compositeVariables = CompositeTypeVariables(inspector, resourceTypes)

			//save all usage of resource variables members
			// e.g "res" -> "getBalance"
			var invocations = CompositeVariableInvocations(inspector, compositeVariables)

			//save all composite type variables members invocations
			// e.g "Vault" -> "res" -> "getBalance"
			var usedMembers = UsedMembersMeta(invocations, compositeVariables, resourceTypes)

			for variable, types := range compositeVariables {
				if len(types.Restrictions) == 0 {
					location := pass.Program.Location
					report := pass.Report
					varmeta := usedMembers[types.CompositeTypeName][variable]
					var memberIDs []string
					for _, meta := range usedMembers[types.CompositeTypeName] {
						memberIDs = append(memberIDs, meta.MemberMeta.Identifier)
					}

					msg := "Consider creating restricted type variable with members " + strings.Join(memberIDs, ",")
					report(analysis.Diagnostic{
						Location:         location,
						Range:            ast.NewRangeFromPositioned(nil, varmeta.Position),
						Category:         ReplacementCategory,
						Message:          "unsafe use of resource type for " + string(variable),
						SecondaryMessage: msg,
					})
				}
			}
			//for resource, members := range usedMembers {
			//
			//	location := pass.Program.Location
			//	report := pass.Report
			//
			//	if len(members) < len(resourceTypes[resource]) {
			//		var memberIDs []string
			//
			//		for _, meta := range members {
			//			memberIDs = append(memberIDs, meta.MemberMeta.Identifier)
			//		}
			//		msg := "Consider creating restricted type variable with members " + strings.Join(memberIDs, ",")
			//		for k, meta := range members {
			//			report(
			//				analysis.Diagnostic{
			//					Location:         location,
			//					Range:            ast.NewRangeFromPositioned(nil, meta.Position),
			//					Category:         ReplacementCategory,
			//					Message:          "unsafe use of resource type for " + k,
			//					SecondaryMessage: msg,
			//				})
			//		}
			//	}
			//}

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
