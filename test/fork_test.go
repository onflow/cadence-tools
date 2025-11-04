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
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/cadence/common"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	flowmodel "github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

const (
	mainnetForkURL = "access.mainnet.nodes.onflow.org:9000"
	testnetForkURL = "access.testnet.nodes.onflow.org:9000"
)

// All tests in this file connect to live networks and run sequentially
// to avoid overwhelming the remote Access nodes.
//
// Test strategy:
// - One testnet test to verify testnet connectivity works
// - Mainnet tests for specific fork functionality (since mainnet is the production network)

// TestForkTestnet_FlowTokenSupply verifies that testnet fork mode works.
// This is a smoke test to ensure testnet connectivity and basic fork functionality.
func TestForkTestnet_FlowTokenSupply(t *testing.T) {
	var blockHeight uint64

	// Resolve testnet FlowToken address via system contracts
	sc := systemcontracts.SystemContractsForChain(flowmodel.Testnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)

	// Script to query FlowToken.totalSupply
	script := `
        import FlowToken from 0xFLOWTOKEN
        access(all) fun main(): UFix64 { return FlowToken.totalSupply }
    `
	script = strings.ReplaceAll(script, "0xFLOWTOKEN", flowTokenAddr.HexWithPrefix())

	fileResolver := func(path string) (string, error) {
		if path == "flowtoken_supply.cdc" {
			return script, nil
		}
		return "", fmt.Errorf("unknown path: %s", path)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			if name == "FlowToken" {
				return flowTokenAddr, nil
			}
			return common.Address{}, fmt.Errorf("unknown contract: %s", name)
		}).
		WithFork(ForkConfig{ForkHost: testnetForkURL, ChainID: flowmodel.Testnet.Chain().ChainID(), ForkHeight: blockHeight})

	result, err := runner.RunTest(`
        import Test
        access(all) fun test() {
            let res = Test.executeScript(Test.readFile("flowtoken_supply.cdc"), [])
            Test.expect(res, Test.beSucceeded())
            let supply = res.returnValue! as! UFix64
            Test.assert(supply > 0.0)
        }
    `, "test")

	require.NoError(t, err)
	require.NoError(t, result.Error)
}

// TestForkMainnet_WriteAndReadState tests writing and reading account storage in fork mode.
func TestForkMainnet_WriteAndReadState(t *testing.T) {
	var blockHeight uint64

	// Resolve mainnet FlowToken address via system contracts
	sc := systemcontracts.SystemContractsForChain(flowmodel.Mainnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)

	fileResolver := func(path string) (string, error) {
		return "", fmt.Errorf("unknown path: %s", path)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			if name == "FlowToken" {
				return flowTokenAddr, nil
			}
			return common.Address{}, fmt.Errorf("unknown contract: %s", name)
		}).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ChainID: flowmodel.Mainnet.Chain().ChainID(), ForkHeight: blockHeight})

	// Test: Write value to storage, commit, read it back
	result, err := runner.RunTest(`
        import Test

        access(all) fun test() {
            // Get a test account (mainnet address)
            let testAccount = Test.getAccount(0x1654653399040a61)

            // Transaction: Write an Int to storage
            let writeTx = Test.Transaction(
                code: "transaction { prepare(acct: auth(Storage, Capabilities) &Account) { acct.storage.save<Int>(42, to: /storage/testValue); log(\"written\") } }",
                authorizers: [testAccount.address],
                signers: [testAccount],
                arguments: []
            )
            let writeResult = Test.executeTransaction(writeTx)
            Test.expect(writeResult, Test.beSucceeded())

            // Commit the block to persist the change
            Test.commitBlock()

            // Script: Read the Int back (cleaner than using a transaction)
            let readScript = "access(all) fun main(addr: Address): Int? { let acct = getAuthAccount<auth(Storage) &Account>(addr); if let ref = acct.storage.borrow<&Int>(from: /storage/testValue) { return *ref }; return nil }"
            let scriptResult = Test.executeScript(readScript, [testAccount.address])
            Test.expect(scriptResult, Test.beSucceeded())
            Test.assertEqual(42, scriptResult.returnValue! as! Int)

            // Verify the write transaction log
            let allLogs = Test.logs()
            Test.assert(allLogs.length >= 1, message: "Expected at least 1 log entry from write transaction")
        }
    `, "test")

	require.NoError(t, err)
	require.NoError(t, result.Error)
}

