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
	"github.com/stretchr/testify/require"
)

const (
	mainnetForkURL = "access.mainnet.nodes.onflow.org:9000"
	testnetForkURL = "access.testnet.nodes.onflow.org:9000"
)

// defaultNetworkResolver provides network-to-host mappings for fork tests.
func defaultNetworkResolver(network string) (string, bool) {
	switch strings.ToLower(network) {
	case "mainnet":
		return mainnetForkURL, true
	case "testnet":
		return testnetForkURL, true
	default:
		// If it looks like a host:port, return it as-is
		if strings.Contains(network, ":") {
			return network, true
		}
		return "", false
	}
}

// All tests in this file connect to live networks and run sequentially
// to avoid overwhelming the remote Access nodes.
//
// Test strategy:
// - One testnet test to verify testnet connectivity works
// - Mainnet tests for specific fork functionality (since mainnet is the production network)

// TestForkTestnet_FlowTokenSupply verifies that testnet fork mode works.
// This is a smoke test to ensure testnet connectivity and basic fork functionality.
func TestForkTestnet_FlowTokenSupply(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestFork_ImportResolverAlias verifies that Test.executeScript returns types from the emulator
// that match the test framework's types when using custom import resolution.
func TestFork_ImportResolverAlias(t *testing.T) {
	helperContract := `access(all) contract Helper {}`
	helperAddr := common.MustBytesToAddress([]byte{0x16, 0x54, 0x65, 0x33, 0x99, 0x04, 0x0a, 0x61})

	fileResolver := func(path string) (string, error) {
		if path == "Helper.cdc" {
			return helperContract, nil
		}
		return "", fmt.Errorf("unknown path: %s", path)
	}

	importResolver := func(network string, location common.Location) (string, error) {
		if loc, ok := location.(common.AddressLocation); ok {
			if loc.Name == "Helper" && common.Address(loc.Address) == helperAddr {
				return helperContract, nil
			}
		}
		return "", fmt.Errorf("unknown import: %s", location.ID())
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithNetworkResolver(defaultNetworkResolver).
		WithImportResolver(importResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			if name == "Helper" {
				return helperAddr, nil
			}
			return common.Address{}, fmt.Errorf("unknown contract: %s", name)
		})

	script := `
        #test_fork(network: "mainnet", height: nil)
        import Test
        import "Helper"
        access(all) fun test() {
            let err = Test.deployContract(name: "Helper", path: "Helper.cdc", arguments: [])
            Test.expect(err, Test.beNil())
            Test.commitBlock()
            
            let script = "import \"Helper\"\naccess(all) fun main(): Type { return Type<Helper>() }"
            let result = Test.executeScript(script, [])
            Test.expect(result, Test.beSucceeded())
            
            let emulatorType = result.returnValue! as! Type
            Test.assertEqual(Type<Helper>(), emulatorType)
        }
    `

	res, err := runner.RunTest(script, "test")
	require.NoError(t, err)
	require.NoError(t, res.Error)
}

// TestFork_PragmaOverridesWithFork verifies pragma overrides WithFork configuration.
func TestFork_PragmaOverridesWithFork(t *testing.T) {
	script := `
        #test_fork(network: "testnet", height: nil)
        import Test
        access(all) fun test() {
            // Just verify the test runs without error
            Test.assertEqual(1, 1)
        }
    `

	// WithFork sets mainnet, but pragma should override to testnet
	res, err := NewTestRunner().
		WithNetworkResolver(defaultNetworkResolver).
		WithFork(ForkConfig{ForkHost: mainnetForkURL, ForkHeight: 0}).
		RunTest(script, "test")
	require.NoError(t, err)
	require.NoError(t, res.Error)
}

// TestFork_PragmasRejectNonLiteralArgs verifies that using non-literal args produces a clear error.
func TestFork_PragmasRejectNonLiteralArgs(t *testing.T) {
	script := `
        #test_fork(network: network, height: nil)
        access(all) fun test() {}
    `

	_, err := NewTestRunner().WithNetworkResolver(defaultNetworkResolver).RunTest(script, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "test_fork pragma 'network' must be a string literal")
}

// TestFork_Events verifies that Test.eventsOfType() works with inline Test.loadFork()
// without hanging or scanning remote blocks.
func TestFork_Events(t *testing.T) {
	sc := systemcontracts.SystemContractsForChain(flowmodel.Mainnet.Chain().ChainID())
	flowTokenAddr := common.Address(sc.FlowToken.Address)
	fungibleTokenAddr := common.Address(sc.FungibleToken.Address)

	importResolver := func(network string, location common.Location) (string, error) {
		// No custom imports needed - rely on fork mode's built-in resolution
		return "", fmt.Errorf("cannot resolve import: %s", location.ID())
	}

	runner := NewTestRunner().
		WithNetworkResolver(defaultNetworkResolver).
		WithImportResolver(importResolver).
		WithContractAddressResolver(func(network string, name string) (common.Address, error) {
			if name == "FlowToken" {
				return flowTokenAddr, nil
			}
			if name == "FungibleToken" {
				return fungibleTokenAddr, nil
			}
			return common.Address{}, fmt.Errorf("unknown contract: %s", name)
		})

	script := `
        #test_fork(network: "mainnet", height: nil)
        import Test
        import "FlowToken"

        access(all) fun test() {
            // Get the initial event count before our transaction
            let eventsBefore = Test.eventsOfType(Type<FlowToken.TokensWithdrawn>())
            let countBefore = eventsBefore.length
            
            // Get a funded account that has a FlowToken vault
            let acct = Test.getAccount(0x42a06f24a1049154)
            
            // Execute a transaction that withdraws tokens (definitely emits TokensWithdrawn)
            let tx = Test.Transaction(
                code: "import FlowToken from 0x1654653399040a61\nimport FungibleToken from 0xf233dcee88fe0abe\ntransaction { prepare(acct: auth(BorrowValue) &Account) { let vault = acct.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)!; let withdrawn <- vault.withdraw(amount: 0.00000001); destroy withdrawn } }",
                authorizers: [acct.address],
                signers: [acct],
                arguments: []
            )
            let res = Test.executeTransaction(tx)
            Test.expect(res, Test.beSucceeded())
            
            // Fetch events after - should NOT hang and should only return local events
            let eventsAfter = Test.eventsOfType(Type<FlowToken.TokensWithdrawn>())
            let countAfter = eventsAfter.length
            
            // Verify that we got at least one new event from our transaction
            log("Events before:")
            log(countBefore)
            log("Events after:")
            log(countAfter)
            
            // Assert that our event shows up
            Test.assert(countAfter > countBefore, message: "Expected new TokensWithdrawn event from withdraw")
        }
    `

	result, err := runner.RunTest(script, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}
