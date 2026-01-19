/*
 * Cadence test - The Cadence test framework
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

package test

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow-go/fvm/environment"
)

// transactionIndexFunctionType is the type of the `getTransactionIndex` function.
// This defines the signature as `func(): UInt32`
var transactionIndexFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UInt32Type),
}

func transactionIndexDeclaration(env environment.Environment) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:      "getTransactionIndex",
		DocString: `Returns the transaction index in the current block, i.e. first transaction in a block has index of 0, second has index of 1...`,
		Type:      transactionIndexFunctionType,
		Kind:      common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			transactionIndexFunctionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				return interpreter.NewUInt32Value(
					invocation.InvocationContext,
					func() uint32 {
						return env.TxIndex()
					})
			},
		),
	}
}