// TestForkMainnet_DeployAndCallContract tests deploying a contract in fork mode.
func TestForkMainnet_DeployAndCallContract(t *testing.T) {
	var blockHeight uint64

	// Get system contracts for mainnet
	sc := systemcontracts.SystemContractsForChain(flowmodel.Mainnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)

	// Use an arbitrary mainnet account address
	testAccount, err := common.HexToAddress("0x1654653399040a61")
	require.NoError(t, err)

	// Simple counter contract
	counterContract := `
        access(all) contract Counter {
            access(all) var count: Int
            
            init() {
                self.count = 0
            }
            
            access(all) fun increment() {
                self.count = self.count + 1
            }
            
            access(all) fun getCount(): Int {
                return self.count
            }
        }
    `

	fileResolver := func(path string) (string, error) {
		if path == "Counter.cdc" {
			return counterContract, nil
		}
		return "", fmt.Errorf("unknown path: %s", path)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			switch name {
			case "FlowToken":
				return flowTokenAddr, nil
			case "Counter":
				return testAccount, nil
			default:
				return common.Address{}, fmt.Errorf("unknown contract: %s", name)
			}
		}).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ChainID: flowmodel.Mainnet.Chain().ChainID(), ForkHeight: blockHeight})

	// Test: Deploy contract, call increment, verify count
	result, err := runner.RunTest(`
        import Test

        access(all) fun test() {
            // Deploy the Counter contract to the test account
            let err = Test.deployContract(
                name: "Counter",
                path: "Counter.cdc",
                arguments: []
            )
            Test.expect(err, Test.beNil())

            // Get the test account
            let testAccount = Test.getAccount(0x1654653399040a61)

            // Execute script to check initial count
            let initialCount = Test.executeScript(
                "import Counter from 0x1654653399040a61\naccess(all) fun main(): Int { return Counter.getCount() }",
                []
            )
            Test.expect(initialCount, Test.beSucceeded())
            Test.assertEqual(0, initialCount.returnValue! as! Int)

            // Execute transaction to increment counter
            let incrementTx = Test.Transaction(
                code: "import Counter from 0x1654653399040a61\ntransaction { execute { Counter.increment() } }",
                authorizers: [],
                signers: [testAccount],
                arguments: []
            )
            let txResult = Test.executeTransaction(incrementTx)
            Test.expect(txResult, Test.beSucceeded())

            // Execute script to check count after increment
            let finalCount = Test.executeScript(
                "import Counter from 0x1654653399040a61\naccess(all) fun main(): Int { return Counter.getCount() }",
                []
            )
            Test.expect(finalCount, Test.beSucceeded())
            Test.assertEqual(1, finalCount.returnValue! as! Int)
        }
    `, "test")

	require.NoError(t, err)
	require.NoError(t, result.Error)
}

