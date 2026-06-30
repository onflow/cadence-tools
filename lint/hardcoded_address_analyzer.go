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
	"strings"

	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"
)

// HardcodedAddressAnalyzer detects integer literals that are typed as Address values.
// Hardcoded addresses reduce portability across networks (mainnet, testnet, emulator).
// Consider using named address imports or configuration instead.
var HardcodedAddressAnalyzer = (func() *analysis.Analyzer {

	elementFilter := []ast.Element{
		(*ast.IntegerExpression)(nil),
	}

	return &analysis.Analyzer{
		Description: "Detects hardcoded address literals — consider using named address imports for portability",
		Requires: []*analysis.Analyzer{
			analysis.InspectorAnalyzer,
		},
		Run: func(pass *analysis.Pass) interface{} {
			program := pass.Program
			location := program.Location

			// Skip test files — hardcoded addresses are expected in tests.
			if stringLoc, ok := location.(common.StringLocation); ok {
				if strings.HasSuffix(string(stringLoc), "_test.cdc") {
					return nil
				}
			}

			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			elaboration := program.Checker.Elaboration
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					intExpr, ok := element.(*ast.IntegerExpression)
					if !ok {
						return
					}

					// Use type information from the elaboration to determine
					// whether this integer literal is typed as an Address.
					typ := elaboration.IntegerExpressionType(intExpr)
					if _, ok := typ.(*sema.AddressType); !ok {
						return
					}

					report(
						analysis.Diagnostic{
							Location: location,
							Range:    ast.NewRangeFromPositioned(nil, element),
							Category: SecurityCategory,
							Message:  "hardcoded address detected — consider using named address imports for portability",
						},
					)
				},
			)

			return nil
		},
	}
})()

func init() {
	RegisterAnalyzer(
		"hardcoded-address",
		HardcodedAddressAnalyzer,
	)
}
