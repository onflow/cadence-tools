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

package lint_test

import (
	"testing"

	"github.com/onflow/cadence-tools/lint"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"
)

func TestAuthReferenceLeak(t *testing.T) {

	t.Parallel()

	t.Run("unnecessary", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub contract VaultContract {
				pub resource Vault {
					pub var balance : Int
			
					init(amount: Int) {    
						self.balance = amount
					}
			
					pub fun getBalance(): Int{
						return self.balance
					}
			
					pub fun rob() {
						log("this should not be reachable")
					}
			   }
			
			   pub fun createVault(){
				 let res <- create Vault(amount: 10)
                 
                 log(res.getBalance())
				 
                 self.account.save(<-res, to: /storage/vault)
			   }
			}`,
			lint.SuggestRestrictedType,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 417, Line: 22, Column: 21},
						EndPos:   ast.Position{Offset: 432, Line: 22, Column: 36},
					},
					Location:         testLocation,
					Category:         lint.ReplacementCategory,
					Message:          "unsafe use of resource type for res",
					SecondaryMessage: "Consider creating restricted type variable with members getBalance",
				},
			},
			diagnostics,
		)
	})

	t.Run("valid", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub contract VaultContract {
				pub resource interface VaultBalance {
				   pub fun getBalance(): Int
				}
	
				pub resource Vault: VaultBalance {
					access(self) var balance : Int
	
					init(amount: Int) {
						self.balance = amount
					}
	
					pub fun getBalance(): Int{
						return self.balance
					}
	
					pub fun rob() {
						log("this should not be reachable")
					}
			   }
	
			   pub fun createVault(){
				let res : @{VaultBalance} <- create Vault(amount: 10)
				log(res.getBalance())			 
 	            self.account.save(<-res, to: /storage/vault)
	          }
			}
			`,
			lint.SuggestRestrictedType,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