// TestForkMainnet_ContractUpdate tests that deploying a contract in fork mode
// will update the contract if it already exists on the forked account.
func TestForkMainnet_ContractUpdate(t *testing.T) {
	var blockHeight uint64

	// Get system contracts for mainnet
	sc := systemcontracts.SystemContractsForChain(flowmodel.Mainnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)

	// Use an arbitrary mainnet account address
	testAccount, err := common.HexToAddress("0x1654653399040a61")
	require.NoError(t, err)

	// Initial version of the contract
	counterV1 := `
        access(all) contract Counter {
            access(all) var count: Int
            
            init() {
                self.count = 0
            }
            
            access(all) fun increment() {
                self.count = self.count + 1
            }
            
            access(all) fun getCount(): Int {
                return self.count
            }
        }
    `

	// Updated version with a new function
	counterV2 := `
        access(all) contract Counter {
            access(all) var count: Int
            
            init() {
                self.count = 0
            }
            
            access(all) fun increment() {
                self.count = self.count + 1
            }
            
            access(all) fun incrementBy(_ amount: Int) {
                self.count = self.count + amount
            }
            
            access(all) fun getCount(): Int {
                return self.count
            }
        }
    `

	currentContract := counterV1
	fileResolver := func(path string) (string, error) {
		if path == "Counter.cdc" {
			return currentContract, nil
		}
		return "", fmt.Errorf("unknown path: %s", path)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			switch name {
			case "FlowToken":
				return flowTokenAddr, nil
			case "Counter":
				return testAccount, nil
			default:
				return common.Address{}, fmt.Errorf("unknown contract: %s", name)
			}
		}).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ChainID: flowmodel.Mainnet.Chain().ChainID(), ForkHeight: blockHeight})

	// Test: Deploy V1, use it, then update to V2 and use the new function
	result, err := runner.RunTest(`
        import Test

        access(all) fun test() {
            // Deploy initial version
            let err1 = Test.deployContract(
                name: "Counter",
                path: "Counter.cdc",
                arguments: []
            )
            Test.expect(err1, Test.beNil())

            // Test V1 functionality
            let account = Test.getAccount(0x1654653399040a61)
            let incrementTx = Test.Transaction(
                code: "import Counter from 0x1654653399040a61\ntransaction { execute { Counter.increment() } }",
                authorizers: [],
                signers: [account],
                arguments: []
            )
            let txResult = Test.executeTransaction(incrementTx)
            Test.expect(txResult, Test.beSucceeded())

            // Check count is 1
            let checkScript1 = "import Counter from 0x1654653399040a61\naccess(all) fun main(): Int { return Counter.getCount() }"
            let scriptResult1 = Test.executeScript(checkScript1, [])
            Test.expect(scriptResult1, Test.beSucceeded())
            Test.assertEqual(1, scriptResult1.returnValue! as! Int)

            // Now "update" the contract by deploying again
            let err2 = Test.deployContract(
                name: "Counter",
                path: "Counter.cdc",
                arguments: []
            )
            Test.expect(err2, Test.beNil())

            // After update, the state should be preserved (count should still be 1)
            let checkScript2 = "import Counter from 0x1654653399040a61\naccess(all) fun main(): Int { return Counter.getCount() }"
            let scriptResult2 = Test.executeScript(checkScript2, [])
            Test.expect(scriptResult2, Test.beSucceeded())
            Test.assertEqual(1, scriptResult2.returnValue! as! Int)
        }
    `, "test")

	require.NoError(t, err)
	require.NoError(t, result.Error)

	// Now test with V2 that has the new function
	currentContract = counterV2
	runner2 := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			switch name {
			case "FlowToken":
				return flowTokenAddr, nil
			case "Counter":
				return testAccount, nil
			default:
				return common.Address{}, fmt.Errorf("unknown contract: %s", name)
			}
		}).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ChainID: flowmodel.Mainnet.Chain().ChainID(), ForkHeight: blockHeight})

	result2, err2 := runner2.RunTest(`
        import Test

        access(all) fun test() {
            // Deploy V2 (with new incrementBy function)
            let err = Test.deployContract(
                name: "Counter",
                path: "Counter.cdc",
                arguments: []
            )
            Test.expect(err, Test.beNil())

            // Test the new V2 function
            let account = Test.getAccount(0x1654653399040a61)
            let incrementByTx = Test.Transaction(
                code: "import Counter from 0x1654653399040a61\ntransaction { execute { Counter.incrementBy(5) } }",
                authorizers: [],
                signers: [account],
                arguments: []
            )
            let txResult = Test.executeTransaction(incrementByTx)
            Test.expect(txResult, Test.beSucceeded())

            // Check count increased by 5
            let checkScript = "import Counter from 0x1654653399040a61\naccess(all) fun main(): Int { return Counter.getCount() }"
            let scriptResult = Test.executeScript(checkScript, [])
            Test.expect(scriptResult, Test.beSucceeded())
            Test.assertEqual(5, scriptResult.returnValue! as! Int)
        }
    `, "test")

	require.NoError(t, err2)
	require.NoError(t, result2.Error)
}

// TestForkMainnet_ArbitraryAccount tests that in fork mode, you can interact with
// ANY account on the forked chain, not just pre-created accounts.
func TestForkMainnet_ArbitraryAccount(t *testing.T) {
	var blockHeight uint64

	// Get system contracts for mainnet
	sc := systemcontracts.SystemContractsForChain(flowmodel.Mainnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)

	// Use an arbitrary address - any valid mainnet address works!
	arbitraryAddr, err := common.HexToAddress("0x1654653399040a61")
	require.NoError(t, err)

	// Simple contract for testing
	testContract := `
        access(all) contract TestContract {
            access(all) var value: String
            
            init() {
                self.value = "Hello from arbitrary account!"
            }
            
            access(all) fun getValue(): String {
                return self.value
            }
            
            access(all) fun setValue(_ newValue: String) {
                self.value = newValue
            }
        }
    `

	fileResolver := func(path string) (string, error) {
		if path == "TestContract.cdc" {
			return testContract, nil
		}
		return "", fmt.Errorf("unknown path: %s", path)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			switch name {
			case "FlowToken":
				return flowTokenAddr, nil
			case "TestContract":
				return arbitraryAddr, nil
			default:
				return common.Address{}, fmt.Errorf("unknown contract: %s", name)
			}
		}).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ChainID: flowmodel.Mainnet.Chain().ChainID(), ForkHeight: blockHeight})

	// Test: Get arbitrary account, deploy contract, execute transactions
	result, err := runner.RunTest(`
        import Test

        access(all) fun test() {
            // Get an arbitrary account that's not pre-registered
            // This works in fork mode because GetAccount returns a dummy account
            let arbitraryAccount = Test.getAccount(0x1654653399040a61)
            
            // Verify we got an account back (not nil)
            Test.assert(arbitraryAccount.address.toString() == "0x1654653399040a61", message: "Account address should match")

            // Deploy a contract to this arbitrary account
            let err = Test.deployContract(
                name: "TestContract",
                path: "TestContract.cdc",
                arguments: []
            )
            Test.expect(err, Test.beNil())

            // Execute script to read initial value
            let initialValue = Test.executeScript(
                "import TestContract from 0x1654653399040a61\naccess(all) fun main(): String { return TestContract.getValue() }",
                []
            )
            Test.expect(initialValue, Test.beSucceeded())
            Test.assertEqual("Hello from arbitrary account!", initialValue.returnValue! as! String)

            // Execute transaction to update value
            let updateTx = Test.Transaction(
                code: "import TestContract from 0x1654653399040a61\ntransaction { execute { TestContract.setValue(\"Updated value\") } }",
                authorizers: [],
                signers: [arbitraryAccount],
                arguments: []
            )
            let txResult = Test.executeTransaction(updateTx)
            Test.expect(txResult, Test.beSucceeded())

            // Execute script to verify updated value
            let updatedValue = Test.executeScript(
                "import TestContract from 0x1654653399040a61\naccess(all) fun main(): String { return TestContract.getValue() }",
                []
            )
            Test.expect(updatedValue, Test.beSucceeded())
            Test.assertEqual("Updated value", updatedValue.returnValue! as! String)
        }
    `, "test")

	require.NoError(t, err)
	require.NoError(t, result.Error)
}

