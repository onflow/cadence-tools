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
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/tools/analysis"
)

type cadenceV1Analyzer struct {
	program     *analysis.Program
	elaboration *sema.Elaboration
	report      func(analysis.Diagnostic)
	inspector   *ast.Inspector
}

var CadenceV1Analyzer = (func() *analysis.Analyzer {
	return &analysis.Analyzer{
		Description: "Detects uses of removed features in Cadence 1.0",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			analyzer := newCadenceV1Analyzer(pass)
			analyzer.AnalyzeAll()
			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"cadence-v1",
		CadenceV1Analyzer,
	)
}

func newCadenceV1Analyzer(pass *analysis.Pass) *cadenceV1Analyzer {
	return &cadenceV1Analyzer{
		program:     pass.Program,
		elaboration: pass.Program.Checker.Elaboration,
		report:      pass.Report,
		inspector:   pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector),
	}
}

func (v *cadenceV1Analyzer) AnalyzeAll() {
	// Analyze account members removed in Cadence 1.0
	v.analyzeRemovedAccountMembers()
	// Analyze any type identifiers removed in Cadence 1.0
	v.analyzeRemovedTypeIdentifiers()
	// Analyze use of removed `destroy` function for resources
	v.analyzeResourceDestructors()
}

func (v *cadenceV1Analyzer) analyzeRemovedAccountMembers() {
	v.inspector.Preorder(
		[]ast.Element{
			(*ast.MemberExpression)(nil),
		},
		func(element ast.Element) {
			memberExpression, ok := element.(*ast.MemberExpression)
			if !ok {
				return
			}

			memberInfo, _ := v.elaboration.MemberExpressionMemberAccessInfo(memberExpression)
			unwrappedType := unwrapReferenceType(memberInfo.AccessedType)
			if unwrappedType != sema.AccountType {
				return
			}

			identifier := memberExpression.Identifier
			switch identifier.String() {
			// acct.save()
			case "save":
				v.newDiagnostic(
					identifier,
					"`save` has been replaced by the new Storage API.",
					"C1.0-StorageAPI-Save",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
				).WithSimpleReplacement("storage.save").Report()

			// acct.linkAccount()
			case "linkAccount":
				v.newDiagnostic(
					identifier,
					"`linkAccount` has been replaced by the Capability Controller API.",
					"C1.0-StorageAPI-LinkAccount",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).Report()

			// acct.link()
			case "link":
				v.newDiagnostic(
					identifier,
					"`link` has been replaced by the Capability Controller API.",
					"C1.0-StorageAPI-Link",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).Report()

			// acct.unlink()
			case "unlink":
				v.newDiagnostic(
					identifier,
					"`unlink` has been replaced by the Capability Controller API.",
					"C1.0-StorageAPI-Unlink",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).Report()

			// acct.getCapability<&A>()
			case "getCapability":
				v.newDiagnostic(
					identifier,
					"`getCapability` has been replaced by the Capability Controller API.",
					"C1.0-CapabilityAPI-GetCapability",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).WithSimpleReplacement("capabilities.get").Report()

			// acct.getLinkTarget()
			case "getLinkTarget":
				v.newDiagnostic(
					identifier,
					"`getLinkTarget` has been replaced by the Capability Controller API.",
					"C1.0-CapabilityAPI-GetLinkTarget",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).Report()

			// acct.addPublicKey()
			case "addPublicKey":
				v.newDiagnostic(
					identifier,
					"`addPublicKey` has been removed in favour of the new Key Management API. Please use `keys.add` instead.",
					"C1.0-KeyAPI-AddPublicKey",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#capability-controller-api-replaced-existing-linking-based-capability-api-82",
				).Report()

			// acct.removePublicKey()
			case "removePublicKey":
				v.newDiagnostic(
					identifier,
					"`removePublicKey` has been removed in favour of the new Key Management API.\nPlease use `keys.revoke` instead.",
					"C1.0-KeyAPI-RemovePublicKey",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#deprecated-key-management-api-got-removed-60",
				).WithSimpleReplacement("keys.revoke").Report()
			}
		},
	)
}

func (v *cadenceV1Analyzer) analyzeRemovedTypeIdentifiers() {
	v.inspectTypeAnnotations(func(typeAnnotation *ast.TypeAnnotation) {
		switch typeAnnotation.Type.String() {
		case "AuthAccount":
			v.
				newDiagnostic(
					typeAnnotation.Type,
					"`AuthAccount` has been removed in Cadence 1.0.  Please use an authorized `&Account` reference with necessary entitlements instead.",
					"C1.0-AuthAccount",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
				).
				WithSimpleReplacement("&Account").
				Report()
		case "PublicAccount":
			v.
				newDiagnostic(
					typeAnnotation.Type,
					"`PublicAccount` has been removed in Cadence 1.0.  Please use an `&Account` reference instead.",
					"C1.0-PublicAccount",
					"https://forum.flow.com/t/update-on-cadence-1-0/5197#account-access-got-improved-55",
				).
				WithSimpleReplacement("&Account").
				Report()
		}
	})
}

