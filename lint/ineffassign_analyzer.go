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
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
	"slices"
)

type operationKind int

const (
	operationKindAssign = iota
	operationKindPartialAssign
	operationKindUse
)

type warning struct {
	el      ast.Element
	message string
}

type analyzeResult struct {
	warnings []warning
}

func (r *analyzeResult) add(el ast.Element, message string) {
	w := warning{
		el:      el,
		message: message,
	}
	if !slices.Contains(r.warnings, w) {
		r.warnings = append(r.warnings, w)
	}
}

func (r *analyzeResult) merge(r2 *analyzeResult) {
	for _, w := range r2.warnings {
		r.add(w.el, w.message)
	}
}

type variable struct {
	name          string
	declaration   ast.Element
	functionBlock *block
}

type operations []*operation

func (ops operations) uniqueVariables() []*variable {
	variables := make([]*variable, 0)
	for _, op := range ops {
		if !slices.Contains(variables, op.variable) {
			variables = append(variables, op.variable)
		}
	}
	return variables
}

func (ops operations) filterByVariable(v *variable) operations {
	res := make(operations, 0, len(ops))
	for _, op := range ops {
		if op.variable.declaration != v.declaration {
			continue
		}
		res = append(res, op)
	}
	return res
}

type operation struct {
	variable *variable
	kind     operationKind
	el       ast.Element
}

type block struct {
	parent     *block
	children   []*block
	locals     map[string]*variable
	operations operations
}

func newOperation(kind operationKind, variable *variable, el ast.Element) *operation {
	return &operation{
		variable: variable,
		kind:     kind,
		el:       el,
	}
}

func newBlock(parents ...*block) *block {
	bl := &block{
		locals:     make(map[string]*variable),
		operations: make(operations, 0),
		children:   make([]*block, 0),
	}

	for _, parent := range parents {
		parent.addChild(bl)
	}
	return bl
}

func (b *block) addChild(child *block) {
	b.children = append(b.children, child)
	child.parent = b
}

func (b *block) variable(name string) *variable {

	if v, ok := b.locals[name]; ok {
		return v
	}
	if b.parent == nil {
		return nil
	}
	return b.parent.variable(name)
}

func (b *block) declare(name string, el ast.Element) *variable {
	b.locals[name] = &variable{
		name:          name,
		declaration:   el,
		functionBlock: newBlock(),
	}
	b.assign(name, el)
	return b.locals[name]
}

func (b *block) assign(name string, el ast.Element) {
	v := b.variable(name)
	if v != nil {
		b.operations = append(b.operations, newOperation(operationKindAssign, v, el))
	}
}

func (b *block) partialAssign(name string, el ast.Element) {
	v := b.variable(name)
	if v != nil {
		b.operations = append(b.operations, newOperation(operationKindPartialAssign, v, el))
	}
}

func (b *block) useElement(el ast.Element) {
	if el == nil {
		return
	}
	switch e := el.(type) {
	case *ast.IdentifierExpression:
		v := b.variable(e.Identifier.Identifier)
		if v != nil {
			b.operations = append(b.operations, newOperation(operationKindUse, v, nil))
		}
	}
	el.Walk(b.useElement)
}

