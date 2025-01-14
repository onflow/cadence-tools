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
	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/tools/analysis"
)

type operationKind int

const (
	operationKindUnknown operationKind = iota
	operationKindAssign
	operationKindUse
)

type operation struct {
	variable string
	kind     operationKind
	el       ast.Element
}

func newOperation(kind operationKind, identifier string, el ast.Element) *operation {
	return &operation{
		variable: identifier,
		kind:     kind,
		el:       el,
	}
}

type block struct {
	children   []*block
	locals     map[string]ast.Element
	useCount   map[string]int
	functions  map[string][]*operation
	operations []*operation
}

func newBlock() *block {
	return &block{
		locals:     make(map[string]ast.Element),
		useCount:   make(map[string]int),
		functions:  make(map[string][]*operation),
		operations: make([]*operation, 0),
		children:   make([]*block, 0),
	}
}

func (b *block) addChild(child *block) {
	b.children = append(b.children, child)
}

func Walk(b *block, el ast.Element) {

}

func parseFunction(pass *analysis.Pass, block *ast.FunctionBlock) []*operation {

	program := pass.Program
	location := program.Location
	report := pass.Report
	elaboration := program.Checker.Elaboration

	externalOperations := make([]*operation, 0)

	functions := make(map[string][]*operation)
	locals := make(map[string]ast.Element)
	useCount := make(map[string]int)

	var walk func(expr ast.Element)
	var use func(expr ast.Element)

	declare := func(variable string, el ast.Element) {
		locals[variable] = el
		useCount[variable] = 0
	}

	assign := func(variable string, e *ast.AssignmentStatement, check bool) {
		useCount[variable] = 0
		v, isLocal := locals[variable]
		if !isLocal {
			id, isIdentifier := e.Target.(*ast.IdentifierExpression)
			//only identifier targets from parent scope needs tracking
			if isIdentifier {
				externalOperations = append(externalOperations, newOperation(operationKindAssign, id.Identifier.Identifier, e))
			}
			return
		}
		if v != nil && check {
			report(
				analysis.Diagnostic{
					Location: location,
					Range:    ast.NewRangeFromPositioned(nil, v),
					Category: RemovalCategory,
					Message:  "ineffectual assign",
				},
			)
		}
		locals[variable] = e
	}

	use = func(expr ast.Element) {
		if expr == nil {
			return
		}
		switch e := expr.(type) {
		case *ast.IdentifierExpression:
			_, isLocal := locals[e.Identifier.Identifier]
			if !isLocal {
				externalOperations = append(externalOperations, newOperation(operationKindUse, e.Identifier.Identifier, e))
				return
			}
			locals[e.Identifier.Identifier] = nil
			useCount[e.Identifier.Identifier]++
			return

		case *ast.IndexExpression:
			use(e.IndexingExpression)
			use(e.TargetExpression)
			return
		}

		expr.Walk(use)

	}

	walk = func(element ast.Element) {
		if element == nil {
			return
		}
		switch e := element.(type) {

		case *ast.VariableDeclaration:
			identifier := e.Identifier.Identifier

			// resource tracking handles those cases no need to track the variable
			// just track uses on values ( function parameters etc )
			if e.Transfer.Operation == ast.TransferOperationMove {
				use(e.Value)
				use(e.SecondValue)
				return
			}

			variableType := elaboration.VariableDeclarationTypes(e)

			fb, isFunction := e.Value.(*ast.FunctionExpression)
			if isFunction {
				// delay function expression until it is called
				functions[identifier] = parseFunction(pass, fb.FunctionBlock)
				declare(identifier, e)
				return
			}

			// mark locals used in value expression as used
			use(e.Value)

			// track identifier if it is not reference
			if !variableType.TargetType.IsOrContainsReferenceType() {
				declare(identifier, e)
			}

		case *ast.AssignmentStatement:

			// no need to track resources, just track locals used on RHS
			if e.Transfer.Operation == ast.TransferOperationMove {
				use(e.Value)
				return
			}

			switch target := e.Target.(type) {
			case *ast.IdentifierExpression:
				//assignedType := elaboration.AssignmentStatementTypes(e)
				//TODO: check function type

				function, isFunction := e.Value.(*ast.FunctionExpression)
				if isFunction {
					// parse function and store operations to use on invocation
					functions[target.Identifier.Identifier] = parseFunction(pass, function.FunctionBlock)
					assign(target.Identifier.Identifier, e, true)
					return
				}
				use(e.Value)
				assign(target.Identifier.Identifier, e, true)

			case *ast.IndexExpression:
				use(e.Value)
				assign(target.TargetExpression.String(), e, false)
			}
			return

		case *ast.InvocationExpression:
			target, ok := e.InvokedExpression.(*ast.IdentifierExpression)
			if !ok {
				use(e)
				return
			}

			use(target)

			f, ok := functions[target.Identifier.Identifier]
			if !ok {
				use(e)
				return
			}

			//simulate operations
			for _, op := range f {
				switch op.kind {
				case operationKindAssign:
					assign(op.variable, op.el.(*ast.AssignmentStatement), true)
				case operationKindUse:
					locals[op.variable] = nil
				default:
					panic("unhandled default case")
				}
			}

		case *ast.ReturnStatement:
			use(e.Expression)

		case *ast.IfStatement:
			use(e.Test)
			use(e.Then)
			if e.Else != nil {
				use(e.Else)
			}

		case *ast.WhileStatement:
			//TODO: handle while block later
			use(e.Test)
			use(e.Block)
			return

		case *ast.ForStatement:
			//TODO: handle for block later
			use(e.Value)
			use(e.Block)
			return

		case *ast.SwitchStatement:
			for _, swCase := range e.Cases {
				use(swCase.Expression)
				for _, s := range swCase.Statements {
					use(s)
				}
			}
			return
		}
		if element != nil {
			element.Walk(walk)
		}
	}

	if block != nil {
		walk(block)
	}

	for k, v := range locals {
		if v != nil && useCount[k] == 0 {
			report(
				analysis.Diagnostic{
					Location: location,
					Range:    ast.NewRangeFromPositioned(nil, v),
					Category: RemovalCategory,
					Message:  "unused assign",
				},
			)
		}
	}

	return externalOperations
}

var IneffAssignAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.FunctionDeclaration)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects ineffectual assignments",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					block := element.(*ast.FunctionDeclaration)
					parseFunction(pass, block.FunctionBlock)
				},
			)

			return nil
		},
	}

})()

func init() {
	RegisterAnalyzer(
		"ineff-assign",
		IneffAssignAnalyzer,
	)
}