// TestFork_ImportResolverAlias verifies that after loadFork in setup(),
// imports are resolved using the correct network string (e.g., "testnet").
func TestFork_ImportResolverAlias(t *testing.T) {
	var observedNetwork string

	importResolver := func(network string, location common.Location) (string, error) {
		observedNetwork = network
		if loc, ok := location.(common.StringLocation); ok && string(loc) == "Helper" {
			return `access(all) contract Helper {}`, nil
		}
		if loc, ok := location.(common.AddressLocation); ok && loc.Name == "Helper" {
			return `access(all) contract Helper {}`, nil
		}
		return "", fmt.Errorf("unknown import: %s", location.ID())
	}

	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			return common.Address{0x5}, nil
		})

	script := `
        import Test
        import "Helper"

        access(all) fun setup() {
            Test.loadFork(network: "testnet", height: nil)
        }

        access(all) fun test() {
            Test.assertEqual(Type<Helper>(), Type<Helper>())
        }
    `

	res, err := runner.RunTest(script, "test")
	require.NoError(t, err)
	require.NoError(t, res.Error)

	// Verify import resolver received "testnet" after loadFork pre-execution
	require.Equal(t, "testnet", observedNetwork)

	backend, ok := runner.testFramework.EmulatorBackend().(*EmulatorBackend)
	require.True(t, ok)
	require.Equal(t, "testnet", backend.NetworkLabel())
}

// TestFork_ContractAddressResolver verifies dynamic address resolution based on network.
func TestFork_ContractAddressResolver(t *testing.T) {
	testnetAddr := common.MustBytesToAddress([]byte{0x9a, 0x07, 0x66, 0xd9, 0x3b, 0x66, 0x08, 0xb7})

	var capturedNetwork string
	addressResolver := func(network string, contractName string) (common.Address, error) {
		capturedNetwork = network
		if contractName == "FungibleToken" {
			if network == "testnet" {
				return testnetAddr, nil
			}
			return common.Address{0xee}, nil
		}
		return common.Address{}, fmt.Errorf("unknown contract: %s", contractName)
	}

	backend := NewEmulatorBackend(zerolog.Nop(), nil, nil, nil)
	backend.contractAddressResolver = addressResolver

	// Test emulator mode
	result := backend.replaceImports(`import "FungibleToken"`)
	require.Equal(t, "emulator", capturedNetwork)
	require.Contains(t, result, "0xee")

	// Test fork mode
	backend.forkEnabled = true
	backend.forkLabel = "testnet"
	result = backend.replaceImports(`import "FungibleToken"`)
	require.Equal(t, "testnet", capturedNetwork)
	require.Contains(t, result, testnetAddr.Hex())
}

// TestFork_LoadForkOutsideSetupErrors verifies loadFork must be in setup().
func TestFork_LoadForkOutsideSetupErrors(t *testing.T) {
	script := `
        import Test
        access(all) fun testInvalid() {
            Test.loadFork(network: "testnet", height: nil)
        }
    `

	_, err := NewTestRunner().RunTest(script, "testInvalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Test.loadFork() must be called in setup() function only")
}

// TestFork_LoadForkVariableArguments verifies that using variables in loadFork arguments produces a clear error.
func TestFork_LoadForkVariableArguments(t *testing.T) {
	script := `
        import Test
        
        access(all) fun setup() {
            let network = "testnet"
            Test.loadFork(network: network, height: nil)
        }
        
        access(all) fun test() {}
    `

	_, err := NewTestRunner().RunTest(script, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "network argument must be a string literal")
	require.Contains(t, err.Error(), "Variables are not supported because loadFork is pre-executed via AST analysis")
}