func (v *cadenceV1Analyzer) analyzeResourceDestructors() {
	v.inspector.WithStack(
		[]ast.Element{
			(*ast.SpecialFunctionDeclaration)(nil),
		},
		func(element ast.Element, push bool, stack []ast.Element) bool {
			declaration := element.(*ast.SpecialFunctionDeclaration)
			if declaration.DeclarationIdentifier().Identifier != "destroy" {
				return true
			}

			if len(stack) < 2 {
				return true
			}

			parent, ok := stack[len(stack)-2].(*ast.CompositeDeclaration)
			if !ok {
				return true
			}

			if parent.Kind() != common.CompositeKindResource {
				return true
			}

			v.reportRemovedResourceDestructor(declaration)
			return false
		},
	)
}

func (v *cadenceV1Analyzer) reportRemovedResourceDestructor(
	declaration *ast.SpecialFunctionDeclaration,
) {
	shouldSuggestRemoval := func() bool {
		functionDeclaration := declaration.FunctionDeclaration
		if !functionDeclaration.FunctionBlock.HasStatements() {
			return true
		}

		for _, statement := range functionDeclaration.FunctionBlock.Block.Statements {
			expressionStatement, ok := statement.(*ast.ExpressionStatement)
			if !ok {
				return false
			}

			if _, ok := expressionStatement.Expression.(*ast.DestroyExpression); !ok {
				return false
			}
		}

		return true
	}

	diagnostic := v.newDiagnostic(
		declaration,
		"`destroy` keyword has been removed.  Nested resources will now be implicitly destroyed with their parent.  A `ResourceDestroyed` event can be configured to be emitted to notify clients of the destruction.",
		"C1.0-ResourceDestruction",
		"https://forum.flow.com/t/update-on-cadence-1-0/5197#force-destruction-of-resources-101",
	)

	if shouldSuggestRemoval() {
		diagnostic.WithSimpleReplacement("")
	}
	diagnostic.Report()
}

// Type annotations are not part of traversal, so we need to inspect them separately
func (v *cadenceV1Analyzer) inspectTypeAnnotations(f func(typeAnnotation *ast.TypeAnnotation)) {
	var processAnnotation func(annotation *ast.TypeAnnotation)
	processAnnotation = func(annotation *ast.TypeAnnotation) {
		// Filter out nil type annotations
		if annotation == nil {
			return
		}

		switch t := annotation.Type.(type) {
		case *ast.InstantiationType:
			// We need to process the type arguments of an instantiation type
			for _, typeArgument := range t.TypeArguments {
				processAnnotation(typeArgument)
			}
		}

		f(annotation)
	}

	// Helper function to process a parameter list
	processParameterList := func(parameterList *ast.ParameterList) {
		// Filter out nil parameter lists
		// This can happen for example in a function declaration with no parameters
		// (i.e. transaction declaration with no arguments)
		if parameterList == nil {
			return
		}
		for _, parameter := range parameterList.Parameters {
			processAnnotation(parameter.TypeAnnotation)
		}
	}

	v.inspector.Preorder(
		[]ast.Element{
			(*ast.FieldDeclaration)(nil),
			(*ast.FunctionExpression)(nil),
			(*ast.CastingExpression)(nil),
			(*ast.FunctionDeclaration)(nil),
			(*ast.TransactionDeclaration)(nil),
			(*ast.VariableDeclaration)(nil),
			(*ast.SpecialFunctionDeclaration)(nil),
			(*ast.InvocationExpression)(nil),
		},
		func(element ast.Element) {
			switch declaration := element.(type) {
			case *ast.FieldDeclaration:
				processAnnotation(declaration.TypeAnnotation)
			case *ast.FunctionExpression:
				processAnnotation(declaration.ReturnTypeAnnotation)
				processParameterList(declaration.ParameterList)
			case *ast.CastingExpression:
				processAnnotation(declaration.TypeAnnotation)
			case *ast.FunctionDeclaration:
				processAnnotation(declaration.ReturnTypeAnnotation)
				processParameterList(declaration.ParameterList)
			case *ast.TransactionDeclaration:
				processParameterList(declaration.ParameterList)
			case *ast.VariableDeclaration:
				processAnnotation(declaration.TypeAnnotation)
			case *ast.SpecialFunctionDeclaration:
				processParameterList(declaration.FunctionDeclaration.ParameterList)
			case *ast.InvocationExpression:
				for _, argument := range declaration.TypeArguments {
					processAnnotation(argument)
				}
			}
		},
	)
}

func (v *cadenceV1Analyzer) newDiagnostic(
	position ast.HasPosition,
	message string,
	code string,
	docURL string,
) *diagnostic {
	return newDiagnostic(
		v.program.Location,
		v.report,
		fmt.Sprintf("[Cadence 1.0] %s", message),
		ast.NewRangeFromPositioned(nil, position),
	).WithCode(code).WithURL(docURL).WithCategory(CadenceV1Category)
}

// Helpers
func unwrapReferenceType(t sema.Type) sema.Type {
	if refType, ok := t.(*sema.ReferenceType); ok {
		return refType.Type
	}
	return t
}
