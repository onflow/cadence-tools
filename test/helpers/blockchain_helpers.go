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

package helpers

import (
	_ "embed"
	"fmt"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/parser"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
)

//go:embed blockchain_helpers.cdc
var BlockchainHelpers []byte

const BlockchainHelpersLocation = common.IdentifierLocation("BlockchainHelpers")

func BlockchainHelpersChecker() *sema.Checker {
	program, err := parser.ParseProgram(
		nil,
		BlockchainHelpers,
		parser.Config{},
	)
	if err != nil {
		panic(err)
	}

	importHandler := func(
		checker *sema.Checker,
		importedLocation common.Location,
		importRange ast.Range,
	) (sema.Import, error) {
		var elaboration *sema.Elaboration
		switch importedLocation {
		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			elaboration = testChecker.Elaboration
		default:
			return nil, fmt.Errorf("import not supported")
		}

		return sema.ElaborationImport{
			Elaboration: elaboration,
		}, nil
	}

	activation := sema.NewVariableActivation(sema.BaseValueActivation)
	activation.DeclareValue(stdlib.InterpreterAssertFunction)
	activation.DeclareValue(stdlib.InterpreterPanicFunction)

	checker, err := sema.NewChecker(
		program,
		BlockchainHelpersLocation,
		nil,
		&sema.Config{
			BaseValueActivationHandler: func(_ common.Location) *sema.VariableActivation {
				return activation
			},
			AccessCheckMode: sema.AccessCheckModeStrict,
			ImportHandler:   importHandler,
		},
	)
	if err != nil {
		panic(err)
	}

	err = checker.Check()
	if err != nil {
		panic(err)
	}

	return checker
}