func Walk(pass *analysis.Pass, bl *block, el ast.Element) {

	program := pass.Program
	elaboration := program.Checker.Elaboration
	var walk func(el ast.Element)
	b := bl

	walk = func(element ast.Element) {
		if element == nil {
			return
		}

		switch e := element.(type) {

		case *ast.IdentifierExpression:
			b.useElement(e)

		case *ast.FunctionDeclaration:
			v := b.declare(e.Identifier.Identifier, e)
			v.functionBlock = newBlock(b)
			Walk(pass, v.functionBlock, e.FunctionBlock)
			v.functionBlock.parent = nil
			return

		case *ast.VariableDeclaration:
			identifier := e.Identifier.Identifier

			// resource tracking handles those cases no need to track the variable
			// just track uses on values ( function parameters etc )
			if e.Transfer.Operation == ast.TransferOperationMove {
				walk(e.Value)
				if e.SecondValue != nil {
					walk(e.SecondValue)
				}
				return
			}

			variableType := elaboration.VariableDeclarationTypes(e)
			if variableType.TargetType.Tag().Equals(sema.FunctionTypeTag) {
				v := b.declare(identifier, e)
				v.functionBlock = newBlock(b)
				walk(e.Value)
				v.functionBlock.parent = nil
				return
			}

			// mark locals used in value expression as used
			walk(e.Value)

			// track identifier if it is not a reference
			if !variableType.TargetType.IsOrContainsReferenceType() {
				b.declare(identifier, e)
			}
			return

		case *ast.AssignmentStatement:

			// no need to track resources, just track locals used on RHS
			if e.Transfer.Operation == ast.TransferOperationMove {
				walk(e.Value)
				return
			}

			switch target := e.Target.(type) {
			case *ast.IdentifierExpression:
				identifier := target.Identifier.Identifier
				variableType := elaboration.AssignmentStatementTypes(e)
				if variableType.TargetType.Tag().Equals(sema.FunctionTypeTag) {
					v := b.variable(identifier)
					v.functionBlock = newBlock(b)
					walk(e.Value)
					v.functionBlock.parent = nil
					return
				}
				walk(e.Value)
				b.assign(target.Identifier.Identifier, e)

			case *ast.IndexExpression:
				walk(e.Value)
				b.partialAssign(target.TargetExpression.String(), e)

			default:
				walk(e.Value)
			}
			return

		case *ast.InvocationExpression:
			walk(e.InvokedExpression)
			for _, arg := range e.Arguments {
				walk(arg.Expression)
			}

			target, ok := e.InvokedExpression.(*ast.IdentifierExpression)
			if !ok {
				return
			}

			identifier := target.Identifier.Identifier
			v := b.variable(identifier)
			if v != nil {
				for _, op := range v.functionBlock.operations {
					b.operations = append(b.operations, op)
				}
			}

		case *ast.ReturnStatement:
			walk(e.Expression)

		case *ast.IfStatement:
			walk(e.Test)

			thenBlock := newBlock(b)
			elseBlock := newBlock(b)
			contBlock := newBlock(thenBlock, elseBlock)

			Walk(pass, thenBlock, e.Then)
			if e.Else != nil {
				Walk(pass, elseBlock, e.Else)
			}
			b = contBlock

		case *ast.WhileStatement:
			b.useElement(e.Block)
			b.useElement(e.Test)

		case *ast.ForStatement:
			b.useElement(e.Block)
			b.useElement(e.Value)

		case *ast.SwitchStatement:
			walk(e.Expression)
			for _, swCase := range e.Cases {
				walk(swCase.Expression)
				for _, s := range swCase.Statements {
					b.useElement(s)
				}
			}

		default:
			e.Walk(walk)
		}

	}

	if el != nil {
		walk(el)
	}
}

func (b *block) checkConsecutiveAssignmentsAndUnused(ops operations) *analyzeResult {
	result := &analyzeResult{}

	var lastWrite ast.Element = nil

	for _, v := range ops.uniqueVariables() {
		vOps := ops.filterByVariable(v)

		for i := 0; i < len(vOps); i++ {
			currentOp := vOps[i]
			if v.declaration != currentOp.variable.declaration {
				continue
			}

			switch currentOp.kind {
			case operationKindAssign, operationKindPartialAssign:
				lastWrite = currentOp.el
				if i < len(vOps)-1 {
					nextOp := vOps[i+1]
					if nextOp.kind == operationKindAssign {
						if len(b.children) == 0 {
							result.add(currentOp.el, "ineffectual assign")
						}
					}
				}

			case operationKindUse:
				lastWrite = nil

			}
		}

		if lastWrite != nil {
			if len(b.children) == 0 {
				result.add(lastWrite, "unused assign")
			}
		}
	}

	return result
}

func (b *block) getBestChildWarnings(ops operations) *analyzeResult {
	if len(b.children) == 0 {
		return &analyzeResult{warnings: []warning{}}
	}

	bestResult := b.children[0].check(ops)
	bestCount := len(bestResult.warnings)

	for _, child := range b.children[1:] {
		childResult := child.check(ops)
		childWarnings := len(childResult.warnings)

		if childWarnings < bestCount {
			bestResult = childResult
			bestCount = childWarnings
		} else if childWarnings == bestCount {
			bestResult.merge(childResult)
		}
	}

	return bestResult
}

func (b *block) check(baseOps operations) *analyzeResult {
	ops := append(baseOps, b.operations...)
	result := b.checkConsecutiveAssignmentsAndUnused(ops)
	childResult := b.getBestChildWarnings(ops)
	result.merge(childResult)
	return result
}

var IneffAssignAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.Block)(nil),
		(*ast.SwitchStatement)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects ineffectual & unused assignments",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			reportBlock := func(b *block) {
				seen := operations{}
				for _, warning := range b.check(seen).warnings {
					pass.Report(
						analysis.Diagnostic{
							Location: pass.Program.Location,
							Range:    ast.NewRangeFromPositioned(nil, warning.el),
							Category: RemovalCategory,
							Message:  warning.message,
						},
					)
				}
			}
			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					switch el := element.(type) {
					case *ast.Block:
						b := newBlock()
						Walk(pass, b, el)
						reportBlock(b)
					case *ast.SwitchStatement:
						for _, swCase := range el.Cases {
							b := newBlock()
							for _, s := range swCase.Statements {
								Walk(pass, b, s)
							}
							reportBlock(b)
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
		"ineff-assign",
		IneffAssignAnalyzer,
	)
}
