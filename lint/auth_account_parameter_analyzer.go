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
)

var AuthAccountParameterAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.FunctionDeclaration)(nil),
		(*ast.SpecialFunctionDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects functions with AuthAccount type parameters",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			location := pass.Program.Location
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					var parameterList *ast.ParameterList
					switch declaration := element.(type) {
					case *ast.FunctionDeclaration:
						parameterList = declaration.ParameterList
					case *ast.SpecialFunctionDeclaration:
						if declaration.DeclarationKind() == common.DeclarationKindInitializer {
							parameterList = declaration.FunctionDeclaration.ParameterList
						}
					}

					if parameterList == nil {
						return
					}

					for _, parameter := range parameterList.Parameters {
						nominalType, ok := parameter.TypeAnnotation.Type.(*ast.NominalType)
						if ok && nominalType.Identifier.Identifier == "AuthAccount" {
							report(
								analysis.Diagnostic{
									Location:         location,
									Range:            ast.NewRangeFromPositioned(nil, element),
									Category:         UpdateCategory,
									Message:          "It is an anti-pattern to pass AuthAccount to functions.",
									SecondaryMessage: "Consider using Capabilities instead.",
								},
							)
						}
					}
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"auth-account-parameter",
		AuthAccountParameterAnalyzer,
	)
}
