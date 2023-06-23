/*
 * Cadence - The resource-oriented smart contract programming language
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

package test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/cadence/runtime/tests/checker"
)

func TestRunningMultipleTests(t *testing.T) {
	t.Parallel()

	const code = `
        pub fun testFunc1() {
            assert(false)
        }

        pub fun testFunc2() {
            assert(true)
        }
    `

	runner := NewTestRunner()
	results, err := runner.RunTests(code)
	require.NoError(t, err)

	require.Len(t, results, 2)

	result1 := results[0]
	assert.Equal(t, result1.TestName, "testFunc1")
	assert.Error(t, result1.Error)

	result2 := results[1]
	assert.Equal(t, result2.TestName, "testFunc2")
	require.NoError(t, result2.Error)
}

func TestRunningSingleTest(t *testing.T) {
	t.Parallel()

	const code = `
        pub fun testFunc1() {
            assert(false)
        }

        pub fun testFunc2() {
            assert(true)
        }
    `

	runner := NewTestRunner()

	result, err := runner.RunTest(code, "testFunc1")
	require.NoError(t, err)
	assert.Error(t, result.Error)

	result, err = runner.RunTest(code, "testFunc2")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestAssertFunction(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        pub fun testAssertWithNoArgs() {
            Test.assert(true)
        }

        pub fun testAssertWithNoArgsFail() {
            Test.assert(false)
        }

        pub fun testAssertWithMessage() {
            Test.assert(true, message: "some reason")
        }

        pub fun testAssertWithMessageFail() {
            Test.assert(false, message: "some reason")
        }
    `

	runner := NewTestRunner()

	result, err := runner.RunTest(code, "testAssertWithNoArgs")
	require.NoError(t, err)
	require.NoError(t, result.Error)

	result, err = runner.RunTest(code, "testAssertWithNoArgsFail")
	require.NoError(t, err)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "assertion failed")

	result, err = runner.RunTest(code, "testAssertWithMessage")
	require.NoError(t, err)
	require.NoError(t, result.Error)

	result, err = runner.RunTest(code, "testAssertWithMessageFail")
	require.NoError(t, err)
	require.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "assertion failed: some reason")
}

func TestExecuteScript(t *testing.T) {
	t.Parallel()

	t.Run("no args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var result = blockchain.executeScript("pub fun main(): Int {  return 2 + 3 }", [])

                assert(result.status == Test.ResultStatus.succeeded)
                assert((result.returnValue! as! Int) == 5)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("with args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var result = blockchain.executeScript(
                    "pub fun main(a: Int, b: Int): Int {  return a + b }",
                    [2, 3]
                )

                assert(result.status == Test.ResultStatus.succeeded)
                assert((result.returnValue! as! Int) == 5)
        }
    `
		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("non-empty array returns", func(t *testing.T) {
		t.Parallel()

		const code = ` pub fun main(): [UInt64] { return [1, 2, 3]} `

		testScript := fmt.Sprintf(
			`
                import Test

                pub fun test() {
                    var blockchain = Test.newEmulatorBlockchain()
                    var result = blockchain.executeScript("%s", [])

                    assert(result.status == Test.ResultStatus.succeeded)

                    let resultArray = result.returnValue! as! [UInt64]
                    assert(resultArray.length == 3)
                    assert(resultArray[0] == 1)
                    assert(resultArray[1] == 2)
                    assert(resultArray[2] == 3)
            }`,
			code,
		)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("empty array returns", func(t *testing.T) {
		t.Parallel()

		const code = `pub fun main(): [UInt64] { return [] }`

		testScript := fmt.Sprintf(`
		    import Test

		    pub fun test() {
		        let blockchain = Test.newEmulatorBlockchain()
		        let result = blockchain.executeScript("%s", [])

		        Test.assert(result.status == Test.ResultStatus.succeeded)

		        let resultArray = result.returnValue! as! [UInt64]
		        Test.assert(resultArray.length == 0)
		    }`,
			code,
		)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("non-empty dictionary returns", func(t *testing.T) {
		t.Parallel()

		const code = ` pub fun main(): {String: Int} { return {\"foo\": 5, \"bar\": 10}} `

		testScript := fmt.Sprintf(
			`
                import Test

                pub fun test() {
                    var blockchain = Test.newEmulatorBlockchain()
                    var result = blockchain.executeScript("%s", [])

                    assert(result.status == Test.ResultStatus.succeeded)

                    let dictionary = result.returnValue! as! {String: Int}
                    assert(dictionary["foo"] == 5)
                    assert(dictionary["bar"] == 10)
            }`,
			code,
		)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("empty dictionary returns", func(t *testing.T) {
		t.Parallel()

		const code = `pub fun main(): {String: Int} { return {} }`

		testScript := fmt.Sprintf(`
		    import Test

		    pub fun test() {
		        let blockchain = Test.newEmulatorBlockchain()
		        let result = blockchain.executeScript("%s", [])

		        Test.assert(result.status == Test.ResultStatus.succeeded)

		        let dictionary = result.returnValue! as! {String: Int}
		        Test.assert(dictionary["foo"] == nil)
		    }`,
			code,
		)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestImportContract(t *testing.T) {
	t.Parallel()

	t.Run("init no params", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
                var result = foo.sayHello()
                assert(result == "hello from Foo")
            }
        `

		const fooContract = `
            pub contract FooContract {
                init() {}

                pub fun sayHello(): String {
                    return "hello from Foo"
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("init with params", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract(greeting: "hello from Foo")
                var result = foo.sayHello()
                assert(result == "hello from Foo")
            }
        `

		const fooContract = `
            pub contract FooContract {

                pub var greeting: String

                init(greeting: String) {
                    self.greeting = greeting
                }

                pub fun sayHello(): String {
                    return self.greeting
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("invalid import", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return "", errors.New("cannot load file")
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		assert.ErrorAs(t, errs[0], &importedProgramError)
		assert.Contains(t, importedProgramError.Err.Error(), "cannot load file")

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("import resolver not provided", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
            }
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		require.ErrorAs(t, errs[0], &importedProgramError)
		assert.IsType(t, ImportResolverNotProvidedError{}, importedProgramError.Err)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("nested imports", func(t *testing.T) {
		t.Parallel()

		testLocation := common.AddressLocation{
			Address: common.MustBytesToAddress([]byte{0x1}),
			Name:    "BarContract",
		}

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {}
        `

		const fooContract = `
           import BarContract from 0x01

            pub contract FooContract {
                init() {}
            }
        `
		const barContract = `
            pub contract BarContract {
                init() {}
            }
        `

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
			case common.AddressLocation:
				if location == testLocation {
					return barContract, nil
				}
			}

			return "", fmt.Errorf("unsupported import %s", location)
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		_, err := runner.RunTest(code, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nested imports are not supported")
	})
}

func TestImportBuiltinContracts(t *testing.T) {
	t.Parallel()

	testCode := `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun setup() {
	        blockchain.useConfiguration(Test.Configuration({
	            "FooContract": account.address
	        }))
	    }

	    pub fun testSetupFUSDVault() {
	        let code = Test.readFile("../transactions/setup_fusd_vault.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [account.address],
	            signers: [account],
	            arguments: []
	        )

	        let result = blockchain.executeTransaction(tx)
	        assert(result.status == Test.ResultStatus.succeeded)
	    }

	    pub fun testGetIntegerTrait() {
	        let script = Test.readFile("../scripts/import_common_contracts.cdc")
	        let result = blockchain.executeScript(script, [])

	        if result.status != Test.ResultStatus.succeeded {
	            panic(result.error!.message)
	        }
	        assert(result.returnValue! as! Bool)
	    }
	`

	transactionCode := `
	    import "FungibleToken"
	    import "FUSD"

	    transaction {
	        prepare(signer: AuthAccount) {
	            // It's OK if the account already has a Vault, but we don't want to replace it
	            if(signer.borrow<&FUSD.Vault>(from: /storage/fusdVault) != nil) {
	                return
	            }

	            // Create a new FUSD Vault and put it in storage
	            signer.save(<-FUSD.createEmptyVault(), to: /storage/fusdVault)

	            // Create a public capability to the Vault that only exposes
	            // the deposit function through the Receiver interface
	            signer.link<&FUSD.Vault{FungibleToken.Receiver}>(
	                /public/fusdReceiver,
	                target: /storage/fusdVault
	            )

	            // Create a public capability to the Vault that only exposes
	            // the balance field through the Balance interface
	            signer.link<&FUSD.Vault{FungibleToken.Balance}>(
	                /public/fusdBalance,
	                target: /storage/fusdVault
	            )
	        }
	    }
	`

	scriptCode := `
	    import FungibleToken from "FungibleToken"
	    import FlowToken from "FlowToken"
	    import FUSD from "FUSD"
	    import NonFungibleToken from "NonFungibleToken"
	    import MetadataViews from "MetadataViews"
	    import ExampleNFT from "ExampleNFT"
	    import NFTStorefrontV2 from "NFTStorefrontV2"
	    import NFTStorefront from "NFTStorefront"

	    pub fun main(): Bool {
	        return true
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../transactions/setup_fusd_vault.cdc":
			return transactionCode, nil
		case "../scripts/import_common_contracts.cdc":
			return scriptCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)

	result, err := runner.RunTest(testCode, "testSetupFUSDVault")
	require.NoError(t, err)
	require.NoError(t, result.Error)

	result, err = runner.RunTest(testCode, "testGetIntegerTrait")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestUsingEnv(t *testing.T) {
	t.Parallel()

	t.Run("public key creation", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub fun test() {
                var publicKey = PublicKey(
                    publicKey: "1234".decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm.ECDSA_secp256k1
                )
            }
        `

		runner := NewTestRunner()

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)
		publicKeyError := interpreter.InvalidPublicKeyError{}
		assert.ErrorAs(t, result.Error, &publicKeyError)
	})

	t.Run("public account", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub fun test() {
                var acc = getAccount(0x01)
                var bal = acc.balance
                assert(acc.balance == 0.0)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	// Imported programs also should have the access to the env.
	t.Run("account access in imported program", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            pub fun test() {
                var foo = FooContract()
                var result = foo.getBalance()
                assert(result == 0.0)
            }
        `

		const fooContract = `
            pub contract FooContract {
                init() {}

                pub fun getBalance(): UFix64 {
                    var acc = getAccount(0x01)
                    return acc.balance
                }
            }
        `

		importResolver := func(location common.Location) (string, error) {
			return fooContract, nil
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("verify using public key of account", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                // just checking the invocation of verify function
                assert(!account.publicKey.verify(
                    signature: [],
                    signedData: [],
                    domainSeparationTag: "something",
                    hashAlgorithm: HashAlgorithm.SHA2_256
                ))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("verify using public key returned from blockchain", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                var script = Test.readFile("./sample/script.cdc")

                var result = blockchain.executeScript(script, [])

                assert(result.status == Test.ResultStatus.succeeded)

                let pubKey = result.returnValue! as! PublicKey

                // just checking the invocation of verify function
                assert(!pubKey.verify(
                   signature: [],
                   signedData: [],
                   domainSeparationTag: "",
                   hashAlgorithm: HashAlgorithm.SHA2_256
                ))
            }
        `

		const scriptCode = `
            pub fun main(): PublicKey {
                return PublicKey(
                    publicKey: "db04940e18ec414664ccfd31d5d2d4ece3985acb8cb17a2025b2f1673427267968e52e2bbf3599059649d4b2cce98fdb8a3048e68abf5abe3e710129e90696ca".decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
                )
            }
        `

		resolverInvoked := false
		fileResolver := func(path string) (string, error) {
			resolverInvoked = true
			assert.Equal(t, path, "./sample/script.cdc")

			return scriptCode, nil
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)

		assert.True(t, resolverInvoked)
	})
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        pub fun test() {
            var blockchain = Test.newEmulatorBlockchain()
            var account = blockchain.createAccount()
        }
    `

	runner := NewTestRunner()
	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestExecutingTransactions(t *testing.T) {
	t.Parallel()

	t.Run("add transaction", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run next transaction", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run next transaction with authorizer", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { prepare(acct: AuthAccount) {} execute{ assert(true) } }",
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("transaction failure", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx)

                let result = blockchain.executeNextTransaction()!
                assert(result.status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run non existing transaction", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let result = blockchain.executeNextTransaction()
                assert(result == nil)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("commit block", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("commit un-executed block", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx)

                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "cannot be committed before execution")
	})

	t.Run("commit partially executed block", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                // Add two transactions
                blockchain.addTransaction(tx)
                blockchain.addTransaction(tx)

                // But execute only one
                blockchain.executeNextTransaction()

                // Then try to commit
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "is currently being executed")
	})

	t.Run("multiple commit block", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                blockchain.commitBlock()
                blockchain.commitBlock()
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run given transaction", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = blockchain.executeTransaction(tx)
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run transaction with args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction(a: Int, b: Int) { execute{ assert(a == b) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [4, 4],
                )

                let result = blockchain.executeTransaction(tx)
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run transaction with multiple authorizers", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account1 = blockchain.createAccount()
                var account2 = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction() { prepare(acct1: AuthAccount, acct2: AuthAccount) {}  }",
                    authorizers: [account1.address, account2.address],
                    signers: [account1, account2],
                    arguments: [],
                )

                let result = blockchain.executeTransaction(tx)
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run given transaction unsuccessful", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(fail) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = blockchain.executeTransaction(tx)
                assert(result.status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run multiple transactions", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx1 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let tx2 = Test.Transaction(
                    code: "transaction { prepare(acct: AuthAccount) {} execute{ assert(true) } }",
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [],
                )

                let tx3 = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let firstResults = blockchain.executeTransactions([tx1, tx2, tx3])

                assert(firstResults.length == 3)
                assert(firstResults[0].status == Test.ResultStatus.succeeded)
                assert(firstResults[1].status == Test.ResultStatus.succeeded)
                assert(firstResults[2].status == Test.ResultStatus.failed)


                // Execute them again: To verify the proper increment/reset of sequence numbers.
                let secondResults = blockchain.executeTransactions([tx1, tx2, tx3])

                assert(secondResults.length == 3)
                assert(secondResults[0].status == Test.ResultStatus.succeeded)
                assert(secondResults[1].status == Test.ResultStatus.succeeded)
                assert(secondResults[2].status == Test.ResultStatus.failed)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run empty transactions", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let result = blockchain.executeTransactions([])
                assert(result.length == 0)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run transaction with pending transactions", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx1 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                blockchain.addTransaction(tx1)

                let tx2 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )
                let result = blockchain.executeTransaction(tx2)

                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "is currently being executed")
	})

	t.Run("transaction with array typed args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                let tx = Test.Transaction(
                    code: "transaction(a: [Int]) { execute{ assert(a[0] == a[1]) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [[4, 4]],
                )

                let result = blockchain.executeTransaction(tx)
                assert(result.status == Test.ResultStatus.succeeded)
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestSetupAndTearDown(t *testing.T) {
	t.Parallel()

	t.Run("setup", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub(set) var setupRan = false

            pub fun setup() {
                assert(!setupRan)
                setupRan = true
            }

            pub fun testFunc() {
                assert(setupRan)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 1)
		result := results[0]
		assert.Equal(t, result.TestName, "testFunc")
		require.NoError(t, result.Error)
	})

	t.Run("setup failed", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub fun setup() {
                panic("error occurred")
            }

            pub fun testFunc() {
                assert(true)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.Error(t, err)
		require.Empty(t, results)
	})

	t.Run("teardown", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub(set) var tearDownRan = false

            pub fun testFunc() {
                assert(!tearDownRan)
            }

            pub fun tearDown() {
                assert(true)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 1)
		result := results[0]
		assert.Equal(t, result.TestName, "testFunc")
		require.NoError(t, result.Error)
	})

	t.Run("teardown failed", func(t *testing.T) {
		t.Parallel()

		const code = `
            pub(set) var tearDownRan = false

            pub fun testFunc() {
                assert(!tearDownRan)
            }

            pub fun tearDown() {
                assert(false)
            }
        `

		runner := NewTestRunner()
		results, err := runner.RunTests(code)

		// Running tests will return an error since the tear down failed.
		require.Error(t, err)

		// However, test cases should have been passed.
		require.Len(t, results, 1)
		result := results[0]
		assert.Equal(t, result.TestName, "testFunc")
		require.NoError(t, result.Error)
	})
}

func TestBeforeAndAfterEach(t *testing.T) {
	t.Parallel()

	t.Run("beforeEach", func(t *testing.T) {
		t.Parallel()

		code := `
		    pub(set) var counter = 0

		    pub fun beforeEach() {
		        counter = counter + 1
		    }

		    pub fun testFuncOne() {
		        assert(counter == 1)
		    }

		    pub fun testFuncTwo() {
		        assert(counter == 2)
		    }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 2)
		assert.Equal(t, results[0].TestName, "testFuncOne")
		require.NoError(t, results[0].Error)
		assert.Equal(t, results[1].TestName, "testFuncTwo")
		require.NoError(t, results[1].Error)
	})

	t.Run("beforeEach failed", func(t *testing.T) {
		t.Parallel()

		code := `
		    pub fun beforeEach() {
		        panic("error occurred")
		    }

		    pub fun testFunc() {
		        assert(true)
		    }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.Error(t, err)
		require.Empty(t, results)
	})

	t.Run("afterEach", func(t *testing.T) {
		t.Parallel()

		code := `
		    pub(set) var counter = 2

		    pub fun afterEach() {
		        counter = counter - 1
		    }

		    pub fun testFuncOne() {
		        assert(counter == 2)
		    }

		    pub fun testFuncTwo() {
		        assert(counter == 1)
		    }

		    pub fun tearDown() {
		        assert(counter == 0)
		    }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 2)
		assert.Equal(t, results[0].TestName, "testFuncOne")
		require.NoError(t, results[0].Error)
		assert.Equal(t, results[1].TestName, "testFuncTwo")
		require.NoError(t, results[1].Error)
	})

	t.Run("afterEach failed", func(t *testing.T) {
		t.Parallel()

		code := `
		    pub(set) var tearDownRan = false

		    pub fun testFunc() {
		        assert(!tearDownRan)
		    }

		    pub fun afterEach() {
		        assert(false)
		    }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)

		require.Error(t, err)
		require.Len(t, results, 0)
	})
}

func TestPrettyPrintTestResults(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        pub fun testFunc1() {
            Test.assert(true, message: "should pass")
        }

        pub fun testFunc2() {
            Test.assert(false, message: "unexpected error occurred")
        }

        pub fun testFunc3() {
            Test.assert(true, message: "should pass")
        }

        pub fun testFunc4() {
            panic("runtime error")
        }
    `

	runner := NewTestRunner()
	results, err := runner.RunTests(code)
	require.NoError(t, err)

	resultsStr := PrettyPrintResults(results, "test_script.cdc")

	const expected = `Test results: "test_script.cdc"
- PASS: testFunc1
- FAIL: testFunc2
		Execution failed:
			error: assertion failed: unexpected error occurred
			 --> 7465737400000000000000000000000000000000000000000000000000000000:9:12
			
- PASS: testFunc3
- FAIL: testFunc4
		Execution failed:
			error: panic: runtime error
			  --> 7465737400000000000000000000000000000000000000000000000000000000:17:12
			
`

	assert.Equal(t, expected, resultsStr)
}

func TestLoadingProgramsFromLocalFile(t *testing.T) {
	t.Parallel()

	t.Run("read script", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                var script = Test.readFile("./sample/script.cdc")

                var result = blockchain.executeScript(script, [])

                assert(result.status == Test.ResultStatus.succeeded)
                assert((result.returnValue! as! Int) == 5)
            }
        `

		const scriptCode = `
            pub fun main(): Int {
                return 2 + 3
            }
        `

		resolverInvoked := false
		fileResolver := func(path string) (string, error) {
			resolverInvoked = true
			assert.Equal(t, path, "./sample/script.cdc")

			return scriptCode, nil
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)

		assert.True(t, resolverInvoked)
	})

	t.Run("read invalid", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                var script = Test.readFile("./sample/script.cdc")

                var result = blockchain.executeScript(script, [])

                assert(result.status == Test.ResultStatus.succeeded)
                assert((result.returnValue! as! Int) == 5)
            }
        `

		resolverInvoked := false
		fileResolver := func(path string) (string, error) {
			resolverInvoked = true
			assert.Equal(t, path, "./sample/script.cdc")

			return "", fmt.Errorf("cannot find file %s", path)
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "cannot find file ./sample/script.cdc")

		assert.True(t, resolverInvoked)
	})

	t.Run("no resolver set", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                var blockchain = Test.newEmulatorBlockchain()
                var account = blockchain.createAccount()

                var script = Test.readFile("./sample/script.cdc")
            }
        `

		runner := NewTestRunner()

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &FileResolverNotProvidedError{})
	})
}

func TestDeployingContracts(t *testing.T) {
	t.Parallel()

	t.Run("no args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let contractCode = "pub contract Foo{ init(){}  pub fun sayHello(): String { return \"hello from Foo\"} }"

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }

                var script = "import Foo from ".concat(account.address.toString()).concat("\n")
                script = script.concat("pub fun main(): String {  return Foo.sayHello() }")

                let result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }

                let returnedStr = result.returnValue! as! String
                assert(returnedStr == "hello from Foo", message: "found: ".concat(returnedStr))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("with args", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let contractCode = "pub contract Foo{ pub let msg: String;   init(_ msg: String){ self.msg = msg }   pub fun sayHello(): String { return self.msg } }" 

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: ["hello from args"],
                )

                if err != nil {
                    panic(err!.message)
                }

                var script = "import Foo from ".concat(account.address.toString()).concat("\n")
                script = script.concat("pub fun main(): String {  return Foo.sayHello() }")

                let result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }

                let returnedStr = result.returnValue! as! String
                assert(returnedStr == "hello from args", message: "found: ".concat(returnedStr))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("contract deployment error", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let contractCode = "pub contract Foo{ init(){}  pub fun sayHello() { return 0 } }"

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "cannot deploy invalid contract")
	})

	t.Run("script error", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let script = "import Foo from 0x01; pub fun main() {}"
                let result = blockchain.executeScript(script, [])

                if result.status == Test.ResultStatus.failed {
                    panic(result.error!.message)
                }
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(
			t,
			result.Error.Error(),
			"cannot find declaration `Foo` in `0000000000000001.Foo`",
		)
	})

	t.Run("transaction error", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub fun test() {
                let blockchain = Test.newEmulatorBlockchain()
                let account = blockchain.createAccount()

                let tx2 = Test.Transaction(
                    code: "transaction { execute{ panic(\"some error\") } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = blockchain.executeTransaction(tx2)!

                if result.status == Test.ResultStatus.failed {
                    panic(result.error!.message)
                }
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "panic: some error")
	})
}

func TestInterpretFailFunction(t *testing.T) {
	t.Parallel()

	t.Run("without message", func(t *testing.T) {
		const script = `
        import Test

        pub fun test() {
            Test.fail()
        }
    `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &stdlib.AssertionError{})
	})

	t.Run("with message", func(t *testing.T) {
		const script = `
        import Test

        pub fun test() {
            Test.fail(message: "some error")
        }
    `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assertionErr := stdlib.AssertionError{}
		require.ErrorAs(t, result.Error, &assertionErr)
		assert.Contains(t, assertionErr.Message, "some error")
	})
}

func TestInterpretMatcher(t *testing.T) {
	t.Parallel()

	t.Run("custom matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher(fun (_ value: AnyStruct): Bool {
                     if !value.getType().isSubtype(of: Type<Int>()) {
                        return false
                    }

                    return (value as! Int) > 5
                })

                assert(matcher.test(8))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("custom matcher primitive type", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher(fun (_ value: Int): Bool {
                     return value == 7
                })

                assert(matcher.test(7))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("custom matcher invalid type usage", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher(fun (_ value: Int): Bool {
                     return (value + 7) == 4
                })

                // Invoke with an incorrect type
                assert(matcher.test("Hello"))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &interpreter.TypeMismatchError{})
	})

	t.Run("custom resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher(fun (_ value: &Foo): Bool {
                    return value.a == 4
                })

                let f <-create Foo(4)

                assert(matcher.test(&f as &Foo))

                destroy f
            }

            pub resource Foo {
                pub let a: Int

                init(_ a: Int) {
                    self.a = a
                }
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("custom resource matcher invalid type", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher(fun (_ value: @Foo): Bool {
                     destroy value
                     return true
                })
            }

            pub resource Foo {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)
		errs := checker.RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("custom matcher with explicit type", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher<Int>(fun (_ value: Int): Bool {
                     return value == 7
                })

                assert(matcher.test(7))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("custom matcher with mismatching types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher = Test.newMatcher<String>(fun (_ value: Int): Bool {
                     return value == 7
                })
            }
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)
		errs := checker.RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeParameterTypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("combined matcher mismatching types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {

                let matcher1 = Test.newMatcher(fun (_ value: Int): Bool {
                     return (value + 5) == 10
                })

                let matcher2 = Test.newMatcher(fun (_ value: String): Bool {
                     return value.length == 10
                })

                let matcher3 = matcher1.and(matcher2)

                // Invoke with a type that matches to only one matcher
                assert(matcher3.test(5))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &interpreter.TypeMismatchError{})
	})
}

func TestInterpretEqualMatcher(t *testing.T) {

	t.Parallel()

	t.Run("equal matcher with primitive", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let matcher = Test.equal(1)
                assert(matcher.test(1))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("equal matcher with struct", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let f = Foo()
                let matcher = Test.equal(f)
                assert(matcher.test(f))
            }

            pub struct Foo {}
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("equal matcher with resource", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let f <- create Foo()
                let matcher = Test.equal(<-f)
                assert(matcher.test(<- create Foo()))
            }

            pub resource Foo {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("with explicit types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let matcher = Test.equal<String>("hello")
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("with incorrect types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let matcher = Test.equal<String>(1)
            }
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeParameterTypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("matcher or", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneOrTwo = one.or(two)

                assert(oneOrTwo.test(1))
                assert(oneOrTwo.test(2))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("matcher or fail", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneOrTwo = one.or(two)

                assert(oneOrTwo.test(3))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &stdlib.AssertionError{})
	})

	t.Run("matcher and", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneAndTwo = one.and(two)

                assert(oneAndTwo.test(1))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &stdlib.AssertionError{})
	})

	t.Run("chained matchers", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)
                let three = Test.equal(3)

                let oneOrTwoOrThree = one.or(two).or(three)

                assert(oneOrTwoOrThree.test(3))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("resource matcher or", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let foo <- create Foo()
                let bar <- create Bar()

                let fooMatcher = Test.equal(<-foo)
                let barMatcher = Test.equal(<-bar)

                let matcher = fooMatcher.or(barMatcher)

                assert(matcher.test(<-create Foo()))
                assert(matcher.test(<-create Bar()))
            }

            pub resource Foo {}
            pub resource Bar {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 4)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[3])
	})

	t.Run("resource matcher and", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let foo <- create Foo()
                let bar <- create Bar()

                let fooMatcher = Test.equal(<-foo)
                let barMatcher = Test.equal(<-bar)

                let matcher = fooMatcher.and(barMatcher)

                assert(matcher.test(<-create Foo()))
            }

            pub resource Foo {}
            pub resource Bar {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 3)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
	})
}

func TestInterpretExpectFunction(t *testing.T) {

	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                Test.expect("this string", Test.equal("this string"))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("fail", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                Test.expect("this string", Test.equal("other string"))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &stdlib.AssertionError{})
	})

	t.Run("different types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                Test.expect("string", Test.equal(1))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.ErrorAs(t, result.Error, &stdlib.AssertionError{})
	})

	t.Run("with explicit types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                Test.expect<String>("hello", Test.equal("hello"))
            }
        `

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("mismatching types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                Test.expect<Int>("string", Test.equal(1))
            }
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)
		errs := checker.RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeParameterTypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("resource with resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let f1 <- create Foo()
                let f2 <- create Foo()
                Test.expect(<-f1, Test.equal(<-f2))
            }

            pub resource Foo {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("resource with struct matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let foo <- create Foo()
                let bar = Bar()
                Test.expect(<-foo, Test.equal(bar))
            }

            pub resource Foo {}
            pub struct Bar {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("struct with resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            pub fun test() {
                let foo = Foo()
                let bar <- create Bar()
                Test.expect(foo, Test.equal(<-bar))
            }

            pub struct Foo {}
            pub resource Bar {}
        `

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := checker.RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
}

func TestReplacingImports(t *testing.T) {
	t.Parallel()

	t.Run("file location", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub var blockchain = Test.newEmulatorBlockchain()
            pub var account = blockchain.createAccount()

            pub fun setup() {
                // Deploy the contract
                var contractCode = Test.readFile("./sample/contract.cdc")

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }

                // Set the configurations to use the address of the deployed contract.

                blockchain.useConfiguration(Test.Configuration({
                    "./FooContract": account.address
                }))
            }

            pub fun test() {
                var script = Test.readFile("./sample/script.cdc")
                var result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }
                assert((result.returnValue! as! String) == "hello from Foo")
            }
        `

		const contractCode = `
            pub contract Foo{ 
                init(){}

                pub fun sayHello(): String {
                    return "hello from Foo" 
                }
            }
        `

		const scriptCode = `
            import Foo from "./FooContract"

            pub fun main(): String {
                return Foo.sayHello()
            }
        `

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./sample/script.cdc":
				return scriptCode, nil
			case "./sample/contract.cdc":
				return contractCode, nil
			default:
				return "", fmt.Errorf("cannot find import location: %s", path)
			}
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("address location", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub var blockchain = Test.newEmulatorBlockchain()
            pub var account = blockchain.createAccount()

            pub fun setup() {
                var contractCode = Test.readFile("./sample/contract.cdc")

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }

                // Address locations are not replaceable!

                blockchain.useConfiguration(Test.Configuration({
                    "0x01": account.address
                }))
            }

            pub fun test() {
                var script = Test.readFile("./sample/script.cdc")
                var result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }
                assert((result.returnValue! as! String) == "hello from Foo")
            }
        `

		const contractCode = `
            pub contract Foo{ 
                init(){}

                pub fun sayHello(): String {
                    return "hello from Foo" 
                }
            }
        `

		const scriptCode = `
            import Foo from 0x01

            pub fun main(): String {
                return Foo.sayHello()
            }
        `

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./sample/script.cdc":
				return scriptCode, nil
			case "./sample/contract.cdc":
				return contractCode, nil
			default:
				return "", fmt.Errorf("cannot find import location: %s", path)
			}
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(
			t,
			result.Error.Error(),
			"cannot find declaration `Foo` in `0000000000000001.Foo`",
		)
	})

	t.Run("config not provided", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub var blockchain = Test.newEmulatorBlockchain()
            pub var account = blockchain.createAccount()

            pub fun setup() {
                var contractCode = Test.readFile("./sample/contract.cdc")

                let err = blockchain.deployContract(
                    name: "Foo",
                    code: contractCode,
                    account: account,
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }

                // Configurations are not provided.
            }

            pub fun test() {
                var script = Test.readFile("./sample/script.cdc")
                var result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }
                assert((result.returnValue! as! String) == "hello from Foo")
            }
        `

		const contractCode = `
            pub contract Foo{ 
                init(){}

                pub fun sayHello(): String {
                    return "hello from Foo" 
                }
            }
        `

		const scriptCode = `
            import Foo from "./FooContract"

            pub fun main(): String {
                return Foo.sayHello()
            }
        `

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./sample/script.cdc":
				return scriptCode, nil
			case "./sample/contract.cdc":
				return contractCode, nil
			default:
				return "", fmt.Errorf("cannot find import location: %s", path)
			}
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(
			t,
			result.Error.Error(),
			"expecting an AddressLocation, but other location types are passed",
		)
	})

	t.Run("config with missing imports", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            pub var blockchain = Test.newEmulatorBlockchain()
            pub var account = blockchain.createAccount()

            pub fun setup() {
                // Configurations provided, but some imports are missing.
                blockchain.useConfiguration(Test.Configuration({
                    "./FooContract": account.address
                }))
            }

            pub fun test() {
                var script = Test.readFile("./sample/script.cdc")
                var result = blockchain.executeScript(script, [])

                if result.status != Test.ResultStatus.succeeded {
                    panic(result.error!.message)
                }
                assert((result.returnValue! as! String) == "hello from Foo")
            }
        `

		const scriptCode = `
            import Foo from "./FooContract"
            import Foo from "./BarContract"  // This is missing in configs

            pub fun main(): String {
                return Foo.sayHello()
            }
        `

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./sample/script.cdc":
				return scriptCode, nil
			default:
				return "", fmt.Errorf("cannot find import location: %s", path)
			}
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(
			t,
			result.Error.Error(),
			"expecting an AddressLocation, but other location types are passed",
		)
	})
}

func TestReplaceImports(t *testing.T) {
	t.Parallel()

	emulatorBackend := NewEmulatorBackend(nil, nil, nil)
	emulatorBackend.UseConfiguration(&stdlib.Configuration{
		Addresses: map[string]common.Address{
			"./sample/contract1.cdc": {0x1},
			"./sample/contract2.cdc": {0x2},
			"./sample/contract3.cdc": {0x3},
		},
	})

	const code = `
        import C1 from "./sample/contract1.cdc"
        import C2 from "./sample/contract2.cdc"
        import C3 from "./sample/contract3.cdc"

        pub fun main() {}
    `
	const expected = `
        import C1 from 0x0100000000000000
        import C2 from 0x0200000000000000
        import C3 from 0x0300000000000000

        pub fun main() {}
    `

	replacedCode := emulatorBackend.replaceImports(code)

	assert.Equal(t, expected, replacedCode)
}

func TestServiceAccount(t *testing.T) {
	t.Parallel()

	t.Run("retrieve from EmulatorBackend", func(t *testing.T) {
		t.Parallel()

		emulatorBackend := NewEmulatorBackend(nil, nil, nil)

		serviceAccount, err := emulatorBackend.ServiceAccount()

		require.NoError(t, err)
		assert.Equal(
			t,
			"0xf8d6e0586b0a20c7",
			serviceAccount.Address.HexWithPrefix(),
		)
	})

	t.Run("retrieve from Test framework's blockchain", func(t *testing.T) {
		t.Parallel()

		const testCode = `
		    import Test

		    pub let blockchain = Test.newEmulatorBlockchain()

		    pub fun testGetServiceAccount() {
		        // Act
		        let account = blockchain.serviceAccount()

		        // Assert
		        Test.assert(account.address.getType() == Type<Address>())
		        Test.assert(account.publicKey.getType() == Type<PublicKey>())
		        Test.assert(account.address == 0xf8d6e0586b0a20c7)
		    }
		`

		runner := NewTestRunner()

		result, err := runner.RunTest(testCode, "testGetServiceAccount")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("run scripts and transactions with service account", func(t *testing.T) {
		t.Parallel()

		const testCode = `
		    import Test

		    pub let blockchain = Test.newEmulatorBlockchain()

		    pub fun testGetServiceAccountBalance() {
		        // Arrange
		        let account = blockchain.serviceAccount()

		        // Act
		        var script = Test.readFile("../scripts/get_account_balance.cdc")
		        let value = blockchain.executeScript(script, [account.address])
		        Test.assert(value.status == Test.ResultStatus.succeeded)
		        let balance = value.returnValue! as! UFix64

		        // Assert
		        Test.assert(balance == 1000000000.0)
		    }

		    pub fun testTransferFlowTokens() {
		        // Arrange
		        let account = blockchain.serviceAccount()
		        let receiver = blockchain.createAccount()

		        let code = Test.readFile("../transactions/transfer_flow_tokens.cdc")
		        let tx = Test.Transaction(
		            code: code,
		            authorizers: [account.address],
		            signers: [],
		            arguments: [receiver.address, 1500.0]
		        )

		        // Act
		        let result = blockchain.executeTransaction(tx)
		        Test.assert(result.status == Test.ResultStatus.succeeded)

		        var script = Test.readFile("../scripts/get_account_balance.cdc")
		        let value = blockchain.executeScript(script, [receiver.address])

		        Test.assert(value.status == Test.ResultStatus.succeeded)

		        let balance = value.returnValue! as! UFix64

		        // Assert
		        Test.assert(balance == 1500.0)
		    }
		`

		const scriptCode = `
		    import "FungibleToken"
		    import "FlowToken"

		    pub fun main(address: Address): UFix64 {
		        let balanceRef = getAccount(address)
		            .getCapability(/public/flowTokenBalance)
		            .borrow<&FlowToken.Vault{FungibleToken.Balance}>()
		            ?? panic("Could not borrow FungibleToken.Balance reference")

		        return balanceRef.balance
		    }
		`

		const transactionCode = `
		    import "FungibleToken"
		    import "FlowToken"

		    transaction(receiver: Address, amount: UFix64) {
		        prepare(account: AuthAccount) {
		            let flowVault = account.borrow<&FlowToken.Vault>(
		                from: /storage/flowTokenVault
		            ) ?? panic("Could not borrow BlpToken.Vault reference")

		            let receiverRef = getAccount(receiver)
		                .getCapability(/public/flowTokenReceiver)
		                .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
		                ?? panic("Could not borrow FungibleToken.Receiver reference")

		            let tokens <- flowVault.withdraw(amount: amount)
		            receiverRef.deposit(from: <- tokens)
		        }
		    }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "../scripts/get_account_balance.cdc":
				return scriptCode, nil
			case "../transactions/transfer_flow_tokens.cdc":
				return transactionCode, nil
			default:
				return "", fmt.Errorf("cannot find import location: %s", path)
			}
		}

		runner := NewTestRunner().WithFileResolver(fileResolver)

		result, err := runner.RunTest(testCode, "testGetServiceAccountBalance")
		require.NoError(t, err)
		require.NoError(t, result.Error)

		result, err = runner.RunTest(testCode, "testTransferFlowTokens")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestCoverageReportForUnitTests(t *testing.T) {
	t.Parallel()

	const fooContract = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {
	                1729: "Harshad",
	                8128: "Harmonic",
	                41041: "Carmichael"
	            }
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if n < 0 {
	                return "Negative"
	            } else if n == 0 {
	                return "Zero"
	            } else if n < 10 {
	                return "Small"
	            } else if n < 100 {
	                return "Big"
	            } else if n < 1000 {
	                return "Huge"
	            }

	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Enormous"
	        }
	    }
	`

	const code = `
	    import Test
	    import FooContract from "FooContract.cdc"

	    pub let foo = FooContract()

	    pub fun testGetIntegerTrait() {
	        // Arrange
	        let testInputs: {Int: String} = {
	            -1: "Negative",
	            0: "Zero",
	            9: "Small",
	            99: "Big",
	            999: "Huge",
	            1001: "Enormous",
	            1729: "Harshad",
	            8128: "Harmonic",
	            41041: "Carmichael"
	        }

	        for input in testInputs.keys {
	            // Act
	            let result = foo.getIntegerTrait(input)

	            // Assert
	            Test.assertEqual(result, testInputs[input]!)
	        }
	    }

	    pub fun testAddSpecialNumber() {
	        // Act
	        foo.addSpecialNumber(78557, "Sierpinski")

	        // Assert
	        Test.assertEqual("Sierpinski", foo.getIntegerTrait(78557))
	    }
	`

	importResolver := func(location common.Location) (string, error) {
		if location == common.StringLocation("FooContract.cdc") {
			return fooContract, nil
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	coverageReport := runtime.NewCoverageReport()
	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithCoverageReport(coverageReport)

	results, err := runner.RunTests(code)
	require.NoError(t, err)

	require.Len(t, results, 2)

	result1 := results[0]
	assert.Equal(t, result1.TestName, "testGetIntegerTrait")
	assert.NoError(t, result1.Error)

	result2 := results[1]
	assert.Equal(t, result2.TestName, "testAddSpecialNumber")
	require.NoError(t, result2.Error)

	location := common.StringLocation("FooContract.cdc")
	coverage := coverageReport.Coverage[location]

	assert.Equal(t, []int{}, coverage.MissedLines())
	assert.Equal(t, 15, coverage.Statements)
	assert.Equal(t, "100.0%", coverage.Percentage())
	assert.EqualValues(
		t,
		map[int]int{
			6: 1, 14: 1, 18: 10, 19: 1, 20: 9, 21: 1, 22: 8, 23: 1,
			24: 7, 25: 1, 26: 6, 27: 1, 30: 5, 31: 4, 34: 1,
		},
		coverage.LineHits,
	)

	assert.ElementsMatch(
		t,
		[]string{
			"A.0ae53cb6e3f42a79.FlowToken",
			"A.ee82856bf20e2aa6.FungibleToken",
			"A.e5a8b7f23e8b548f.FlowFees",
			"A.f8d6e0586b0a20c7.FlowStorageFees",
			"A.f8d6e0586b0a20c7.FlowServiceAccount",
			"A.f8d6e0586b0a20c7.FlowClusterQC",
			"A.f8d6e0586b0a20c7.FlowDKG",
			"A.f8d6e0586b0a20c7.FlowEpoch",
			"A.f8d6e0586b0a20c7.FlowIDTableStaking",
			"A.f8d6e0586b0a20c7.FlowStakingCollection",
			"A.f8d6e0586b0a20c7.LockedTokens",
			"A.f8d6e0586b0a20c7.NodeVersionBeacon",
			"A.f8d6e0586b0a20c7.StakingProxy",
			"s.7465737400000000000000000000000000000000000000000000000000000000",
			"I.Crypto",
			"I.Test",
			"A.f8d6e0586b0a20c7.FUSD",
			"A.f8d6e0586b0a20c7.ExampleNFT",
			"A.f8d6e0586b0a20c7.MetadataViews",
			"A.f8d6e0586b0a20c7.NFTStorefrontV2",
			"A.f8d6e0586b0a20c7.NFTStorefront",
			"A.f8d6e0586b0a20c7.NonFungibleToken",
			"A.f8d6e0586b0a20c7.ViewResolver",
		},
		coverageReport.ExcludedLocationIDs(),
	)
	assert.Equal(
		t,
		"Coverage: 97.2% of statements",
		coverageReport.String(),
	)
}

func TestCoverageReportForIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {
	                1729: "Harshad",
	                8128: "Harmonic",
	                41041: "Carmichael"
	            }
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if n < 0 {
	                return "Negative"
	            } else if n == 0 {
	                return "Zero"
	            } else if n < 10 {
	                return "Small"
	            } else if n < 100 {
	                return "Big"
	            } else if n < 1000 {
	                return "Huge"
	            }

	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Enormous"
	        }
	    }
	`

	const scriptCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    pub fun main(): Bool {
	        // Arrange
	        let testInputs: {Int: String} = {
	            -1: "Negative",
	            0: "Zero",
	            9: "Small",
	            99: "Big",
	            999: "Huge",
	            1001: "Enormous",
	            1729: "Harshad",
	            8128: "Harmonic",
	            41041: "Carmichael"
	        }

	        for input in testInputs.keys {
	            // Act
	            let result = FooContract.getIntegerTrait(input)

	            // Assert
	            assert(result == testInputs[input])
	        }

	        return true
	    }
	`

	const testCode = `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun setup() {
	        let contractCode = Test.readFile("../contracts/FooContract.cdc")
	        let err = blockchain.deployContract(
	            name: "FooContract",
	            code: contractCode,
	            account: account,
	            arguments: []
	        )

	        if err != nil {
	            panic(err!.message)
	        }

	        blockchain.useConfiguration(Test.Configuration({
	            "../contracts/FooContract.cdc": account.address
	        }))
	    }

	    pub fun testGetIntegerTrait() {
	        let script = Test.readFile("../scripts/get_integer_traits.cdc")
	        let result = blockchain.executeScript(script, [])

	        if result.status != Test.ResultStatus.succeeded {
	            panic(result.error!.message)
	        }
	        assert(result.returnValue! as! Bool)
	    }

	    pub fun testAddSpecialNumber() {
	        let code = Test.readFile("../transactions/add_special_number.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [account.address],
	            signers: [account],
	            arguments: [78557, "Sierpinski"]
	        )

	        let result = blockchain.executeTransaction(tx)
	        assert(result.status == Test.ResultStatus.succeeded)
	    }
	`

	const transactionCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    transaction(number: Int, trait: String) {
	        prepare(acct: AuthAccount) {}

	        execute {
	            // Act
	            FooContract.addSpecialNumber(number, trait)

	            // Assert
	            assert(trait == FooContract.getIntegerTrait(number))
	        }
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return contractCode, nil
		case "../scripts/get_integer_traits.cdc":
			return scriptCode, nil
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	coverageReport := runtime.NewCoverageReport()
	coverageReport.WithLocationFilter(func(location common.Location) bool {
		_, addressLoc := location.(common.AddressLocation)
		_, stringLoc := location.(common.StringLocation)
		// We only allow inspection of AddressLocation or StringLocation
		return addressLoc || stringLoc
	})
	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithCoverageReport(coverageReport)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)

	require.Len(t, results, 2)

	result1 := results[0]
	assert.Equal(t, result1.TestName, "testGetIntegerTrait")
	assert.NoError(t, result1.Error)

	result2 := results[1]
	assert.Equal(t, result2.TestName, "testAddSpecialNumber")
	require.NoError(t, result2.Error)

	address, err := common.HexToAddress("0x01cf0e2f2f715450")
	require.NoError(t, err)
	location := common.AddressLocation{
		Address: address,
		Name:    "FooContract",
	}
	coverage := coverageReport.Coverage[location]

	assert.Equal(t, []int{}, coverage.MissedLines())
	assert.Equal(t, 15, coverage.Statements)
	assert.Equal(t, "100.0%", coverage.Percentage())
	assert.EqualValues(
		t,
		map[int]int{
			6: 1, 14: 1, 18: 10, 19: 1, 20: 9, 21: 1, 22: 8, 23: 1,
			24: 7, 25: 1, 26: 6, 27: 1, 30: 5, 31: 4, 34: 1,
		},
		coverage.LineHits,
	)

	assert.ElementsMatch(
		t,
		[]string{
			"A.0ae53cb6e3f42a79.FlowToken",
			"A.ee82856bf20e2aa6.FungibleToken",
			"A.e5a8b7f23e8b548f.FlowFees",
			"A.f8d6e0586b0a20c7.FlowStorageFees",
			"A.f8d6e0586b0a20c7.FlowServiceAccount",
			"A.f8d6e0586b0a20c7.FlowClusterQC",
			"A.f8d6e0586b0a20c7.FlowDKG",
			"A.f8d6e0586b0a20c7.FlowEpoch",
			"A.f8d6e0586b0a20c7.FlowIDTableStaking",
			"A.f8d6e0586b0a20c7.FlowStakingCollection",
			"A.f8d6e0586b0a20c7.LockedTokens",
			"A.f8d6e0586b0a20c7.NodeVersionBeacon",
			"A.f8d6e0586b0a20c7.StakingProxy",
			"s.7465737400000000000000000000000000000000000000000000000000000000",
			"I.Crypto",
			"I.Test",
			"A.f8d6e0586b0a20c7.FUSD",
			"A.f8d6e0586b0a20c7.ExampleNFT",
			"A.f8d6e0586b0a20c7.MetadataViews",
			"A.f8d6e0586b0a20c7.NFTStorefrontV2",
			"A.f8d6e0586b0a20c7.NFTStorefront",
			"A.f8d6e0586b0a20c7.NonFungibleToken",
			"A.f8d6e0586b0a20c7.ViewResolver",
		},
		coverageReport.ExcludedLocationIDs(),
	)
	assert.Equal(t, 1, coverageReport.TotalLocations())
	assert.Equal(
		t,
		"Coverage: 100.0% of statements",
		coverageReport.String(),
	)
}

func TestRetrieveLogsFromUnitTests(t *testing.T) {
	t.Parallel()

	const fooContract = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {1729: "Harshad"}
	            log("init successful")
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	            log("specialNumbers updated")
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Unknown"
	        }
	    }
	`

	const code = `
	    import FooContract from "FooContract.cdc"

	    pub let foo = FooContract()

	    pub fun setup() {
	        log("setup successful")
	    }

	    pub fun testGetIntegerTrait() {
	        // Act
	        let result = foo.getIntegerTrait(1729)

	        // Assert
	        assert(result == "Harshad")
	        log("getIntegerTrait works")
	    }

	    pub fun testAddSpecialNumber() {
	        // Act
	        foo.addSpecialNumber(78557, "Sierpinski")

	        // Assert
	        assert("Sierpinski" == foo.getIntegerTrait(78557))
	        log("addSpecialNumber works")
	    }
	`

	importResolver := func(location common.Location) (string, error) {
		if location == common.StringLocation("FooContract.cdc") {
			return fooContract, nil
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	runner := NewTestRunner().WithImportResolver(importResolver)

	results, err := runner.RunTests(code)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}

	logs := runner.Logs()
	assert.ElementsMatch(
		t,
		[]string{
			"init successful",
			"setup successful",
			"getIntegerTrait works",
			"specialNumbers updated",
			"addSpecialNumber works",
		},
		logs,
	)
}

func TestRetrieveEmptyLogsFromUnitTests(t *testing.T) {
	t.Parallel()

	const fooContract = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {1729: "Harshad"}
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Unknown"
	        }
	    }
	`

	const code = `
	    import FooContract from "FooContract.cdc"

	    pub let foo = FooContract()

	    pub fun testGetIntegerTrait() {
	        // Act
	        let result = foo.getIntegerTrait(1729)

	        // Assert
	        assert(result == "Harshad")
	    }

	    pub fun testAddSpecialNumber() {
	        // Act
	        foo.addSpecialNumber(78557, "Sierpinski")

	        // Assert
	        assert("Sierpinski" == foo.getIntegerTrait(78557))
	    }
	`

	importResolver := func(location common.Location) (string, error) {
		if location == common.StringLocation("FooContract.cdc") {
			return fooContract, nil
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	runner := NewTestRunner().WithImportResolver(importResolver)

	results, err := runner.RunTests(code)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}

	logs := runner.Logs()
	assert.Equal(t, []string{}, logs)
}

func TestRetrieveLogsFromIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {1729: "Harshad"}
	            log("init successful")
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	            log("specialNumbers updated")
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Unknown"
	        }
	    }
	`

	const scriptCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    pub fun main(): Bool {
	        // Act
	        let trait = FooContract.getIntegerTrait(1729)

	        // Assert
	        assert(trait == "Harshad")

	        log("getIntegerTrait works")
	        return true
	    }
	`

	const testCode = `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun setup() {
	        let contractCode = Test.readFile("../contracts/FooContract.cdc")
	        let err = blockchain.deployContract(
	            name: "FooContract",
	            code: contractCode,
	            account: account,
	            arguments: []
	        )

	        if err != nil {
	            panic(err!.message)
	        }

	        blockchain.useConfiguration(Test.Configuration({
	            "../contracts/FooContract.cdc": account.address
	        }))
	    }

	    pub fun testGetIntegerTrait() {
	        let script = Test.readFile("../scripts/get_integer_traits.cdc")
	        let result = blockchain.executeScript(script, [])

	        if result.status != Test.ResultStatus.succeeded {
	            panic(result.error!.message)
	        }
	        assert(result.returnValue! as! Bool)
	    }

	    pub fun testAddSpecialNumber() {
	        let code = Test.readFile("../transactions/add_special_number.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [account.address],
	            signers: [account],
	            arguments: [78557, "Sierpinski"]
	        )

	        let result = blockchain.executeTransaction(tx)
	        assert(result.status == Test.ResultStatus.succeeded)
	    }

	    pub fun tearDown() {
	        let expectedLogs = [
	            "init successful",
	            "getIntegerTrait works",
	            "specialNumbers updated",
	            "addSpecialNumber works"
	        ]

	        for logMsg in blockchain.logs() {
	            Test.assert(expectedLogs.contains(logMsg))
	        }
	    }
	`

	const transactionCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    transaction(number: Int, trait: String) {
	        prepare(acct: AuthAccount) {}

	        execute {
	            // Act
	            FooContract.addSpecialNumber(number, trait)

	            // Assert
	            assert(trait == FooContract.getIntegerTrait(number))
	            log("addSpecialNumber works")
	        }
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return contractCode, nil
		case "../scripts/get_integer_traits.cdc":
			return scriptCode, nil
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestRetrieveEmptyLogsFromIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        init() {
	            self.specialNumbers = {1729: "Harshad"}
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Unknown"
	        }
	    }
	`

	const scriptCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    pub fun main(): Bool {
	        // Act
	        let trait = FooContract.getIntegerTrait(1729)

	        // Assert
	        assert(trait == "Harshad")

	        return true
	    }
	`

	const testCode = `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun setup() {
	        let contractCode = Test.readFile("../contracts/FooContract.cdc")
	        let err = blockchain.deployContract(
	            name: "FooContract",
	            code: contractCode,
	            account: account,
	            arguments: []
	        )

	        if err != nil {
	            panic(err!.message)
	        }

	        blockchain.useConfiguration(Test.Configuration({
	            "../contracts/FooContract.cdc": account.address
	        }))
	    }

	    pub fun testGetIntegerTrait() {
	        let script = Test.readFile("../scripts/get_integer_traits.cdc")
	        let result = blockchain.executeScript(script, [])

	        if result.status != Test.ResultStatus.succeeded {
	            panic(result.error!.message)
	        }
	        assert(result.returnValue! as! Bool)
	    }

	    pub fun testAddSpecialNumber() {
	        let code = Test.readFile("../transactions/add_special_number.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [account.address],
	            signers: [account],
	            arguments: [78557, "Sierpinski"]
	        )

	        let result = blockchain.executeTransaction(tx)
	        assert(result.status == Test.ResultStatus.succeeded)
	    }

	    pub fun tearDown() {
	        Test.assert(blockchain.logs() == [])
	    }
	`

	const transactionCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    transaction(number: Int, trait: String) {
	        prepare(acct: AuthAccount) {}

	        execute {
	            // Act
	            FooContract.addSpecialNumber(number, trait)

	            // Assert
	            assert(trait == FooContract.getIntegerTrait(number))
	        }
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return contractCode, nil
		case "../scripts/get_integer_traits.cdc":
			return scriptCode, nil
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestGetEventsFromIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
	    pub contract FooContract {
	        pub let specialNumbers: {Int: String}

	        pub event ContractInitialized()
	        pub event NumberAdded(n: Int, trait: String)

	        init() {
	            self.specialNumbers = {1729: "Harshad"}
	            emit ContractInitialized()
	        }

	        pub fun addSpecialNumber(_ n: Int, _ trait: String) {
	            self.specialNumbers[n] = trait
	            emit NumberAdded(n: n, trait: trait)
	        }

	        pub fun getIntegerTrait(_ n: Int): String {
	            if self.specialNumbers.containsKey(n) {
	                return self.specialNumbers[n]!
	            }

	            return "Unknown"
	        }
	    }
	`

	const scriptCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    pub fun main(): Bool {
	        // Act
	        let trait = FooContract.getIntegerTrait(1729)

	        // Assert
	        assert(trait == "Harshad")
	        return true
	    }
	`

	const testCode = `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun setup() {
	        let contractCode = Test.readFile("../contracts/FooContract.cdc")
	        let err = blockchain.deployContract(
	            name: "FooContract",
	            code: contractCode,
	            account: account,
	            arguments: []
	        )

	        if err != nil {
	            panic(err!.message)
	        }

	        blockchain.useConfiguration(Test.Configuration({
	            "../contracts/FooContract.cdc": account.address
	        }))
	    }

	    pub fun testGetIntegerTrait() {
	        let script = Test.readFile("../scripts/get_integer_traits.cdc")
	        let result = blockchain.executeScript(script, [])

	        if result.status != Test.ResultStatus.succeeded {
	            panic(result.error!.message)
	        }
	        assert(result.returnValue! as! Bool)

	        let typ = CompositeType("A.01cf0e2f2f715450.FooContract.ContractInitialized")!
	        let events = blockchain.eventsOfType(typ)
	        Test.assert(events.length == 1)
	    }

	    pub fun testAddSpecialNumber() {
	        let code = Test.readFile("../transactions/add_special_number.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [account.address],
	            signers: [account],
	            arguments: [78557, "Sierpinski"]
	        )

	        let result = blockchain.executeTransaction(tx)
	        assert(result.status == Test.ResultStatus.succeeded)

	        let typ = CompositeType("A.01cf0e2f2f715450.FooContract.NumberAdded")!
	        let events = blockchain.eventsOfType(typ)
	        Test.assert(events.length == 1)

	        let evts = blockchain.events()
	        Test.assert(evts.length >= 19)

	        let blockchain2 = Test.newEmulatorBlockchain()
	        Test.assert(blockchain2.events().length >= 19)
	    }
	`

	const transactionCode = `
	    import FooContract from "../contracts/FooContract.cdc"

	    transaction(number: Int, trait: String) {
	        prepare(acct: AuthAccount) {}

	        execute {
	            // Act
	            FooContract.addSpecialNumber(number, trait)

	            // Assert
	            assert(trait == FooContract.getIntegerTrait(number))
	        }
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return contractCode, nil
		case "../scripts/get_integer_traits.cdc":
			return scriptCode, nil
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		return "", nil
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithImportResolver(importResolver)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestImportingHelperFile(t *testing.T) {
	t.Parallel()

	const helpersCode = `
	    import Test

	    pub fun createTransaction(
	        _ path: String,
	        account: Test.Account,
	        args: [AnyStruct]
	    ): Test.Transaction {
	        return Test.Transaction(
	            code: Test.readFile(path),
	            authorizers: [account.address],
	            signers: [account],
	            arguments: args
	        )
	    }
	`

	const transactionCode = `
	    transaction() {
	        prepare(acct: AuthAccount) {}

	        execute {
	            assert(true)
	        }
	    }
	`

	const testCode = `
	    import Test
	    import "test_helpers.cdc"

	    pub let blockchain = Test.newEmulatorBlockchain()
	    pub let account = blockchain.createAccount()

	    pub fun testRunTransaction() {
	        let tx = createTransaction(
	            "../transactions/add_special_number.cdc",
	            account: account,
	            args: []
	        )

	        let result = blockchain.executeTransaction(tx)
	        Test.assert(result.status == Test.ResultStatus.succeeded)
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find file: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.StringLocation:
			if location == "test_helpers.cdc" {
				return helpersCode, nil
			}
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithImportResolver(importResolver)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestBlockchainReset(t *testing.T) {
	t.Parallel()

	const testCode = `
	    import Test

	    pub let blockchain = Test.newEmulatorBlockchain()

	    pub fun testBlockchainReset() {
	        // Arrange
	        let serviceAccount = blockchain.serviceAccount()
	        let receiver = blockchain.createAccount()

	        let code = Test.readFile("../transactions/transfer_flow_tokens.cdc")
	        let tx = Test.Transaction(
	            code: code,
	            authorizers: [serviceAccount.address],
	            signers: [],
	            arguments: [receiver.address, 1500.0]
	        )

	        // Act
	        var result = blockchain.executeTransaction(tx)
	        Test.assert(result.status == Test.ResultStatus.succeeded)

	        var script = Test.readFile("../scripts/get_account_balance.cdc")
	        var value = blockchain.executeScript(script, [serviceAccount.address])

	        Test.assert(value.status == Test.ResultStatus.succeeded)

	        var balance = value.returnValue! as! UFix64

	        // Assert
	        Test.assert(balance == 999998500.0)

	        // Act
	        blockchain.reset()

	        script = Test.readFile("../scripts/get_account_balance.cdc")
	        value = blockchain.executeScript(script, [serviceAccount.address])

	        Test.assert(value.status == Test.ResultStatus.succeeded)

	        balance = value.returnValue! as! UFix64

	        // Assert
	        Test.assert(balance == 1000000000.0)
	    }
	`

	const scriptCode = `
	    import "FungibleToken"
	    import "FlowToken"

	    pub fun main(address: Address): UFix64 {
	        let balanceRef = getAccount(address)
	            .getCapability(/public/flowTokenBalance)
	            .borrow<&FlowToken.Vault{FungibleToken.Balance}>()
	            ?? panic("Could not borrow FungibleToken.Balance reference")

	        return balanceRef.balance
	    }
	`

	const transactionCode = `
	    import "FungibleToken"
	    import "FlowToken"

	    transaction(receiver: Address, amount: UFix64) {
	        prepare(account: AuthAccount) {
	            let flowVault = account.borrow<&FlowToken.Vault>(
	                from: /storage/flowTokenVault
	            ) ?? panic("Could not borrow BlpToken.Vault reference")

	            let receiverRef = getAccount(receiver)
	                .getCapability(/public/flowTokenReceiver)
	                .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
	                ?? panic("Could not borrow FungibleToken.Receiver reference")

	            let tokens <- flowVault.withdraw(amount: amount)
	            receiverRef.deposit(from: <- tokens)
	        }
	    }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../scripts/get_account_balance.cdc":
			return scriptCode, nil
		case "../transactions/transfer_flow_tokens.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find import location: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)

	result, err := runner.RunTest(testCode, "testBlockchainReset")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}
