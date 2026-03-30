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

// minAddressHexDigits is the minimum number of hex digits for a literal
// to be considered a potential hardcoded address.
const minAddressHexDigits = 8

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
			inspector := pass.ResultOf[analysis.InspectorAnalyzer].(*ast.Inspector)

			program := pass.Program
			location := program.Location
			report := pass.Report

			inspector.Preorder(
				elementFilter,
				func(element ast.Element) {
					intExpr, ok := element.(*ast.IntegerExpression)
					if !ok {
						return
					}

					// Only consider hex literals (base 16)
					if intExpr.Base != 16 {
						return
					}

					// Check if the hex literal is long enough to be an address.
					// PositiveLiteral contains the literal bytes as written in source.
					if len(intExpr.PositiveLiteral) < minAddressHexDigits {
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
