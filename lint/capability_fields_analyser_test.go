/*
 * Cadence-lint - The Cadence linter
 *
 * Copyright 2019-2023 Dapper Labs, Inc.
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

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence-tools/lint"
)

func TestCapabilityFieldInContract(t *testing.T) {

	t.Parallel()

	t.Run("public capability field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
				pub contract ExposingCapability {
					pub let my_capability : Capability?
					init() {
						self.my_capability = nil
					}
				}
			`,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 44, Line: 3, Column: 5},
						EndPos:   ast.Position{Offset: 78, Line: 3, Column: 39},
					},
					Location:         testLocation,
					Category:         lint.UpdateCategory,
					Message:          "It is an anti-pattern to have public Capability fields.",
					SecondaryMessage: "Consider restricting access.",
				},
			},
			diagnostics,
		)
	})

	t.Run("private capability field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
				pub contract ExposingCapability {
					priv let my_capability : Capability?
					pub let data: Int
					init() {
						self.my_capability = nil
						self.data = 42
					}
				}
			`,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}

func TestPublicDictionaryFieldWithCapabilityValueType(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    pub let my_capability : {String: Capability?}
			    init() {
				    self.my_capability = {"key": nil}
			    }
		    }
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 48, Line: 3, Column: 7},
					EndPos:   ast.Position{Offset: 92, Line: 3, Column: 51},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to have public Capability fields.",
				SecondaryMessage: "Consider restricting access.",
			},
		},
		diagnostics,
	)
}

func TestPublicArrayFieldWithConcreteCapabilityType(t *testing.T) {

	t.Parallel()

	diagnostics := testAnalyzers(t,
		`
		    pub contract ExposingCapability {
			    pub resource interface MySecretStuffInterface {}
			    pub let array_capability : [Capability<&{MySecretStuffInterface}>]
			    init() {
				    self.array_capability = [self.account.link<&{MySecretStuffInterface}>(/public/Stuff, target: /storage/Stuff)!]
			    }
		}
		`,
		lint.CapabilityFieldAnalyzer,
	)

	require.Equal(
		t,
		[]analysis.Diagnostic{
			{
				Range: ast.Range{
					StartPos: ast.Position{Offset: 104, Line: 4, Column: 7},
					EndPos:   ast.Position{Offset: 169, Line: 4, Column: 72},
				},
				Location:         testLocation,
				Category:         lint.UpdateCategory,
				Message:          "It is an anti-pattern to have public Capability fields.",
				SecondaryMessage: "Consider restricting access.",
			},
		},
		diagnostics,
	)
}

func TestImplicitCapabilityLeakViaArray(t *testing.T) {

	t.Parallel()

	t.Run("public leaking capability in an array", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  pub contract MyContract {
				  pub let myCapArray: [Capability]
				  init() {
						  self.myCapArray = []
				  }
			  }
			  `,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 38, Line: 3, Column: 6},
						EndPos:   ast.Position{Offset: 69, Line: 3, Column: 37},
					},
					Location:         testLocation,
					Category:         lint.UpdateCategory,
					Message:          "It is an anti-pattern to have public Capability fields.",
					SecondaryMessage: "Consider restricting access.",
				},
			},
			diagnostics,
		)
	})

	t.Run("private nonleaking capability in an array", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
			  pub contract MyContract {
				  priv let myCapArray: [Capability]
	  
				  init() {
						 self.myCapArray = []
				  }
			  }
			  `,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}

func TestImplicitCapabilityLeakViaStruct(t *testing.T) {
	t.Parallel()

	t.Run("leak via struct field", func(t *testing.T) {

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

			pub struct ContractDataIgnored {
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
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 484, Line: 25, Column: 6},
						EndPos:   ast.Position{Offset: 517, Line: 25, Column: 39},
					},
					Location:         testLocation,
					Category:         lint.UpdateCategory,
					Message:          "It is an anti-pattern to have public Capability fields.",
					SecondaryMessage: "Consider restricting access.",
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
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}

func TestImplicitCapabilityLeakViaResource(t *testing.T) {
	t.Parallel()

	t.Run("leak via resource field", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
		pub resource MyResource {
			pub let my_capability : Capability?
			init() {
				self.my_capability = nil
			}
		}
		pub contract MyContract {

			// Declare a public field using the MyResource resource
			pub var myResource: @MyResource?

			// Initialize the contract
			init() {
				self.myResource <- nil
			}
		}`,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic{
				{
					Range: ast.Range{
						StartPos: ast.Position{Offset: 209, Line: 11, Column: 3},
						EndPos:   ast.Position{Offset: 240, Line: 11, Column: 34},
					},
					Location:         testLocation,
					Category:         lint.UpdateCategory,
					Message:          "It is an anti-pattern to have public Capability fields.",
					SecondaryMessage: "Consider restricting access.",
				},
			},
			diagnostics,
		)
	})
	t.Run("valid", func(t *testing.T) {

		t.Parallel()

		diagnostics := testAnalyzers(t,
			`
		pub resource MyResource {
			pub let my_capability : Capability?
			init() {
				self.my_capability = nil
			}
		}
		pub contract MyContract {

			priv var myResource: @MyResource?

			init() {
				self.myResource <- nil
			}
		}`,
			lint.CapabilityFieldAnalyzer,
		)

		require.Equal(
			t,
			[]analysis.Diagnostic(nil),
			diagnostics,
		)
	})
}
