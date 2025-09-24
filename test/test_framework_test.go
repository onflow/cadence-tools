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
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onflow/cadence/test_utils/sema_utils"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
)

var firstAccountAddress = common.Address{0, 0, 0, 0, 0, 0, 0, 6}
var secondAccountAddress = common.Address{0, 0, 0, 0, 0, 0, 0, 7}
var thirdAccountAddress = common.Address{0, 0, 0, 0, 0, 0, 0, 8}

func TestRunningMultipleTests(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        access(all) fun testFunc1() {
            Test.assert(false)
        }

        access(all) fun testFunc2() {
            Test.assert(true)
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
        import Test

        access(all) fun testFunc1() {
            Test.assert(false)
        }

        access(all) fun testFunc2() {
            Test.assert(true)
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

        access(all) fun testAssertWithNoArgs() {
            Test.assert(true)
        }

        access(all) fun testAssertWithNoArgsFail() {
            Test.assert(false)
        }

        access(all) fun testAssertWithMessage() {
            Test.assert(true, message: "some reason")
        }

        access(all) fun testAssertWithMessageFail() {
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

            access(all)
            fun test() {
                let result = Test.executeScript(
                    "access(all) fun main(): Int {  return 2 + 3 }",
                    []
                )

                Test.expect(result, Test.beSucceeded())
                Test.assertEqual(5, result.returnValue! as! Int)
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

            access(all)
            fun test() {
                let result = Test.executeScript(
                    "access(all) fun main(a: Int, b: Int): Int {  return a + b }",
                    [2, 3]
                )

                Test.expect(result, Test.beSucceeded())
                Test.assertEqual(5, result.returnValue! as! Int)
            }
		`

		runner := NewTestRunner()
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("non-empty array returns", func(t *testing.T) {
		t.Parallel()

		const code = `access(all) fun main(): [UInt64] { return [1, 2, 3] }`

		testScript := fmt.Sprintf(`
            import Test

            access(all)
            fun test() {
                let result = Test.executeScript("%s", [])

                Test.expect(result, Test.beSucceeded())

                let expected: [UInt64] = [1, 2, 3]
                let resultArray = result.returnValue! as! [UInt64]
                Test.assertEqual(expected, resultArray)
            }
		`, code)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("empty array returns", func(t *testing.T) {
		t.Parallel()

		const code = `access(all) fun main(): [UInt64] { return [] }`

		testScript := fmt.Sprintf(`
            import Test

            access(all)
            fun test() {
                let result = Test.executeScript("%s", [])

                Test.expect(result, Test.beSucceeded())

                let expected: [UInt64] = []
                let resultArray = result.returnValue! as! [UInt64]
                Test.assertEqual(expected, resultArray)
            }
		`, code)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("non-empty dictionary returns", func(t *testing.T) {
		t.Parallel()

		const code = `access(all) fun main(): {String: Int} { return {\"foo\": 5, \"bar\": 10} }`

		testScript := fmt.Sprintf(`
            import Test

            access(all)
            fun test() {
                let result = Test.executeScript("%s", [])

                Test.expect(result, Test.beSucceeded())

                let expected: {String: Int} = {"foo": 5, "bar": 10}
                let resultDict = result.returnValue! as! {String: Int}
                Test.assertEqual(expected, resultDict)
            }
		`, code)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("empty dictionary returns", func(t *testing.T) {
		t.Parallel()

		const code = `access(all) fun main(): {String: Int} { return {} }`

		testScript := fmt.Sprintf(`
            import Test

            access(all)
            fun test() {
                let result = Test.executeScript("%s", [])

                Test.expect(result, Test.beSucceeded())

                let expected: {String: Int} = {}
                let resultDict = result.returnValue! as! {String: Int}
                Test.assertEqual(expected, resultDict)
            }
		`, code)

		runner := NewTestRunner()
		result, err := runner.RunTest(testScript, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestImportContract(t *testing.T) {
	t.Parallel()

	t.Run("contract with no init params", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import "FooContract"

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: []
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                Test.assertEqual("hello from Foo", FooContract.sayHello())
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
			case common.StringLocation:
				if location == "FooContract" {
					return fooContract, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("contract with init params", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import FooContract from "./FooContract"

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: ["hello from Foo"]
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                Test.assertEqual("hello from Foo", FooContract.sayHello())
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {

                access(all)
                var greeting: String

                init(greeting: String) {
                    self.greeting = greeting
                }

                access(all)
                fun sayHello(): String {
                    return self.greeting
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("invalid import", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            access(all)
            fun test() {
                let message = FooContract.sayHello()
            }
		`

		importResolver := func(location common.Location) (string, error) {
			return "", errors.New("cannot import location")
		}

		runner := NewTestRunner().WithImportResolver(importResolver)

		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		assert.ErrorAs(t, errs[0], &importedProgramError)
		assert.Contains(t, importedProgramError.Err.Error(), "cannot import location")

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("import resolver not provided", func(t *testing.T) {
		t.Parallel()

		const code = `
            import FooContract from "./FooContract"

            access(all)
            fun test() {
                let message = FooContract.sayHello()
            }
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(code, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 2)

		importedProgramError := &sema.ImportedProgramError{}
		require.ErrorAs(t, errs[0], &importedProgramError)
		assert.IsType(t, ImportResolverNotProvidedError{}, importedProgramError.Err)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[1])
	})

	t.Run("nested imports", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import FooContract from "./FooContract"

            access(all)
            fun setup() {
                var err = Test.deployContract(
                    name: "BazContract",
                    path: "./BazContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())

                err = Test.deployContract(
                    name: "BarContract",
                    path: "./BarContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())

                err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                Test.assertEqual("Hi from BazContract", FooContract.sayHi())
            }
		`

		const fooContract = `
            import BarContract from "./BarContract"

            access(all) contract FooContract {
                init() {}

                access(all)
                fun sayHi(): String {
                    return BarContract.sayHi()
                }
            }
		`

		const barContract = `
            import BazContract from "./BazContract"

            access(all) contract BarContract {
                init() {}

                access(all)
                fun sayHi(): String {
                    return BazContract.sayHi()
                }
            }
		`

		const bazContract = `
            access(all) contract BazContract {
                init() {}

                access(all)
                fun sayHi(): String {
                    return "Hi from BazContract"
                }
            }
		`

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
				if location.Name == "BarContract" {
					return barContract, nil
				}
				if location.Name == "BazContract" {
					return bazContract, nil
				}
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
				if location == "./BarContract" {
					return barContract, nil
				}
				if location == "./BazContract" {
					return bazContract, nil
				}
			}

			return "", fmt.Errorf("unsupported import %s", location)
		}

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			case "./BarContract":
				return barContract, nil
			case "./BazContract":
				return bazContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
			"BarContract": secondAccountAddress,
			"BazContract": thirdAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithContracts(contracts).
			WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("multiple imports", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import BlockchainHelpers
            import Crypto
            import "FlowToken"
            import "RandomBeaconHistory"
            import BarContract from "./BarContract"
            import FooContract from "./FooContract"
            import BazContract from "./BazContract"

            access(all) let admin = Test.getAccount(0x0000000000000008)

            access(all)
            fun setup() {
                var err = Test.deployContract(
                    name: "BarContract",
                    path: "./BarContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())

                err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())

                err = Test.deployContract(
                    name: "BazContract",
                    path: "./BazContract",
                    arguments: []
                )
                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                var scriptResult = Test.executeScript(
                    "access(all) fun main(): Int { return 5 }",
                    []
                )
                var value = scriptResult.returnValue! as! Int
                Test.assertEqual(5, value)

                // Test access of methods & fields from built-in contract
                let keyList = Crypto.KeyList()
                Test.assert(keyList.get(keyIndex: 0) == nil)

                let hash = Crypto.hash([1, 2, 3], algorithm: HashAlgorithm.SHA3_256)
                Test.assertEqual(32, hash.length)

                let totalSupply = FlowToken.totalSupply
                Test.assertEqual(1000000000.0, totalSupply)

                // Test access of methods & fields from deployed contract
                Test.assertEqual("Hi from BarContract", BarContract.sayHi())
                Test.assertEqual(/public/BarContractPublicPath, BarContract.publicPath)

                // TODO:
                //Test.assertEqual(
                //    &["one", "two", "three"] as &[String],
                //    BarContract.proposals
                //)
                Test.assertEqual("one", BarContract.proposals[0])
                Test.assertEqual("two", BarContract.proposals[1])
                Test.assertEqual("three", BarContract.proposals[2])

                // Test access of methods & fields from deployed contract
                Test.assertEqual("Hi from FooContract", FooContract.sayHi())

                // TODO:
                //Test.assertEqual(
                //    &{1: "one", 2: "two", 3: "three"} as &{Int: String},
                //    FooContract.numbers
                //)
                Test.assertEqual("one" as String?, FooContract.numbers[1])
                Test.assertEqual("two" as String?, FooContract.numbers[2])
                Test.assertEqual("three" as String?, FooContract.numbers[3])

                // Test access of methods & fields from deployed contract
                Test.assertEqual("Hi from BazContract", BazContract.sayHi())
                Test.assertEqual(/public/BazContractPublicPath, BazContract.publicPath)

                // Test that we can still create accounts, execute scripts
                // and transactions.
                scriptResult = Test.executeScript(
                    "access(all) fun main(): Int { return 15 }",
                    []
                )
                value = scriptResult.returnValue! as! Int
                Test.assertEqual(15, value)

                let account = Test.createAccount()
                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )
                let result = Test.executeTransaction(tx)

                Test.expect(result, Test.beSucceeded())

                let blockHeight = getCurrentBlockHeight()
                Test.assert(blockHeight > 1)

                mintFlow(to: admin, amount: 500.0)
                Test.assertEqual(500.0, getFlowBalance(account: admin))

                let lowestHeight = RandomBeaconHistory.getLowestHeight()
                Test.assertEqual(UInt64(1), lowestHeight)

                let randomSource = RandomBeaconHistory.sourceOfRandomness(atBlockHeight: lowestHeight)
                let expectedSource = RandomBeaconHistory.RandomSource(
                    blockHeight: lowestHeight,
                    value: [
                        7, 58, 231, 186, 127, 87, 155, 70, 128, 9, 11, 130, 196, 94, 144, 52, 140,
                        71, 60, 164, 34, 100, 227, 42, 235, 244, 53, 223, 152, 116, 213, 241
                    ] as [UInt8]
                )
                Test.assertEqual(expectedSource, randomSource)
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {
                access(all)
                let numbers: {Int: String}

                init() {
                    self.numbers = {1: "one", 2: "two", 3: "three"}
                }

                access(all)
                fun sayHi(): String {
                    return "Hi from FooContract"
                }
            }
		`

		const barContract = `
            access(all)
            contract BarContract {
                access(all)
                let publicPath: PublicPath
                access(all)
                let proposals: [String]

                init() {
                    self.publicPath = /public/BarContractPublicPath
                    self.proposals = ["one", "two", "three"]
                }

                access(all)
                fun sayHi(): String {
                    return "Hi from BarContract"
                }
            }
		`

		const bazContract = `
            access(all)
            contract BazContract {
                access(all)
                let publicPath: PublicPath

                init() {
                    self.publicPath = /public/BazContractPublicPath
                }

                access(all)
                fun sayHi(): String {
                    return "Hi from BazContract"
                }
            }
		`

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
				if location.Name == "BarContract" {
					return barContract, nil
				}
				if location.Name == "BazContract" {
					return bazContract, nil
				}
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
				if location == "./BarContract" {
					return barContract, nil
				}
				if location == "./BazContract" {
					return bazContract, nil
				}
			}

			return "", fmt.Errorf("unsupported import %s", location)
		}

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			case "./BarContract":
				return barContract, nil
			case "./BazContract":
				return bazContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"BarContract": firstAccountAddress,
			"FooContract": firstAccountAddress,
			"BazContract": thirdAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithContracts(contracts).
			WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("undeployed contract", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import FooContract from "./FooContract"

            access(all)
            fun test() {
                Test.assertEqual("Hello", FooContract.sayHello())
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "Hello"
                }
            }
		`

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
			case common.StringLocation:
				if location == "./FooContract" {
					return fooContract, nil
				}
			}

			return "", fmt.Errorf("unsupported import %s", location)
		}

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithContracts(contracts).
			WithFileResolver(fileResolver)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		assert.ErrorContains(
			t,
			result.Error,
			"failed to load contract: 0000000000000006.FooContract",
		)
	})
}

func TestImportCryptoContract(t *testing.T) {
	t.Parallel()

	const code = `
        import Test
        import Crypto
        import FooContract from "./FooContract"

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "./FooContract",
                arguments: []
            )
            Test.expect(err, Test.beNil())
        }

        access(all)
        fun test() {
            let hash = Crypto.hash(
                [0, 0, 1, 2, 3, 5, 8, 11],
                algorithm: HashAlgorithm.SHA3_256
            )
            Test.assertEqual(hash, FooContract.hashNumbers())
        }
	`

	const fooContract = `
        import Crypto

        access(all) contract FooContract {
            access(self) let numbers: [UInt8]

            init() {
                self.numbers = [0, 0, 1, 2, 3, 5, 8, 11]
            }

            access(all)
            fun hashNumbers(): [UInt8] {
                return Crypto.hash(
                    self.numbers,
                    algorithm: HashAlgorithm.SHA3_256
                )
            }
        }
	`

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return fooContract, nil
			}
		case common.StringLocation:
			if location == "./FooContract" {
				return fooContract, nil
			}
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	fileResolver := func(path string) (string, error) {
		switch path {
		case "./FooContract":
			return fooContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithContracts(contracts).
		WithFileResolver(fileResolver)

	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestImportBuiltinContracts(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import "ExampleNFT"
        import "NonFungibleToken"
        import "FlowToken"

        access(all)
        let account = Test.createAccount()

        access(all)
        fun testTransaction() {
            let code = Test.readFile("../transactions/test_transaction.cdc")
            let tx = Test.Transaction(
                code: code,
                authorizers: [account.address],
                signers: [account],
                arguments: []
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())
        }

        access(all)
        fun testScript() {
            let script = Test.readFile("../scripts/test_script.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assertEqual(true, result.returnValue! as! Bool)
        }

        access(all)
        fun testExampleNFT() {
            let storagePath = ExampleNFT.MinterStoragePath
            Test.assertEqual(/storage/exampleNFTMinter, storagePath)

            Test.assertEqual(
                "{A.0000000000000001.NonFungibleToken}",
                Type<{NonFungibleToken}>().identifier
            )

            let vault <- FlowToken.createEmptyVault(vaultType: Type<@FlowToken.Vault>())
            Test.assertEqual(0.0, vault.balance)
            destroy <- vault
        }

        access(all)
        fun testEVMContract() {
            let script = Test.readFile("../scripts/test_evm_contract.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assertEqual(true, result.returnValue! as! Bool)
        }
	`

	const testTransactionCode = `
        import "NonFungibleToken"
        import "ExampleNFT"
        import "MetadataViews"

        transaction {

            prepare(signer: &Account) {}
        }
	`

	const testScriptCode = `
        import "FungibleToken"
        import "FlowToken"
        import "NonFungibleToken"
        import "MetadataViews"
        import "ViewResolver"
        import "ExampleNFT"

        access(all) fun main(): Bool {
            return true
        }
	`

	const testEVMContract = `
        import "EVM"
        import "Burner"

        access(all) fun main(): Bool {
            let coa <- EVM.createCadenceOwnedAccount()
            let address = coa.address()
            Burner.burn(<- coa)

            assert(address.bytes != [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0])
            return true
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../transactions/test_transaction.cdc":
			return testTransactionCode, nil
		case "../scripts/test_script.cdc":
			return testScriptCode, nil
		case "../scripts/test_evm_contract.cdc":
			return testEVMContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		return "", fmt.Errorf("cannot find import location: %s", location)
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

func TestUsingEnv(t *testing.T) {
	t.Parallel()

	t.Run("public key creation", func(t *testing.T) {
		t.Parallel()

		const code = `
            access(all) fun test() {
                let publicKey = PublicKey(
                    publicKey: "1234".decodeHex(),
                    signatureAlgorithm: SignatureAlgorithm.ECDSA_secp256k1
                )
            }
		`

		runner := NewTestRunner()

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)

		var publicKeyError *interpreter.InvalidPublicKeyError
		assert.ErrorAs(t, result.Error, &publicKeyError)
	})

	t.Run("public account", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            fun test() {
                let acc = getAccount(0x10)
                Test.assertEqual(0.0, acc.balance)
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
            import Test
            import "FooContract"

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: []
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                Test.assertEqual(0.0, FooContract.getBalance())
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {
                init() {}

                access(all)
                fun getBalance(): UFix64 {
                    let acc = getAccount(0x10)
                    return acc.balance
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
			case common.StringLocation:
				if location == "FooContract" {
					return fooContract, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("verify using public key of account", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            fun test() {
                let account = Test.createAccount()

                // just checking the invocation of verify function
                Test.assert(!account.publicKey.verify(
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let script = Test.readFile("./sample/script.cdc")

                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())

                let pubKey = result.returnValue! as! PublicKey

                // just checking the invocation of verify function
                Test.assert(!pubKey.verify(
                   signature: [],
                   signedData: [],
                   domainSeparationTag: "",
                   hashAlgorithm: HashAlgorithm.SHA2_256
                ))
            }
		`

		const scriptCode = `
            access(all) fun main(): PublicKey {
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

        access(all)
        fun test() {
            let account = Test.createAccount()

            let typ = CompositeType("flow.AccountCreated")!
            let events = Test.eventsOfType(typ)
            Test.expect(events.length, Test.beGreaterThan(1))
        }
	`

	importResolver := func(location common.Location) (string, error) {
		return "", nil
	}

	runner := NewTestRunner().WithImportResolver(importResolver)
	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestGetAccount(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        access(all)
        fun testMissingAccount() {
            let account = Test.getAccount(0x0000000000000095)

            Test.assertEqual(0x0000000000000005 as Address, account.address)
        }

        access(all)
        fun testExistingAccount() {
            let admin = Test.createAccount()
            let account = Test.getAccount(admin.address)

            Test.assertEqual(account.address, admin.address)
            Test.assertEqual(account.publicKey.publicKey, admin.publicKey.publicKey)
        }
	`

	importResolver := func(location common.Location) (string, error) {
		return "", nil
	}

	runner := NewTestRunner().WithImportResolver(importResolver)
	result, err := runner.RunTest(code, "testMissingAccount")
	require.NoError(t, err)
	require.ErrorContains(
		t,
		result.Error,
		"account with address: 0x0000000000000095 was not found",
	)

	result, err = runner.RunTest(code, "testExistingAccount")
	require.NoError(t, err)
	assert.NoError(t, result.Error)
}

func TestExecutingTransactions(t *testing.T) {
	t.Parallel()

	t.Run("add transaction", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx)
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx)

                let result = Test.executeNextTransaction()!
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { prepare(acct: &Account) {} execute { assert(true) } }",
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx)

                let result = Test.executeNextTransaction()!
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx)

                let result = Test.executeNextTransaction()!
                Test.expect(result, Test.beFailed())
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

            access(all)
            fun test() {
                let result = Test.executeNextTransaction()
                Test.expect(result, Test.beNil())
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

            access(all)
            fun test() {
                Test.commitBlock()
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx)

                Test.commitBlock()
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(false) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                // Add two transactions
                Test.addTransaction(tx)
                Test.addTransaction(tx)

                // But execute only one
                Test.executeNextTransaction()

                // Then try to commit
                Test.commitBlock()
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

            access(all)
            fun test() {
                Test.commitBlock()
                Test.commitBlock()
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = Test.executeTransaction(tx)
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction(a: Int, b: Int) { execute{ assert(a == b) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [4, 4],
                )

                let result = Test.executeTransaction(tx)
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account1 = Test.createAccount()
                let account2 = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction() { prepare(acct1: &Account, acct2: &Account) {}  }",
                    authorizers: [account1.address, account2.address],
                    signers: [account1, account2],
                    arguments: [],
                )

                let result = Test.executeTransaction(tx)
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction { execute{ assert(fail) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = Test.executeTransaction(tx)
                Test.expect(result, Test.beFailed())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx1 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let tx2 = Test.Transaction(
                    code: "transaction { prepare(acct: &Account) {} execute{ assert(true) } }",
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

                let firstResults = Test.executeTransactions([tx1, tx2, tx3])

                Test.assertEqual(3, firstResults.length)
                Test.expect(firstResults[0], Test.beSucceeded())
                Test.expect(firstResults[1], Test.beSucceeded())
                Test.expect(firstResults[2], Test.beFailed())


                // Execute them again: To verify the proper increment/reset of sequence numbers.
                let secondResults = Test.executeTransactions([tx1, tx2, tx3])

                Test.assertEqual(3, secondResults.length)
                Test.expect(secondResults[0], Test.beSucceeded())
                Test.expect(secondResults[1], Test.beSucceeded())
                Test.expect(secondResults[2], Test.beFailed())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let result = Test.executeTransactions([])
                Test.assertEqual(0, result.length)
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx1 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                Test.addTransaction(tx1)

                let tx2 = Test.Transaction(
                    code: "transaction { execute{ assert(true) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = Test.executeTransaction(tx2)
                Test.expect(result, Test.beSucceeded())
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx = Test.Transaction(
                    code: "transaction(a: [Int]) { execute{ assert(a[0] == a[1]) } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [[4, 4]],
                )

                let result = Test.executeTransaction(tx)
                Test.expect(result, Test.beSucceeded())
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
            import Test

            access(all)
            fun setup() {
                log("setup is running!")
            }

            access(all)
            fun testFunc() {
                Test.assert(true)
            }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 1)
		result := results[0]
		assert.Equal(t, result.TestName, "testFunc")
		require.NoError(t, result.Error)

		assert.ElementsMatch(t,
			[]string{"setup is running!"},
			runner.Logs(),
		)
	})

	t.Run("setup failed", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all) fun setup() {
                panic("error occurred")
            }

            access(all) fun testFunc() {
                Test.assert(true)
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
            import Test

            access(all)
            fun testFunc() {
                Test.assert(true)
            }

            access(all)
            fun tearDown() {
                log("tearDown is running!")
            }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.NoError(t, err)

		require.Len(t, results, 1)
		result := results[0]
		assert.Equal(t, result.TestName, "testFunc")
		require.NoError(t, result.Error)

		assert.ElementsMatch(t,
			[]string{"tearDown is running!"},
			runner.Logs(),
		)
	})

	t.Run("teardown failed", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            var tearDownRan = false

            access(all)
            fun testFunc() {
                Test.assert(!tearDownRan)
            }

            access(all)
            fun tearDown() {
                Test.assert(false)
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

		const code = `
            import Test

            access(all)
            var counter = 0

            access(all)
            fun beforeEach() {
                counter = counter + 1
            }

            access(all)
            fun testFuncOne() {
                Test.assertEqual(1, counter)
            }

            access(all)
            fun testFuncTwo() {
                Test.assertEqual(2, counter)
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

		const code = `
            import Test

            access(all)
            fun beforeEach() {
                panic("error occurred")
            }

            access(all)
            fun testFunc() {
                Test.assert(true)
            }
		`

		runner := NewTestRunner()
		results, err := runner.RunTests(code)
		require.Error(t, err)
		require.Empty(t, results)
	})

	t.Run("afterEach", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            var counter = 2

            access(all)
            fun afterEach() {
                counter = counter - 1
            }

            access(all)
            fun testFuncOne() {
                Test.assertEqual(2, counter)
            }

            access(all)
            fun testFuncTwo() {
                Test.assertEqual(1, counter)
            }

            access(all)
            fun tearDown() {
                Test.assertEqual(0, counter)
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

		const code = `
            import Test

            access(all)
            var tearDownRan = false

            access(all) fun testFunc() {
                Test.assert(!tearDownRan)
            }

            access(all) fun afterEach() {
                Test.assert(false)
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

        access(all) fun testFunc1() {
            Test.assert(true, message: "should pass")
        }

        access(all) fun testFunc2() {
            Test.assert(false, message: "unexpected error occurred")
        }

        access(all) fun testFunc3() {
            Test.assert(true, message: "should pass")
        }

        access(all) fun testFunc4() {
            panic("runtime error")
        }
	`

	runner := NewTestRunner()
	results, err := runner.RunTests(code)
	require.NoError(t, err)

	resultsStr := PrettyPrintResults(results, "tests/test_script.cdc")

	const expected = `Test results: "tests/test_script.cdc"
- PASS: testFunc1
- FAIL: testFunc2
		Execution failed:
			error: assertion failed: unexpected error occurred
			 --> tests/test_script.cdc:9:12
			
			Was this error unhelpful?
			Consider suggesting an improvement here: https://github.com/onflow/cadence/issues.
			
- PASS: testFunc3
- FAIL: testFunc4
		Execution failed:
			error: panic: runtime error
			  --> tests/test_script.cdc:17:12
			
			Was this error unhelpful?
			Consider suggesting an improvement here: https://github.com/onflow/cadence/issues.
			
`

	assert.Equal(t, expected, resultsStr)
}

func TestLoadingProgramsFromLocalFile(t *testing.T) {
	t.Parallel()

	t.Run("read script", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            fun test() {
                let account = Test.createAccount()

                let script = Test.readFile("./sample/script.cdc")

                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())
                Test.assertEqual(5, result.returnValue! as! Int)
            }
		`

		const scriptCode = `
            access(all) fun main(): Int {
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let script = Test.readFile("./sample/script.cdc")

                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())
                Test.assertEqual(5, result.returnValue! as! Int)
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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let script = Test.readFile("./sample/script.cdc")
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

		const contract = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		const script = `
            import Foo from "Foo.cdc"

            access(all)
            fun main(): String {
                return Foo.sayHello()
            }
		`

		const code = `
            import Test

            access(all)
            fun test() {
                let account = Test.getAccount(0x0000000000000006)

                let err = Test.deployContract(
                    name: "Foo",
                    path: "Foo.cdc",
                    arguments: [],
                )

                Test.expect(err, Test.beNil())

                let script = Test.readFile("say_hello.cdc")

                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())

                let returnedStr = result.returnValue! as! String
                Test.assertEqual("hello from Foo", returnedStr)
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "Foo.cdc":
				return contract, nil
			case "say_hello.cdc":
				return script, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("with args", func(t *testing.T) {
		t.Parallel()

		const contract = `
            access(all)
            contract Foo {
                access(all)
                let msg: String

                init(_ msg: String) {
                    self.msg = msg
                }

                access(all)
                fun sayHello(): String {
                    return self.msg
                }
            }
		`

		const script = `
            import Foo from "Foo.cdc"

            access(all)
            fun main(): String {
                return Foo.sayHello()
            }
		`

		const code = `
            import Test

            access(all)
            fun test() {
                let account = Test.getAccount(0x0000000000000006)

                let err = Test.deployContract(
                    name: "Foo",
                    path: "Foo.cdc",
                    arguments: ["hello from args"],
                )

                Test.expect(err, Test.beNil())

                let script = Test.readFile("say_hello.cdc")

                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())

                let returnedStr = result.returnValue! as! String
                Test.assertEqual("hello from args", returnedStr)
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "Foo.cdc":
				return contract, nil
			case "say_hello.cdc":
				return script, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("contract deployment error", func(t *testing.T) {
		t.Parallel()

		const contract = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello() {
                    return 0
                }
            }
		`

		const code = `
            import Test

            access(all)
            fun test() {
                let err = Test.deployContract(
                    name: "Foo",
                    path: "Foo.cdc",
                    arguments: [],
                )

                if err != nil {
                    panic(err!.message)
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "Foo.cdc":
				return contract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)
		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "cannot deploy invalid contract")
	})

	t.Run("script error", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            fun test() {
                let script = "import Foo from 0x01; access(all) fun main() {}"
                let result = Test.executeScript(script, [])

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

            access(all)
            fun test() {
                let account = Test.createAccount()

                let tx2 = Test.Transaction(
                    code: "transaction { execute{ panic(\"some error\") } }",
                    authorizers: [],
                    signers: [account],
                    arguments: [],
                )

                let result = Test.executeTransaction(tx2)!

                Test.assertError(result, errorMessage: "some error")
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
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
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
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
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

            access(all) fun test() {
                let matcher = Test.newMatcher(fun (_ value: AnyStruct): Bool {
                     if !value.getType().isSubtype(of: Type<Int>()) {
                        return false
                    }
                    return (value as! Int) > 5
                })

                Test.expect(8, matcher)
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

            access(all) fun test() {
                let matcher = Test.newMatcher(fun (_ value: Int): Bool {
                    return value == 7
                })

                Test.expect(7, matcher)
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

            access(all) fun test() {
                let matcher = Test.newMatcher(fun (_ value: Int): Bool {
                    return (value + 7) == 4
                })

                // Invoke with an incorrect type
                Test.expect("Hello", matcher)
            }
		`

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)

		var typeMismatchErr *interpreter.TypeMismatchError
		assert.ErrorAs(t, result.Error, &typeMismatchErr)
	})

	t.Run("custom resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let matcher = Test.newMatcher(fun (_ value: &Foo): Bool {
                    return value.a == 4
                })

                let f <-create Foo(4)

                Test.expect((&f as &Foo), matcher)

                destroy f
            }

            access(all) resource Foo {
                access(all) let a: Int

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

            access(all) fun test() {
                let matcher = Test.newMatcher(fun (_ value: @Foo): Bool {
                    destroy value
                    return true
                })
            }

            access(all) resource Foo {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)
		errs := RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("custom matcher with explicit type", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let matcher = Test.newMatcher<Int>(fun (_ value: Int): Bool {
                    return value == 7
                })

                Test.expect(7, matcher)
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

            access(all) fun test() {
                let matcher = Test.newMatcher<String>(fun (_ value: Int): Bool {
                    return value == 7
                })
            }
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("combined matcher mismatching types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let matcher1 = Test.newMatcher(fun (_ value: Int): Bool {
                    return (value + 5) == 10
                })

                let matcher2 = Test.newMatcher(fun (_ value: String): Bool {
                    return value.length == 10
                })

                let matcher3 = matcher1.and(matcher2)

                // Invoke with a type that matches to only one matcher
                Test.expect(5, matcher3)
            }
		`

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)

		var typeMismatchErr *interpreter.TypeMismatchError
		assert.ErrorAs(t, result.Error, &typeMismatchErr)
	})
}

func TestInterpretEqualMatcher(t *testing.T) {

	t.Parallel()

	t.Run("equal matcher with primitive", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let matcher = Test.equal(1)
                Test.expect(1, matcher)
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

            access(all) fun test() {
                let f = Foo()
                let matcher = Test.equal(f)
                Test.expect(f, matcher)
            }

            access(all) struct Foo {}
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

            access(all) fun test() {
                let f <- create Foo()
                let matcher = Test.equal(<-f)
                Test.expect(<- create Foo(), matcher)
            }

            access(all) resource Foo {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("with explicit types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
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

            access(all) fun test() {
                let matcher = Test.equal<String>(1)
            }
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("matcher or", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneOrTwo = one.or(two)

                Test.expect(1, oneOrTwo)
                Test.expect(2, oneOrTwo)
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

            access(all) fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneOrTwo = one.or(two)

                Test.expect(3, oneOrTwo)
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

            access(all) fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)

                let oneAndTwo = one.and(two)

                Test.expect(1, oneAndTwo)
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

            access(all) fun test() {
                let one = Test.equal(1)
                let two = Test.equal(2)
                let three = Test.equal(3)

                let oneOrTwoOrThree = one.or(two).or(three)

                Test.expect(3, oneOrTwoOrThree)
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

            access(all) fun test() {
                let foo <- create Foo()
                let bar <- create Bar()

                let fooMatcher = Test.equal(<-foo)
                let barMatcher = Test.equal(<-bar)

                let matcher = fooMatcher.or(barMatcher)

                Test.expect(<-create Foo(), matcher)
                Test.expect(<-create Bar(), matcher)
            }

            access(all) resource Foo {}
            access(all) resource Bar {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 4)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[2])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[3])
	})

	t.Run("resource matcher and", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let foo <- create Foo()
                let bar <- create Bar()

                let fooMatcher = Test.equal(<-foo)
                let barMatcher = Test.equal(<-bar)

                let matcher = fooMatcher.and(barMatcher)

                Test.expect(<-create Foo(), matcher)
            }

            access(all) resource Foo {}
            access(all) resource Bar {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 3)
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

            access(all) fun test() {
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

            access(all) fun test() {
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

            access(all) fun test() {
                Test.expect("string", Test.equal(1))
            }
		`

		runner := NewTestRunner()
		result, err := runner.RunTest(script, "test")
		require.NoError(t, err)

		require.Error(t, result.Error)

		var typeMismatchErr *interpreter.TypeMismatchError
		assert.ErrorAs(t, result.Error, &typeMismatchErr)
	})

	t.Run("with explicit types", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
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

            access(all) fun test() {
                Test.expect<Int>("string", Test.equal(1))
            }
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("resource with resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let f1 <- create Foo()
                let f2 <- create Foo()
                Test.expect(<-f1, Test.equal(<-f2))
            }

            access(all) resource Foo {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
		assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
	})

	t.Run("resource with struct matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let foo <- create Foo()
                let bar = Bar()
                Test.expect(<-foo, Test.equal(bar))
            }

            access(all) resource Foo {}
            access(all) struct Bar {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})

	t.Run("struct with resource matcher", func(t *testing.T) {
		t.Parallel()

		const script = `
            import Test

            access(all) fun test() {
                let foo = Foo()
                let bar <- create Bar()
                Test.expect(foo, Test.equal(<-bar))
            }

            access(all) struct Foo {}
            access(all) resource Bar {}
		`

		runner := NewTestRunner()
		_, err := runner.RunTest(script, "test")
		require.Error(t, err)

		errs := RequireCheckerErrors(t, err, 1)
		assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	})
}

func TestReplacingImports(t *testing.T) {
	t.Parallel()

	t.Run("file location", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            let account = Test.getAccount(0x0000000000000006)

            access(all)
            fun setup() {
                // Deploy the contract
                let err = Test.deployContract(
                    name: "Foo",
                    path: "./sample/contract.cdc",
                    arguments: [],
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                let script = Test.readFile("./sample/script.cdc")
                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())
                Test.assertEqual("hello from Foo", result.returnValue! as! String)
            }
		`

		const contractCode = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		const scriptCode = `
            import Foo from "./FooContract"

            access(all) fun main(): String {
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
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("address location", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            let account = Test.getAccount(0x0000000000000006)

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "Foo",
                    path: "./sample/contract.cdc",
                    arguments: [],
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                let script = Test.readFile("./sample/script.cdc")
                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beFailed())
                if result.status == Test.ResultStatus.failed {
                    panic(result.error!.message)
                }
                Test.assertEqual("hello from Foo", result.returnValue! as! String)
            }
		`

		const contractCode = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		const scriptCode = `
            import Foo from 0x01

            access(all) fun main(): String {
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
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)

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

            access(all)
            let account = Test.createAccount()

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "Foo",
                    path: "./sample/contract.cdc",
                    arguments: [],
                )

                Test.expect(err, Test.beNil())

                // Configurations are not provided.
            }

            access(all)
            fun test() {
                let script = Test.readFile("./sample/script.cdc")
                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beSucceeded())
                if result.status == Test.ResultStatus.failed {
                    panic(result.error!.message)
                }
                Test.assertEqual("hello from Foo", result.returnValue! as! String)
            }
		`

		const contractCode = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		const scriptCode = `
            import Foo from "./FooContract"

            access(all) fun main(): String {
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
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("config with missing imports", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test

            access(all)
            let account = Test.getAccount(0x0000000000000006)

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "Foo",
                    path: "./FooContract",
                    arguments: [],
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                let script = Test.readFile("./sample/script.cdc")
                let result = Test.executeScript(script, [])

                Test.expect(result, Test.beFailed())
                if result.status == Test.ResultStatus.failed {
                    panic(result.error!.message)
                }
                Test.assertEqual("hello from Foo", result.returnValue! as! String)
            }
		`

		const scriptCode = `
            import Foo from "./FooContract"
            import Bar from "./BarContract"  // This is missing in configs

            access(all)
            fun main(): String {
                return Foo.sayHello()
            }
		`

		const contractCode = `
            access(all)
            contract Foo {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./sample/script.cdc":
				return scriptCode, nil
			case "./FooContract":
				return contractCode, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		contracts := map[string]common.Address{
			"Foo": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithContracts(contracts)

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

	emulatorBackend := NewEmulatorBackend(zerolog.Nop(), nil, nil)
	emulatorBackend.contracts = map[string]common.Address{
		"C1": {0, 0, 0, 0, 0, 0, 0, 1},
		"C2": {0, 0, 0, 0, 0, 0, 0, 2},
		"C3": {0, 0, 0, 0, 0, 0, 0, 1},
	}

	const code = `
        import C1 from "./sample/contract1.cdc"
        import C2 from "C2"
        import "C3"
        import C4 from 0x0000000000000009

        access(all)
        fun main() {}
	`

	const expected = `
        import C1 from 0x0000000000000001
        import C2 from 0x0000000000000002
        import C3 from 0x0000000000000001
        import C4 from 0x0000000000000009

        access(all)
        fun main() {}
	`

	replacedCode := emulatorBackend.replaceImports(code)

	assert.Equal(t, expected, replacedCode)
}

func TestGetAccountFlowBalance(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import BlockchainHelpers

        access(all)
        fun testGetFlowBalance() {
            // Arrange
            let account = Test.serviceAccount()

            // Act
            let balance = getFlowBalance(account: account)

            // Assert
            Test.assertEqual(1000000000.0, balance)
        }
	`

	runner := NewTestRunner()

	result, err := runner.RunTest(testCode, "testGetFlowBalance")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestGetCurrentBlockHeight(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import BlockchainHelpers

        access(all)
        fun testGetCurrentBlockHeight() {
            // Act
            let height = getCurrentBlockHeight()

            // Assert
            Test.expect(height, Test.beGreaterThan(1 as UInt64))

            // Act
            Test.commitBlock()
            let newHeight = getCurrentBlockHeight()

            // Assert
            Test.assertEqual(newHeight, height + 1)
        }
	`

	runner := NewTestRunner()

	result, err := runner.RunTest(testCode, "testGetCurrentBlockHeight")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestMintFlow(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import BlockchainHelpers

        access(all)
        fun testMintFlow() {
            // Arrange
            let account = Test.createAccount()

            // Act
            mintFlow(to: account, amount: 1500.0)

            // Assert
            let balance = getFlowBalance(account: account)
            Test.assertEqual(1500.0, balance)
        }
	`

	runner := NewTestRunner()

	result, err := runner.RunTest(testCode, "testMintFlow")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestBurnFlow(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import BlockchainHelpers

        access(all)
        fun testBurnFlow() {
            // Arrange
            let account = Test.createAccount()

            // Act
            mintFlow(to: account, amount: 1500.0)

            // Assert
            var balance = getFlowBalance(account: account)
            Test.assertEqual(1500.0, balance)

            // Act
            burnFlow(from: account, amount: 500.0)

            // Assert
            balance = getFlowBalance(account: account)
            Test.assertEqual(1000.0, balance)
        }
	`

	runner := NewTestRunner()

	result, err := runner.RunTest(testCode, "testBurnFlow")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestExecuteScriptHelper(t *testing.T) {
	t.Parallel()

	const code = `
        import Test
        import BlockchainHelpers

        access(all)
        fun test() {
            let scriptResult = executeScript("add_integers.cdc", [])

            Test.expect(scriptResult, Test.beSucceeded())
            Test.assertEqual(5, scriptResult.returnValue! as! Int)
        }
	`

	const script = `
        access(all)
        fun main(): Int {
            return 2 + 3
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "add_integers.cdc":
			return script, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)
	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestExecuteTransactionHelper(t *testing.T) {
	t.Parallel()

	const code = `
        import Test
        import BlockchainHelpers

        access(all)
        fun test() {
            let account = Test.createAccount()
            let txResult = executeTransaction(
                "test_tx.cdc",
                [],
                account
            )

            Test.expect(txResult, Test.beSucceeded())
        }
	`

	const testTransaction = `
        import "NonFungibleToken"
        import "ExampleNFT"
        import "MetadataViews"

        transaction {

            prepare(signer: &Account) {
                log(signer.address)
            }
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "test_tx.cdc":
			return testTransaction, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	runner := NewTestRunner().WithFileResolver(fileResolver)
	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestServiceAccount(t *testing.T) {
	t.Parallel()

	t.Run("retrieve from EmulatorBackend", func(t *testing.T) {
		t.Parallel()

		emulatorBackend := NewEmulatorBackend(zerolog.Nop(), nil, nil)

		serviceAccount, err := emulatorBackend.ServiceAccount()

		require.NoError(t, err)
		assert.Equal(
			t,
			"0x0000000000000001",
			serviceAccount.Address.HexWithPrefix(),
		)
	})

	t.Run("retrieve from Test framework's blockchain", func(t *testing.T) {
		t.Parallel()

		const testCode = `
            import Test

            access(all)
            fun testGetServiceAccount() {
                // Act
                let account = Test.serviceAccount()

                // Assert
                Test.assertEqual(Type<Address>(), account.address.getType())
                Test.assertEqual(Type<PublicKey>(), account.publicKey.getType())
                Test.assertEqual(Address(0x0000000000000001), account.address)
            }
		`

		runner := NewTestRunner()

		result, err := runner.RunTest(testCode, "testGetServiceAccount")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})

	t.Run("retrieve from Test.getAccount() function", func(t *testing.T) {
		t.Parallel()

		const testCode = `
            import Test

            access(all)
            fun testGetServiceAccount() {
                // Act
                let account = Test.getAccount(0x0000000000000001)

                // Assert
                Test.assertEqual(Type<Address>(), account.address.getType())
                Test.assertEqual(Type<PublicKey>(), account.publicKey.getType())
                Test.assertEqual(Address(0x0000000000000001), account.address)
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
            import BlockchainHelpers

            access(all)
            fun testGetServiceAccountBalance() {
                // Arrange
                let account = Test.serviceAccount()

                // Act
                let balance = getFlowBalance(account: account)

                // Assert
                Test.assertEqual(1000000000.0, balance)
            }

            access(all)
            fun testTransferFlowTokens() {
                // Arrange
                let account = Test.serviceAccount()
                let receiver = Test.createAccount()

                let code = Test.readFile("../transactions/transfer_flow_tokens.cdc")
                let tx = Test.Transaction(
                    code: code,
                    authorizers: [account.address],
                    signers: [account],
                    arguments: [receiver.address, 1500.0]
                )

                // Act
                let txResult = Test.executeTransaction(tx)
                Test.expect(txResult, Test.beSucceeded())

                // Assert
                let balance = getFlowBalance(account: receiver)
                Test.assertEqual(1500.0, balance)
            }
		`

		const transactionCode = `
            import "FungibleToken"
            import "FlowToken"

            transaction(receiver: Address, amount: UFix64) {
                prepare(account: auth(BorrowValue) &Account) {
                    let flowVault = account.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(
                        from: /storage/flowTokenVault
                    ) ?? panic("Could not borrow BlpToken.Vault reference")

                    let receiverRef = getAccount(receiver)
                        .capabilities.borrow<&FlowToken.Vault>(/public/flowTokenReceiver)
                        ?? panic("Could not borrow FlowToken.Vault reference")

                    let tokens <- flowVault.withdraw(amount: amount)
                    receiverRef.deposit(from: <- tokens)
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "../transactions/transfer_flow_tokens.cdc":
				return transactionCode, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
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

	t.Run("deploy contract to service account", func(t *testing.T) {
		t.Parallel()

		const code = `
            import Test
            import "FooContract"

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "./FooContract",
                    arguments: []
                )

                Test.expect(err, Test.beNil())
            }

            access(all)
            fun test() {
                Test.assertEqual("hello from Foo", FooContract.sayHello())
            }
		`

		const fooContract = `
            access(all)
            contract FooContract {
                init() {}

                access(all)
                fun sayHello(): String {
                    return "hello from Foo"
                }
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "./FooContract":
				return fooContract, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return fooContract, nil
				}
			case common.StringLocation:
				if location == "FooContract" {
					return fooContract, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": {0, 0, 0, 0, 0, 0, 0, 1},
		}

		runner := NewTestRunner().
			WithImportResolver(importResolver).
			WithFileResolver(fileResolver).
			WithContracts(contracts)

		result, err := runner.RunTest(code, "test")
		require.NoError(t, err)
		require.NoError(t, result.Error)
	})
}

func TestCoverageReportForUnitTests(t *testing.T) {
	t.Parallel()

	const fooContract = `
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {
                    1729: "Harshad",
                    8128: "Harmonic",
                    41041: "Carmichael"
                }
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
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
        import FooContract from "../contracts/FooContract.cdc"

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
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
                Test.assertEqual(result, testInputs[input]!)
            }
        }

        access(all)
        fun testAddSpecialNumber() {
            // Act
            FooContract.addSpecialNumber(78557, "Sierpinski")

            // Assert
            Test.assertEqual("Sierpinski", FooContract.getIntegerTrait(78557))
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return fooContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return fooContract, nil
			}
		case common.StringLocation:
			if location == "../contracts/FooContract.cdc" {
				return fooContract, nil
			}
		}

		return "", fmt.Errorf("cannot find import location: %s", location.ID())
	}

	contracts := map[string]common.Address{
		"FooContract": secondAccountAddress,
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
		WithImportResolver(importResolver).
		WithCoverageReport(coverageReport).
		WithContracts(contracts)

	results, err := runner.RunTests(code)

	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, result := range results {
		assert.NoError(t, result.Error)
	}

	location := common.AddressLocation{
		Address: secondAccountAddress,
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
			"A.0000000000000001.FlowClusterQC",
			"A.0000000000000002.FungibleToken",
			"A.0000000000000002.FungibleTokenMetadataViews",
			"A.0000000000000001.NodeVersionBeacon",
			"A.0000000000000003.FlowToken",
			"A.0000000000000001.FlowEpoch",
			"A.0000000000000001.FlowIDTableStaking",
			"A.0000000000000001.FlowStakingCollection",
			"A.0000000000000001.FlowServiceAccount",
			"A.0000000000000001.FlowStorageFees",
			"A.0000000000000001.LockedTokens",
			"A.0000000000000001.FlowDKG",
			"A.0000000000000004.FlowFees",
			"A.0000000000000001.ExampleNFT",
			"A.0000000000000001.StakingProxy",
			"A.0000000000000001.MetadataViews",
			"A.0000000000000001.NonFungibleToken",
			"A.0000000000000001.ViewResolver",
			"A.0000000000000001.RandomBeaconHistory",
			"A.0000000000000001.EVM",
			"I.Test",
			"I.Crypto",
			"I.BlockchainHelpers",
			"s.7465737400000000000000000000000000000000000000000000000000000000",
			"A.0000000000000002.FungibleTokenSwitchboard",
			"A.0000000000000001.Burner",
			"A.0000000000000001.Crypto",
			"A.0000000000000001.NFTStorefrontV2",
			"A.0000000000000002.USDCFlow",
			"A.0000000000000002.FlowExecutionParameters",
			"A.0000000000000001.Migration",
			"A.0000000000000001.CrossVMMetadataViews",
			"A.0000000000000001.CrossVMNFT",
			"A.0000000000000001.CrossVMToken",
			"A.0000000000000001.FlowEVMBridge",
			"A.0000000000000001.FlowEVMBridgeAccessor",
			"A.0000000000000001.FlowEVMBridgeConfig",
			"A.0000000000000001.FlowEVMBridgeHandlerInterfaces",
			"A.0000000000000001.FlowEVMBridgeHandlers",
			"A.0000000000000001.FlowEVMBridgeNFTEscrow",
			"A.0000000000000001.FlowEVMBridgeResolver",
			"A.0000000000000001.FlowEVMBridgeTemplates",
			"A.0000000000000001.FlowEVMBridgeTokenEscrow",
			"A.0000000000000001.FlowEVMBridgeUtils",
			"A.0000000000000001.IBridgePermissions",
			"A.0000000000000001.ICrossVM",
			"A.0000000000000001.ICrossVMAsset",
			"A.0000000000000001.IEVMBridgeNFTMinter",
			"A.0000000000000001.IEVMBridgeTokenMinter",
			"A.0000000000000001.IFlowEVMNFTBridge",
			"A.0000000000000001.IFlowEVMTokenBridge",
			"A.0000000000000001.FlowTransactionScheduler",
			"A.0000000000000001.FlowTransactionSchedulerUtils",
		},
		coverageReport.ExcludedLocationIDs(),
	)
	assert.Equal(
		t,
		"Coverage: 100.0% of statements",
		coverageReport.String(),
	)
}

func TestCoverageReportForIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {
                    1729: "Harshad",
                    8128: "Harmonic",
                    41041: "Carmichael"
                }
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
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

        access(all) fun main(): Bool {
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

        access(all)
        let account = Test.getAccount(0x0000000000000009)

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
            let script = Test.readFile("../scripts/get_integer_traits.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assert(result.returnValue! as! Bool)
        }

        access(all) fun testAddSpecialNumber() {
            let code = Test.readFile("../transactions/add_special_number.cdc")
            let tx = Test.Transaction(
                code: code,
                authorizers: [account.address],
                signers: [account],
                arguments: [78557, "Sierpinski"]
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())
        }
	`

	const transactionCode = `
        import FooContract from "../contracts/FooContract.cdc"

        transaction(number: Int, trait: String) {
            prepare(acct: &Account) {}

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
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	contracts := map[string]common.Address{
		"FooContract": secondAccountAddress,
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
		WithCoverageReport(coverageReport).
		WithContracts(contracts)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)

	require.Len(t, results, 2)

	result1 := results[0]
	assert.Equal(t, result1.TestName, "testGetIntegerTrait")
	assert.NoError(t, result1.Error)

	result2 := results[1]
	assert.Equal(t, result2.TestName, "testAddSpecialNumber")
	require.NoError(t, result2.Error)

	location := common.AddressLocation{
		Address: secondAccountAddress,
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
			"A.0000000000000003.FlowToken",
			"A.0000000000000002.FungibleToken",
			"A.0000000000000002.FungibleTokenMetadataViews",
			"A.0000000000000004.FlowFees",
			"A.0000000000000001.FlowStorageFees",
			"A.0000000000000001.FlowServiceAccount",
			"A.0000000000000001.FlowClusterQC",
			"A.0000000000000001.FlowDKG",
			"A.0000000000000001.FlowEpoch",
			"A.0000000000000001.FlowIDTableStaking",
			"A.0000000000000001.FlowStakingCollection",
			"A.0000000000000001.LockedTokens",
			"A.0000000000000001.NodeVersionBeacon",
			"A.0000000000000001.StakingProxy",
			"s.7465737400000000000000000000000000000000000000000000000000000000",
			"I.Crypto",
			"I.Test",
			"I.BlockchainHelpers",
			"A.0000000000000001.ExampleNFT",
			"A.0000000000000001.MetadataViews",
			"A.0000000000000001.NonFungibleToken",
			"A.0000000000000001.ViewResolver",
			"A.0000000000000001.RandomBeaconHistory",
			"A.0000000000000001.EVM",
			"A.0000000000000002.FungibleTokenSwitchboard",
			"A.0000000000000001.Burner",
			"A.0000000000000001.NFTStorefrontV2",
			"A.0000000000000001.Crypto",
			"A.0000000000000002.USDCFlow",
			"A.0000000000000002.FlowExecutionParameters",
			"A.0000000000000001.Migration",
			"A.0000000000000001.CrossVMMetadataViews",
			"A.0000000000000001.CrossVMNFT",
			"A.0000000000000001.CrossVMToken",
			"A.0000000000000001.FlowEVMBridge",
			"A.0000000000000001.FlowEVMBridgeAccessor",
			"A.0000000000000001.FlowEVMBridgeConfig",
			"A.0000000000000001.FlowEVMBridgeHandlerInterfaces",
			"A.0000000000000001.FlowEVMBridgeHandlers",
			"A.0000000000000001.FlowEVMBridgeNFTEscrow",
			"A.0000000000000001.FlowEVMBridgeResolver",
			"A.0000000000000001.FlowEVMBridgeTemplates",
			"A.0000000000000001.FlowEVMBridgeTokenEscrow",
			"A.0000000000000001.FlowEVMBridgeUtils",
			"A.0000000000000001.IBridgePermissions",
			"A.0000000000000001.ICrossVM",
			"A.0000000000000001.ICrossVMAsset",
			"A.0000000000000001.IEVMBridgeNFTMinter",
			"A.0000000000000001.IEVMBridgeTokenMinter",
			"A.0000000000000001.IFlowEVMNFTBridge",
			"A.0000000000000001.IFlowEVMTokenBridge",
			"A.0000000000000001.FlowTransactionScheduler",
			"A.0000000000000001.FlowTransactionSchedulerUtils",
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
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {1729: "Harshad"}
                log("init successful")
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
                log("specialNumbers updated")
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Unknown"
            }
        }
	`

	const code = `
        import Test
        import FooContract from "FooContract.cdc"

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())

            log("setup successful")
        }

        access(all)
        fun testGetIntegerTrait() {
            // Act
            let result = FooContract.getIntegerTrait(1729)

            // Assert
            Test.assertEqual("Harshad", result)
            log("getIntegerTrait works")
        }

        access(all)
        fun testAddSpecialNumber() {
            // Act
            FooContract.addSpecialNumber(78557, "Sierpinski")

            // Assert
            Test.assertEqual("Sierpinski", FooContract.getIntegerTrait(78557))
            log("addSpecialNumber works")
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "FooContract.cdc":
			return fooContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return fooContract, nil
			}
		case common.StringLocation:
			if location == "FooContract.cdc" {
				return fooContract, nil
			}
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithFileResolver(fileResolver).
		WithContracts(contracts)

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
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {1729: "Harshad"}
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Unknown"
            }
        }
	`

	const code = `
        import Test
        import FooContract from "FooContract.cdc"

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
            // Act
            let result = FooContract.getIntegerTrait(1729)

            // Assert
            Test.assertEqual("Harshad", result)
        }

        access(all)
        fun testAddSpecialNumber() {
            // Act
            FooContract.addSpecialNumber(78557, "Sierpinski")

            // Assert
            Test.assertEqual("Sierpinski", FooContract.getIntegerTrait(78557))
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "FooContract.cdc":
			return fooContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return fooContract, nil
			}
		case common.StringLocation:
			if location == "FooContract.cdc" {
				return fooContract, nil
			}
		}

		return "", fmt.Errorf("unsupported import %s", location)
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithFileResolver(fileResolver).
		WithContracts(contracts)

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
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {1729: "Harshad"}
                log("init successful")
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
                log("specialNumbers updated")
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Unknown"
            }
        }
	`

	const scriptCode = `
        import FooContract from "../contracts/FooContract.cdc"

        access(all) fun main(): Bool {
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

        access(all)
        let account = Test.getAccount(0x0000000000000006)

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
            let script = Test.readFile("../scripts/get_integer_traits.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assert(result.returnValue! as! Bool)
        }

        access(all) fun testAddSpecialNumber() {
            let code = Test.readFile("../transactions/add_special_number.cdc")
            let tx = Test.Transaction(
                code: code,
                authorizers: [account.address],
                signers: [account],
                arguments: [78557, "Sierpinski"]
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())
        }

        access(all) fun tearDown() {
            let expectedLogs = [
                "init successful",
                "getIntegerTrait works",
                "specialNumbers updated",
                "addSpecialNumber works"
            ]
            Test.assertEqual(expectedLogs, Test.logs())
        }
	`

	const transactionCode = `
        import FooContract from "../contracts/FooContract.cdc"

        transaction(number: Int, trait: String) {
            prepare(acct: &Account) {}

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
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContracts(contracts)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestRetrieveEmptyLogsFromIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {1729: "Harshad"}
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Unknown"
            }
        }
	`

	const scriptCode = `
        import FooContract from "../contracts/FooContract.cdc"

        access(all) fun main(): Bool {
            // Act
            let trait = FooContract.getIntegerTrait(1729)

            // Assert
            assert(trait == "Harshad")

            return true
        }
	`

	const testCode = `
        import Test

        access(all)
        let account = Test.getAccount(0x0000000000000006)

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
            let script = Test.readFile("../scripts/get_integer_traits.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assert(result.returnValue! as! Bool)
        }

        access(all) fun testAddSpecialNumber() {
            let code = Test.readFile("../transactions/add_special_number.cdc")
            let tx = Test.Transaction(
                code: code,
                authorizers: [account.address],
                signers: [account],
                arguments: [78557, "Sierpinski"]
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())
        }

        access(all)
        fun tearDown() {
            Test.assertEqual([] as [String], Test.logs() )
        }
	`

	const transactionCode = `
        import FooContract from "../contracts/FooContract.cdc"

        transaction(number: Int, trait: String) {
            prepare(acct: &Account) {}

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
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithContracts(contracts)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestWithLogger(t *testing.T) {
	t.Parallel()

	const code = `
        access(all)
        fun testWithLogger() {
            log("Hello, world!")
        }
	`

	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	runner := NewTestRunner().WithLogger(logger)

	result, err := runner.RunTest(code, "testWithLogger")
	require.NoError(t, err)
	require.NoError(t, result.Error)

	assert.Contains(t, buf.String(), "Hello, world!")
	assert.Equal(t, []string{"Hello, world!"}, runner.Logs())
}

func TestGetEventsFromIntegrationTests(t *testing.T) {
	t.Parallel()

	const contractCode = `
        access(all) contract FooContract {
            access(all) let specialNumbers: {Int: String}

            access(all) event ContractInitialized()
            access(all) event NumberAdded(n: Int, trait: String)

            init() {
                self.specialNumbers = {1729: "Harshad"}
                emit ContractInitialized()
            }

            access(all) fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
                emit NumberAdded(n: n, trait: trait)
            }

            access(all) fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Unknown"
            }
        }
	`

	const scriptCode = `
        import FooContract from "../contracts/FooContract.cdc"

        access(all) fun main(): Bool {
            // Act
            let trait = FooContract.getIntegerTrait(1729)

            // Assert
            assert(trait == "Harshad")
            return true
        }
	`

	const testCode = `
        import Test
        import FooContract from "../contracts/FooContract.cdc"

        access(all)
        let account = Test.getAccount(0x0000000000000006)

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testGetIntegerTrait() {
            let script = Test.readFile("../scripts/get_integer_traits.cdc")
            let result = Test.executeScript(script, [])

            Test.expect(result, Test.beSucceeded())
            Test.assert(result.returnValue! as! Bool)

            let typ = Type<FooContract.ContractInitialized>()
            let events = Test.eventsOfType(typ)
            Test.assertEqual(1, events.length)
        }

        access(all) fun testAddSpecialNumber() {
            let code = Test.readFile("../transactions/add_special_number.cdc")
            let tx = Test.Transaction(
                code: code,
                authorizers: [account.address],
                signers: [account],
                arguments: [78557, "Sierpinski"]
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())

            let typ = Type<FooContract.NumberAdded>()
            let events = Test.eventsOfType(typ)
            Test.assertEqual(1, events.length)

            let numberAddedEvent = events[0] as! FooContract.NumberAdded
            Test.assertEqual(78557, numberAddedEvent.n)
            Test.assertEqual("Sierpinski", numberAddedEvent.trait)

            let evts = Test.events()
            Test.expect(evts.length, Test.beGreaterThan(1))
        }
	`

	const transactionCode = `
        import FooContract from "../contracts/FooContract.cdc"

        transaction(number: Int, trait: String) {
            prepare(acct: &Account) {}

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
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return contractCode, nil
			}
		case common.StringLocation:
			if location == "../contracts/FooContract.cdc" {
				return contractCode, nil
			}
		}

		return "", fmt.Errorf("cannot find import location: %s", location.ID())
	}

	contracts := map[string]common.Address{
		"FooContract": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithImportResolver(importResolver).
		WithContracts(contracts)

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

        access(all) fun createTransaction(
            _ path: String,
            account: Test.TestAccount,
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
            prepare(acct: &Account) {}

            execute {
                assert(true)
            }
        }
	`

	const testCode = `
        import Test
        import "test_helpers.cdc"

        access(all)
        let account = Test.createAccount()

        access(all)
        fun testRunTransaction() {
            let tx = createTransaction(
                "../transactions/add_special_number.cdc",
                account: account,
                args: []
            )

            let result = Test.executeTransaction(tx)
            Test.expect(result, Test.beSucceeded())
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../transactions/add_special_number.cdc":
			return transactionCode, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
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
        import BlockchainHelpers

        access(all)
        fun testBlockchainReset() {
            // Arrange
            let account = Test.createAccount()
            var balance = getFlowBalance(account: account)
            Test.assertEqual(0.0, balance)

            let height = getCurrentBlockHeight()

            mintFlow(to: account, amount: 1500.0)

            balance = getFlowBalance(account: account)
            Test.assertEqual(1500.0, balance)
            Test.assertEqual(getCurrentBlockHeight(), height + 1)

            // Act
            Test.reset(to: height)

            // Assert
            balance = getFlowBalance(account: account)
            Test.assertEqual(0.0, balance)
            Test.assertEqual(getCurrentBlockHeight(), height)
        }
	`

	runner := NewTestRunner()

	result, err := runner.RunTest(testCode, "testBlockchainReset")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestTestFunctionValidSignature(t *testing.T) {
	t.Parallel()

	t.Run("with argument", func(t *testing.T) {
		t.Parallel()

		const testCode = `
            import Test

            access(all) fun testValidSignature() {
                Test.assertEqual(2, 5 - 3)
            }

            access(all) fun testInvalidSignature(prefix: String) {
                Test.assertEqual(2, 5 - 3)
            }
		`

		runner := NewTestRunner()

		_, err := runner.RunTests(testCode)
		require.Error(t, err)
		assert.ErrorContains(t, err, "test functions should have no arguments")
	})

	t.Run("with return value", func(t *testing.T) {
		t.Parallel()

		const testCode = `
            import Test

            access(all) fun testValidSignature() {
                Test.assertEqual(2, 5 - 3)
            }

            access(all) fun testInvalidSignature(): Bool {
                return 2 == (5 - 3)
            }
		`

		runner := NewTestRunner()

		_, err := runner.RunTests(testCode)
		require.Error(t, err)
		assert.ErrorContains(t, err, "test functions should have no return values")
	})
}

func TestBlockchainMoveTime(t *testing.T) {
	t.Parallel()

	const contractCode = `
        access(all) contract TimeLocker {
            access(all) let lockPeriod: UFix64
            access(all) let lockedAt: UFix64

            init(lockedAt: UFix64) {
                self.lockedAt = lockedAt
                // Lock period is 30 days, in the form of seconds.
                self.lockPeriod = UFix64(30 * 24 * 60 * 60)
            }

            access(all) fun isOpen(): Bool {
                let currentTime = getCurrentBlock().timestamp
                return currentTime > (self.lockedAt + self.lockPeriod)
            }
        }
	`

	const scriptCode = `
        import "TimeLocker"

        access(all) fun main(): Bool {
            return TimeLocker.isOpen()
        }
	`

	const currentBlockTimestamp = `
        access(all) fun main(): UFix64 {
            return getCurrentBlock().timestamp
        }
	`

	const testCode = `
        import Test

        access(all)
        let account = Test.getAccount(0x0000000000000006)
        access(all)
        var lockedAt: UFix64 = 0.0

        access(all)
        fun setup() {
            let currentBlockTimestamp = Test.readFile("current_block_timestamp.cdc")
            let result = Test.executeScript(currentBlockTimestamp, [])
            lockedAt = result.returnValue! as! UFix64

            let err = Test.deployContract(
                name: "TimeLocker",
                path: "TimeLocker.cdc",
                arguments: [lockedAt]
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testIsNotOpen() {
            let isLockerOpen = Test.readFile("is_locker_open.cdc")
            let result = Test.executeScript(isLockerOpen, [])

            Test.expect(result, Test.beSucceeded())
            Test.assertEqual(false, result.returnValue! as! Bool)
        }

        access(all)
        fun testIsOpen() {
            // timeDelta is the representation of 20 days, in seconds
            let timeDelta = Fix64(20 * 24 * 60 * 60)
            Test.moveTime(by: timeDelta)

            let isLockerOpen = Test.readFile("is_locker_open.cdc")
            var result = Test.executeScript(isLockerOpen, [])

            Test.expect(result, Test.beSucceeded())
            Test.assertEqual(false, result.returnValue! as! Bool)

            // We move time forward by another 20 days
            Test.moveTime(by: timeDelta)

            result = Test.executeScript(isLockerOpen, [])

            Test.assertEqual(true, result.returnValue! as! Bool)

            // We move time backward by 20 days
            Test.moveTime(by: timeDelta * -1.0)

            result = Test.executeScript(isLockerOpen, [])

            Test.assertEqual(false, result.returnValue! as! Bool)
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "TimeLocker.cdc":
			return contractCode, nil
		case "is_locker_open.cdc":
			return scriptCode, nil
		case "current_block_timestamp.cdc":
			return currentBlockTimestamp, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "TimeLocker" {
				return contractCode, nil
			}
		case common.StringLocation:
			if location == "TimeLocker.cdc" {
				return contractCode, nil
			}
		}

		return "", fmt.Errorf("cannot find import location: %s", location.ID())
	}

	contracts := map[string]common.Address{
		"TimeLocker": firstAccountAddress,
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithImportResolver(importResolver).
		WithContracts(contracts)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}

func TestRandomizedTestExecution(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        access(all) fun testCase1() {
            log("testCase1")
        }

        access(all) fun testCase2() {
            log("testCase2")
        }

        access(all) fun testCase3() {
            log("testCase3")
        }

        access(all) fun testCase4() {
            log("testCase4")
        }
	`

	runner := NewTestRunner().WithRandomSeed(1600)
	results, err := runner.RunTests(code)
	require.NoError(t, err)

	resultsStr := PrettyPrintResults(results, "test_script.cdc")

	const expected = `Test results: "test_script.cdc"
- PASS: testCase4
- PASS: testCase3
- PASS: testCase1
- PASS: testCase2
`

	assert.Equal(t, expected, resultsStr)
}

func TestReferenceDeployedContractTypes(t *testing.T) {
	t.Parallel()

	t.Run("without init params", func(t *testing.T) {
		t.Parallel()

		const contractCode = `
            access(all) contract FooContract {
                access(all) let specialNumbers: {Int: String}

                access(all) struct SpecialNumber {
                    access(all) let n: Int
                    access(all) let trait: String
                    access(all) let numbers: {Int: String}

                    init(n: Int, trait: String) {
                        self.n = n
                        self.trait = trait
                        self.numbers = FooContract.specialNumbers
                    }

                    access(all) fun getAllNumbers(): {Int: String} {
                        return FooContract.specialNumbers
                    }
                }

                init() {
                    self.specialNumbers = {1729: "Harshad"}
                }

                access(all) fun getSpecialNumbers(): [SpecialNumber] {
                    return [
                        SpecialNumber(n: 1729, trait: "Harshad")
                    ]
                }
            }
		`

		const scriptCode = `
            import FooContract from "../contracts/FooContract.cdc"

            access(all) fun main(): [FooContract.SpecialNumber] {
                return FooContract.getSpecialNumbers()
            }
		`

		const testCode = `
            import Test
            import FooContract from "../contracts/FooContract.cdc"

            access(all)
            let account = Test.getAccount(0x0000000000000006)

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "../contracts/FooContract.cdc",
                    arguments: []
                )
                Test.expect(err, Test.beNil())
            }

            access(all)
            fun testGetSpecialNumber() {
                let script = Test.readFile("../scripts/get_special_number.cdc")
                let result = Test.executeScript(script, [])
                Test.expect(result, Test.beSucceeded())

                let specialNumbers = result.returnValue! as! [FooContract.SpecialNumber]
                let specialNumber = specialNumbers[0]
                let expected = FooContract.SpecialNumber(n: 1729, trait: "Harshad")
                Test.assertEqual(expected, specialNumber)

                specialNumber.getAllNumbers()
                Test.assertEqual(1729, specialNumber.n)
                Test.assertEqual("Harshad", specialNumber.trait)
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "../contracts/FooContract.cdc":
				return contractCode, nil
			case "../scripts/get_special_number.cdc":
				return scriptCode, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return contractCode, nil
				}
			case common.StringLocation:
				if location == "../contracts/FooContract.cdc" {
					return contractCode, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithImportResolver(importResolver).
			WithContracts(contracts)

		results, err := runner.RunTests(testCode)
		require.NoError(t, err)
		for _, result := range results {
			require.NoError(t, result.Error)
		}
	})

	t.Run("with init params", func(t *testing.T) {
		t.Parallel()

		const contractCode = `
            access(all) contract FooContract {
                access(all) let specialNumbers: {Int: String}

                access(all) struct SpecialNumber {
                    access(all) let n: Int
                    access(all) let trait: String
                    access(all) let numbers: {Int: String}

                    init(n: Int, trait: String) {
                        self.n = n
                        self.trait = trait
                        self.numbers = FooContract.specialNumbers
                    }

                    access(all) fun getAllNumbers(): {Int: String} {
                        return FooContract.specialNumbers
                    }
                }

                init(specialNumbers: {Int: String}) {
                    self.specialNumbers = specialNumbers
                }

                access(all) fun getSpecialNumbers(): [SpecialNumber] {
                    let specialNumbers: [SpecialNumber] = []

                    self.specialNumbers.forEachKey(fun (key: Int): Bool {
                        let trait = self.specialNumbers[key]!
                        specialNumbers.append(
                            SpecialNumber(n: key, trait: trait)
                        )

                        return true
                    })

                    return specialNumbers
                }
            }
		`

		const scriptCode = `
            import FooContract from "../contracts/FooContract.cdc"

            access(all) fun main(): [FooContract.SpecialNumber] {
                return FooContract.getSpecialNumbers()
            }
		`

		const testCode = `
            import Test
            import FooContract from "../contracts/FooContract.cdc"

            access(all)
            let account = Test.getAccount(0x0000000000000006)

            access(all)
            fun setup() {
                let err = Test.deployContract(
                    name: "FooContract",
                    path: "../contracts/FooContract.cdc",
                    arguments: [{1729: "Harshad"}]
                )
                Test.expect(err, Test.beNil())
            }

            access(all)
            fun testGetSpecialNumber() {
                let script = Test.readFile("../scripts/get_special_number.cdc")
                let result = Test.executeScript(script, [])
                Test.expect(result, Test.beSucceeded())

                let specialNumbers = result.returnValue! as! [FooContract.SpecialNumber]
                let specialNumber = specialNumbers[0]
                let expected = FooContract.SpecialNumber(n: 1729, trait: "Harshad")
                Test.assertEqual(expected, specialNumber)

                specialNumber.getAllNumbers()
                Test.assertEqual(1729, specialNumber.n)
                Test.assertEqual("Harshad", specialNumber.trait)
            }
		`

		fileResolver := func(path string) (string, error) {
			switch path {
			case "../contracts/FooContract.cdc":
				return contractCode, nil
			case "../scripts/get_special_number.cdc":
				return scriptCode, nil
			default:
				return "", fmt.Errorf("cannot find file path: %s", path)
			}
		}

		importResolver := func(location common.Location) (string, error) {
			switch location := location.(type) {
			case common.AddressLocation:
				if location.Name == "FooContract" {
					return contractCode, nil
				}
			case common.StringLocation:
				if location == "../contracts/FooContract.cdc" {
					return contractCode, nil
				}
			}

			return "", fmt.Errorf("cannot find import location: %s", location.ID())
		}

		contracts := map[string]common.Address{
			"FooContract": firstAccountAddress,
		}

		runner := NewTestRunner().
			WithFileResolver(fileResolver).
			WithImportResolver(importResolver).
			WithContracts(contracts)

		results, err := runner.RunTests(testCode)
		require.NoError(t, err)
		for _, result := range results {
			require.NoError(t, result.Error)
		}
	})
}

func TestEmulatorBlockchainSnapshotting(t *testing.T) {
	t.Parallel()

	const code = `
        import Test
        import BlockchainHelpers

        access(all)
        fun test() {
            let admin = Test.createAccount()
            Test.createSnapshot(name: "adminCreated")

            mintFlow(to: admin, amount: 1000.0)
            Test.createSnapshot(name: "adminFunded")

            var balance = getFlowBalance(account: admin)
            Test.assertEqual(1000.0, balance)

            Test.loadSnapshot(name: "adminCreated")

            balance = getFlowBalance(account: admin)
            Test.assertEqual(0.0, balance)

            Test.loadSnapshot(name: "adminFunded")

            balance = getFlowBalance(account: admin)
            Test.assertEqual(1000.0, balance)
        }
	`

	runner := NewTestRunner()
	result, err := runner.RunTest(code, "test")
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func TestEnvironmentForUnitTests(t *testing.T) {
	t.Parallel()

	const fooContract = `
        access(all)
        contract FooContract {
            access(all)
            let specialNumbers: {Int: String}

            init() {
                self.specialNumbers = {
                    1729: "Harshad",
                    8128: "Harmonic",
                    41041: "Carmichael"
                }
                self.account.storage.save(self.specialNumbers, to: /storage/specialNumbers)
            }

            access(all)
            fun getSpecialNumbers(): {Int: String} {
                return self.account.storage.load<{Int: String}>(from: /storage/specialNumbers)!
            }

            access(all)
            fun addSpecialNumber(_ n: Int, _ trait: String) {
                self.specialNumbers[n] = trait
                self.account.storage.load<{Int: String}>(from: /storage/specialNumbers)!
                self.account.storage.save(self.specialNumbers, to: /storage/specialNumbers)
            }

            access(all)
            fun getIntegerTrait(_ n: Int): String {
                if self.specialNumbers.containsKey(n) {
                    return self.specialNumbers[n]!
                }

                return "Enormous"
            }

            access(all)
            fun getBlockHeight(): UInt64 {
                return getCurrentBlock().height
            }
        }
	`

	const code = `
        import Test
        import BlockchainHelpers
        import FooContract from "../contracts/FooContract.cdc"

        access(all)
        fun setup() {
            let err = Test.deployContract(
                name: "FooContract",
                path: "../contracts/FooContract.cdc",
                arguments: []
            )

            Test.expect(err, Test.beNil())
        }

        access(all)
        fun testAddSpecialNumber() {
            // Act
            FooContract.addSpecialNumber(78557, "Sierpinski")

            // Assert
            Test.assertEqual("Sierpinski", FooContract.getIntegerTrait(78557))

            let specialNumbers = FooContract.getSpecialNumbers()
            let expected: {Int: String} = {
                8128: "Harmonic",
                1729: "Harshad",
                41041: "Carmichael",
                78557: "Sierpinski"
            }
            Test.assertEqual(expected, specialNumbers)
        }

        access(all)
        fun testGetCurrentBlockHeight() {
            // Act
            let height = FooContract.getBlockHeight()

            // Assert
            Test.expect(height, Test.beGreaterThan(UInt64(1)))
        }
	`

	fileResolver := func(path string) (string, error) {
		switch path {
		case "../contracts/FooContract.cdc":
			return fooContract, nil
		default:
			return "", fmt.Errorf("cannot find file path: %s", path)
		}
	}

	importResolver := func(location common.Location) (string, error) {
		switch location := location.(type) {
		case common.AddressLocation:
			if location.Name == "FooContract" {
				return fooContract, nil
			}
		case common.StringLocation:
			if location == "../contracts/FooContract.cdc" {
				return fooContract, nil
			}
		}

		return "", fmt.Errorf("cannot find import location: %s", location.ID())
	}

	contracts := map[string]common.Address{
		"FooContract": thirdAccountAddress,
	}

	runner := NewTestRunner().
		WithFileResolver(fileResolver).
		WithImportResolver(importResolver).
		WithContracts(contracts)

	results, err := runner.RunTests(code)

	require.NoError(t, err)
	for _, result := range results {
		assert.NoError(t, result.Error)
	}
}
func TestGetTests(t *testing.T) {
	t.Parallel()

	const code = `
        import Test

        access(all)
        fun test1() {
            log("test1")
        }

        access(all)
        fun test2() {
            log("test2")
        }

        access(all)
        fun test3() {
            log("test3")
        }
	`

	runner := NewTestRunner()
	tests, err := runner.GetTests(code)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"test1", "test2", "test3"}, tests)
}

func TestEVMContract(t *testing.T) {
	t.Parallel()

	const testCode = `
        import Test
        import "EVM"

        access(all)
        fun testCOACreation() {
            let coa <- EVM.createCadenceOwnedAccount()
            Test.assertEqual(UInt(0), coa.address().balance().inAttoFLOW())

            destroy <- coa
        }
	`

	importResolver := func(location common.Location) (string, error) {
		return "", fmt.Errorf("cannot find import location: %s", location)
	}

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	log := zerolog.New(output).With().Timestamp().Logger()
	runner := NewTestRunner().
		WithImportResolver(importResolver).
		WithLogger(log)

	results, err := runner.RunTests(testCode)
	require.NoError(t, err)
	for _, result := range results {
		require.NoError(t, result.Error)
	}
}
