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

var AuthorityLeakAnalyzer = (func() *analysis.Analyzer {

	return &analysis.Analyzer{
		Description: "Detects leaks of authority, e.g. public fields exposing (capabilities of) entitled references",
		Run: func(pass *analysis.Pass) interface{} {

			report := pass.Report
			program := pass.Program
			elaboration := program.Checker.Elaboration

			for _, compositeDeclaration := range pass.Program.Program.CompositeDeclarations() {
				compositeType := elaboration.CompositeDeclarationType(compositeDeclaration)
				if compositeType == nil {
					continue
				}

				fieldDeclarations := compositeDeclaration.Members.FieldsByIdentifier()

				compositeType.Members.Foreach(func(memberName string, member *sema.Member) {
					// Only check public fields
					if member.DeclarationKind != common.DeclarationKindField ||
						member.Access != sema.PrimitiveAccess(ast.AccessAll) {

						return
					}

					if isOrContainsType(
						member.TypeAnnotation.Type,
						isEntitledReferenceOrCapability,
					) {
						fieldDeclaration, ok := fieldDeclarations[memberName]
						if !ok {
							return
						}

						report(
							analysis.Diagnostic{
								Location: pass.Program.Location,
								Range:    fieldDeclaration.Range,
								Category: SecurityCategory,
								Message: fmt.Sprintf(
									"field '%s' of %s '%s' exposes (a capability of) an entitled reference, "+
										"which may lead to authority leaks",
									memberName,
									compositeDeclaration.DeclarationKind().Name(),
									compositeType.QualifiedString(),
								),
								SecondaryMessage: "consider restricting access to the field " +
									"or changing its type to avoid authority leaks",
							},
						)
					}

				})
			}

			return nil
		},
	}
})()

func isOrContainsType(ty sema.Type, f func(sema.Type) bool) bool {
	if f(ty) {
		return true
	}

	switch ty := ty.(type) {
	case *sema.VariableSizedType:
		return isOrContainsType(ty.Type, f)

	case *sema.ConstantSizedType:
		return isOrContainsType(ty.Type, f)

	case *sema.DictionaryType:
		return isOrContainsType(ty.KeyType, f) ||
			isOrContainsType(ty.ValueType, f)

	case *sema.OptionalType:
		return isOrContainsType(ty.Type, f)
	}

	return false
}

// isEntitledReferenceOrCapability returns true if the given type is a (capability to an) entitled reference.
func isEntitledReferenceOrCapability(ty sema.Type) bool {
	// Consider only references and capabilities
	var refType *sema.ReferenceType
	switch ty := ty.(type) {
	case *sema.CapabilityType:
		refType, _ = ty.BorrowType.(*sema.ReferenceType)
	case *sema.ReferenceType:
		refType = ty
	}
	if refType == nil {
		return false
	}

	// Check if the referenced type is entitled
	switch refType.Authorization.(type) {
	case *sema.EntitlementMapAccess,
		sema.EntitlementSetAccess:

		return true

	default:
		return false
	}
}

func init() {
	RegisterAnalyzer(
		"authority-leak",
		AuthorityLeakAnalyzer,
	)
}
