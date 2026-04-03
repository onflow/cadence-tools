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

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

// PublicAccountParamAnalyzer detects public (access(all)) functions that accept
// authorized Account references (auth(...) &Account) as parameters.
// Such functions expose broad account access to any caller, which is a significant
// attack surface. Consider using capabilities to provide scoped access instead.
var PublicAccountParamAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.CompositeDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects public functions that accept authorized Account references as parameters",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			report := pass.Report
			program := pass.Program
			elaboration := program.Checker.Elaboration

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					compositeDeclaration, ok := element.(*ast.CompositeDeclaration)
					if !ok {
						return
					}

					compositeType := elaboration.CompositeDeclarationType(compositeDeclaration)
					if compositeType == nil {
						return
					}

					for _, functionDeclaration := range compositeDeclaration.Members.Functions() {
						functionType := elaboration.FunctionDeclarationFunctionType(functionDeclaration)
						checkPublicFunctionAccountParams(
							functionDeclaration,
							functionType,
							compositeDeclaration.DeclarationKind(),
							compositeType.QualifiedString(),
							compositeType.Location,
							report,
						)
					}
				},
			)

			return nil
		},
	}
})()

func checkPublicFunctionAccountParams(
	functionDeclaration *ast.FunctionDeclaration,
	functionType *sema.FunctionType,
	parentKind common.DeclarationKind,
	parentName string,
	location common.Location,
	report func(analysis.Diagnostic),
) {
	// Only check public functions
	if functionDeclaration.Access != ast.AccessAll {
		return
	}

	parameterList := functionDeclaration.ParameterList
	if parameterList == nil || functionType == nil {
		return
	}

	funcName := functionDeclaration.Identifier.Identifier

	for i, parameter := range functionType.Parameters {
		if !isAuthAccountReferenceType(parameter.TypeAnnotation.Type) {
			continue
		}

		report(
			analysis.Diagnostic{
				Location: location,
				Range:    ast.NewRangeFromPositioned(nil, parameterList.Parameters[i]),
				Category: SecurityCategory,
				Message: fmt.Sprintf(
					"public function '%s' of %s '%s' accepts an authorized Account reference "+
						"— this grants callers broad account access, consider using capabilities instead",
					funcName,
					parentKind.Name(),
					parentName,
				),
			},
		)
	}
}

// isAuthAccountReferenceType checks if a type is an authorized reference
// to the Account type (auth(...) &Account).
func isAuthAccountReferenceType(ty sema.Type) bool {
	switch t := ty.(type) {
	case *sema.ReferenceType:
		if t.Type == sema.AccountType {
			switch t.Authorization.(type) {
			case *sema.EntitlementMapAccess,
				sema.EntitlementSetAccess:
				return true
			}
		}
	case *sema.OptionalType:
		return isAuthAccountReferenceType(t.Type)
	}
	return false
}

func init() {
	RegisterAnalyzer(
		"public-account-param",
		PublicAccountParamAnalyzer,
	)
}
