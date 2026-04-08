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

// PermissiveAccessAnalyzer detects mutable (var) fields declared with access(all),
// which allows any code to publicly read and write the field.
// Immutable (let) fields are not flagged since public read-only access is typically safe.
// Consider restricting mutable fields with entitlements to limit write access.
var PermissiveAccessAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.CompositeDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects access(all) on mutable (var) fields, which allows public write access",
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

					checkPermissiveFields(
						compositeDeclaration,
						compositeType,
						program.Location,
						report,
					)
				},
			)

			return nil
		},
	}
})()

func checkPermissiveFields(
	compositeDeclaration *ast.CompositeDeclaration,
	compositeType *sema.CompositeType,
	location common.Location,
	report func(analysis.Diagnostic),
) {
	for _, fieldDeclaration := range compositeDeclaration.Members.Fields() {
		// Only flag mutable fields (var), not constants (let)
		if fieldDeclaration.VariableKind != ast.VariableKindVariable {
			continue
		}

		// Only flag access(all) fields
		if fieldDeclaration.Access != ast.AccessAll {
			continue
		}

		report(
			analysis.Diagnostic{
				Location: compositeType.Location,
				Range:    fieldDeclaration.Range,
				Category: SecurityCategory,
				Message: fmt.Sprintf(
					"mutable field '%s' of %s '%s' has access(all), "+
						"allowing public write access — consider restricting with entitlements",
					fieldDeclaration.Identifier.Identifier,
					compositeDeclaration.DeclarationKind().Name(),
					compositeType.QualifiedString(),
				),
			},
		)
	}
}

func init() {
	RegisterAnalyzer(
		"permissive-access",
		PermissiveAccessAnalyzer,
	)
}
