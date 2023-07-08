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

func TestImplicitCapabilityLeakViaStruct(t *testing.T) {

	t.Parallel()
	t.Run("leak via struct field", func(t *testing.T) {

		diagnostics := testAnalyzers(t,
			`
		pub contract MyContract {
		   pub resource Counter {
		     priv var count: Int
		
		     init(count: Int) {
		       self.count = count
		     }
		   }
		
		   pub struct ContractData {
			   pub var owner: Capability
		       init(cap: Capability) {
		           self.owner = cap
		       }
		   }
		
		   pub var contractData: ContractData
		
		   init(){
		       self.contractData = ContractData(cap:
		           self.account.getCapability<&Counter>(/public/counter)
		       )
		   }
		}`,
			lint.ImplicitCapabilityLeakViaStruct,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 312, Line: 18, Column: 5},
						EndPos:   ast.Position{Offset: 345, Line: 18, Column: 38},
					},
					Location: testLocation,
					Category: lint.ReplacementCategory,
					Message:  "capability might be leaking via public struct field",
				},
			},
			diagnostics,
		)
	})

	t.Run("valid", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			pub contract MyContract {
			   pub resource Counter {
				 priv var count: Int
			
				 init(count: Int) {
				   self.count = count
				 }
			   }
		
			   pub struct ContractData {
				   pub var owner: Capability
				   init(cap: Capability) {
					   self.owner = cap
				   }
			   }
		
               priv var contractData: ContractData
		
			   init(){
				   self.contractData = ContractData(cap:
					   self.account.getCapability<&Counter>(/public/counter)
				   )
			   }
			}
			`,
			lint.ImplicitCapabilityLeakViaStruct,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
