/*
 * Cadence languageserver - The Cadence language server
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

package integration

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/parser"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/flow-go-sdk"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func Test_FileImport(t *testing.T) {
	mockFS := afero.NewMemMapFs()
	af := afero.Afero{Fs: mockFS}
	_ = afero.WriteFile(mockFS, "./test.cdc", []byte(`hello test`), 0644)

	resolver := resolvers{
		loader: af,
	}

	t.Run("existing file", func(t *testing.T) {
		t.Parallel()
		resolved, err := resolver.stringImport("", common.StringLocation("./test.cdc"))
		assert.NoError(t, err)
		assert.Equal(t, "hello test", resolved)
	})

	t.Run("non existing file", func(t *testing.T) {
		t.Parallel()
		resolved, err := resolver.stringImport("", common.StringLocation("./foo.cdc"))
		// Accept either relative or absolute error message depending on OS/CWD
		if assert.Error(t, err) {
			msg := err.Error()
			assert.Contains(t, msg, "file does not exist")
			assert.Contains(t, msg, "foo.cdc")
		}
		assert.Equal(t, "", resolved)
	})
}

func Test_AddressImport(t *testing.T) {
	mock := &mockFlowClient{}
	resolver := resolvers{}
	// Seed cfgManager with a default client under a project flow.json
	mem := afero.NewMemMapFs()
	af := afero.Afero{Fs: mem}
	cm := NewConfigManager(af, true, 0, "")
	projectCfg := "/proj/flow.json"
	// Ensure directory exists in memfs
	_ = af.MkdirAll("/proj", 0755)
	_ = af.WriteFile(projectCfg, []byte("{}"), 0644)
	cm.SetDefaultClientForPath(projectCfg, mock)
	resolver.cfgManager = cm
	resolver.loader = af
	projID := projectCfg

	a, _ := common.HexToAddress("1")
	address := common.NewAddressLocation(nil, a, "test")
	flowAddress := flow.HexToAddress(a.String())

	mock.
		On("GetAccount", flowAddress).
		Return(&flow.Account{
			Address: flowAddress,
			Contracts: map[string][]byte{
				"test": []byte("hello tests"),
				"foo":  []byte("foo bar"),
			},
		}, nil)

	nonExisting := flow.HexToAddress("2")
	mock.
		On("GetAccount", nonExisting).
		Return(nil, fmt.Errorf("failed to get account with address %s", nonExisting.String()))

	t.Run("existing address", func(t *testing.T) {
		resolved, err := resolver.addressImport(projID, address)
		assert.NoError(t, err)
		assert.Equal(t, "hello tests", resolved)
	})

	t.Run("non existing contract import", func(t *testing.T) {
		address.Name = "invalid"
		resolved, err := resolver.addressImport(projID, address)
		assert.NoError(t, err)
		assert.Empty(t, resolved)
	})

	t.Run("non existing address", func(t *testing.T) {
		address.Address, _ = common.HexToAddress("2")
		resolved, err := resolver.addressImport(projID, address)
		assert.EqualError(t, err, "failed to get account with address 0000000000000002")
		assert.Empty(t, resolved)
	})

	t.Run("address contract names", func(t *testing.T) {
		contracts, err := resolver.addressContractNames(a)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"foo", "test"}, contracts)
	})
}

func Test_SimpleImport(t *testing.T) {
	mem := afero.NewMemMapFs()
	af := afero.Afero{Fs: mem}
	code := `access(all) contract Test {}`
	// project structure
	_ = af.MkdirAll("/p", 0755)
	_ = afero.WriteFile(mem, "/p/test.cdc", []byte(code), 0644)
	flowJSON := []byte(`{ "contracts": { "Test": "./test.cdc" } }`)
	_ = afero.WriteFile(mem, "/p/flow.json", flowJSON, 0644)

	cm := NewConfigManager(af, false, 0, "/p/flow.json")
	resolver := resolvers{loader: af, cfgManager: cm}

	t.Run("existing import", func(t *testing.T) {
		resolved, err := resolver.stringImport("/p/flow.json", common.StringLocation("Test"))
		assert.NoError(t, err)
		assert.Equal(t, code, resolved)
	})

	t.Run("non existing import", func(t *testing.T) {
		resolved, err := resolver.stringImport("/p/flow.json", common.StringLocation("Foo"))
		assert.Error(t, err)
		assert.Empty(t, resolved)
	})
}

func createTestChecker(t *testing.T, location common.Location) *sema.Checker {
	program, err := parser.ParseProgram(
		nil,
		[]byte(`access(all) contract Test {}`),
		parser.Config{},
	)
	assert.NoError(t, err)

	checker, err := sema.NewChecker(
		program,
		location,
		nil,
		&sema.Config{
			AccessCheckMode: sema.AccessCheckModeStrict,
		},
	)
	assert.NoError(t, err)
	return checker
}

func Test_AccountAccess(t *testing.T) {
	t.Run("both AddressLocation same address", func(t *testing.T) {
		t.Parallel()
		resolver := resolvers{}
		addr, err := common.HexToAddress("01")
		assert.NoError(t, err)

		checker := createTestChecker(t, common.AddressLocation{Address: addr, Name: "ContractA"})
		memberLoc := common.AddressLocation{Address: addr, Name: "ContractB"}

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.True(t, result, "contracts at same address should have account access")
	})

	t.Run("both AddressLocation different address", func(t *testing.T) {
		t.Parallel()
		resolver := resolvers{}
		addr1, err := common.HexToAddress("01")
		assert.NoError(t, err)
		addr2, err := common.HexToAddress("02")
		assert.NoError(t, err)

		checker := createTestChecker(t, common.AddressLocation{Address: addr1, Name: "ContractA"})
		memberLoc := common.AddressLocation{Address: addr2, Name: "ContractB"}

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.False(t, result, "contracts at different addresses should not have account access")
	})

	t.Run("both StringLocation same account", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		// Setup project with two contracts deployed to same account
		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		flowJSON := []byte(`{
			"contracts": {
				"ContractA": "./ContractA.cdc",
				"ContractB": "./ContractB.cdc"
			},
			"deployments": {
				"emulator": {
					"emulator-account": ["ContractA", "ContractB"]
				}
			},
		"accounts": {
			"emulator-account": {
				"address": "0xf8d6e0586b0a20c7",
				"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
			}
		},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.True(t, result, "contracts deployed to same account should have account access")
	})

	t.Run("both StringLocation different accounts", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		// Setup project with two contracts deployed to different accounts
		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		flowJSON := []byte(`{
			"contracts": {
				"ContractA": "./ContractA.cdc",
				"ContractB": "./ContractB.cdc"
			},
			"deployments": {
				"emulator": {
					"account-1": ["ContractA"],
					"account-2": ["ContractB"]
				}
			},
			"accounts": {
				"account-1": {
					"address": "0xf8d6e0586b0a20c7",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				},
				"account-2": {
					"address": "0x045a1763c93006ca",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				}
			},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.False(t, result, "contracts deployed to different accounts should not have account access")
	})

	t.Run("mixed location types not supported", func(t *testing.T) {
		t.Parallel()
		resolver := resolvers{}
		addr, err := common.HexToAddress("01")
		assert.NoError(t, err)

		checker := createTestChecker(t, common.AddressLocation{Address: addr, Name: "ContractA"})
		memberLoc := common.StringLocation("/some/path.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.False(t, result, "mixed location types should not be supported")
	})

	t.Run("no config manager", func(t *testing.T) {
		t.Parallel()
		resolver := resolvers{} // no cfgManager

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.False(t, result, "should return false when no config manager")
	})

	t.Run("contracts on same account across multiple networks", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		// Contracts are on same account in testnet but different accounts in mainnet
		flowJSON := []byte(`{
			"contracts": {
				"ContractA": "./ContractA.cdc",
				"ContractB": "./ContractB.cdc"
			},
			"deployments": {
				"testnet": {
					"testnet-account": ["ContractA", "ContractB"]
				},
				"mainnet": {
					"mainnet-account-1": ["ContractA"],
					"mainnet-account-2": ["ContractB"]
				}
			},
			"accounts": {
				"testnet-account": {
					"address": "0xf8d6e0586b0a20c7",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				},
				"mainnet-account-1": {
					"address": "0x1e4aa0b87d10b141",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				},
				"mainnet-account-2": {
					"address": "0x045a1763c93006ca",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				}
			},
			"networks": {
				"testnet": "access.devnet.nodes.onflow.org:9000",
				"mainnet": "access.mainnet.nodes.onflow.org:9000"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.True(t, result, "should allow access if contracts are on same account on ANY network")
	})

	t.Run("contracts with aliases same address", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		// Both contracts have aliases pointing to same address on emulator
		flowJSON := []byte(`{
			"contracts": {
				"ContractA": {
					"source": "./ContractA.cdc",
					"aliases": {
						"emulator": "0xf8d6e0586b0a20c7"
					}
				},
				"ContractB": {
					"source": "./ContractB.cdc",
					"aliases": {
						"emulator": "0xf8d6e0586b0a20c7"
					}
				}
			},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.True(t, result, "contracts with aliases at same address should have account access")
	})

	t.Run("contracts with aliases different addresses", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		// Contracts have aliases pointing to different addresses
		flowJSON := []byte(`{
			"contracts": {
				"ContractA": {
					"source": "./ContractA.cdc",
					"aliases": {
						"emulator": "0xf8d6e0586b0a20c7"
					}
				},
				"ContractB": {
					"source": "./ContractB.cdc",
					"aliases": {
						"emulator": "0x045a1763c93006ca"
					}
				}
			},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.False(t, result, "contracts with aliases at different addresses should not have account access")
	})

	t.Run("deployments take priority over aliases", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`access(all) contract ContractA {}`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`access(all) contract ContractB {}`), 0644)
		assert.NoError(t, err)

		// ContractA has both deployment and alias - deployment should win
		// ContractB only has alias at same address as ContractA deployment
		flowJSON := []byte(`{
			"contracts": {
				"ContractA": {
					"source": "./ContractA.cdc",
					"aliases": {
						"emulator": "0x045a1763c93006ca"
					}
				},
				"ContractB": {
					"source": "./ContractB.cdc",
					"aliases": {
						"emulator": "0xf8d6e0586b0a20c7"
					}
				}
			},
			"deployments": {
				"emulator": {
					"emulator-account": ["ContractA"]
				}
			},
		"accounts": {
			"emulator-account": {
				"address": "0xf8d6e0586b0a20c7",
				"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
			}
		},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		checker := createTestChecker(t, common.StringLocation("/proj/ContractA.cdc"))
		memberLoc := common.StringLocation("/proj/ContractB.cdc")

		result := resolver.accountAccess("/proj/flow.json", checker, memberLoc)
		assert.True(t, result, "deployment address should be used instead of alias, matching ContractB's alias")
	})

	t.Run("nested imports with contract identifiers", func(t *testing.T) {
		t.Parallel()
		mem := afero.NewMemMapFs()
		af := afero.Afero{Fs: mem}

		// Setup: ContractA imports ContractB (by identifier) which imports ContractC (by identifier)
		// All three should be on the same account
		err := af.MkdirAll("/proj", 0755)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractA.cdc", []byte(`
			import ContractB
			access(all) contract ContractA {
				access(all) fun foo() {
					ContractB.bar()
				}
			}
		`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractB.cdc", []byte(`
			import ContractC
			access(all) contract ContractB {
				access(account) fun bar() {
					ContractC.baz()
				}
			}
		`), 0644)
		assert.NoError(t, err)
		err = af.WriteFile("/proj/ContractC.cdc", []byte(`
			access(all) contract ContractC {
				access(account) fun baz() {}
			}
		`), 0644)
		assert.NoError(t, err)

		flowJSON := []byte(`{
			"contracts": {
				"ContractA": "./ContractA.cdc",
				"ContractB": "./ContractB.cdc",
				"ContractC": "./ContractC.cdc"
			},
			"deployments": {
				"emulator": {
					"emulator-account": ["ContractA", "ContractB", "ContractC"]
				}
			},
			"accounts": {
				"emulator-account": {
					"address": "0xf8d6e0586b0a20c7",
					"key": "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881"
				}
			},
			"networks": {
				"emulator": "127.0.0.1:3569"
			}
		}`)
		err = af.WriteFile("/proj/flow.json", flowJSON, 0644)
		assert.NoError(t, err)

		cm := NewConfigManager(af, false, 0, "/proj/flow.json")
		resolver := resolvers{loader: af, cfgManager: cm}

		// Test ContractB accessing ContractC (both are contract identifiers when imported)
		checkerB := createTestChecker(t, common.StringLocation("ContractB"))
		memberLocC := common.StringLocation("ContractC")

		result := resolver.accountAccess("/proj/flow.json", checkerB, memberLocC)
		assert.True(t, result, "ContractB should have account access to ContractC when both deployed to same account")

		// Test ContractA accessing ContractB
		checkerA := createTestChecker(t, common.StringLocation("ContractA"))
		memberLocB := common.StringLocation("ContractB")

		result = resolver.accountAccess("/proj/flow.json", checkerA, memberLocB)
		assert.True(t, result, "ContractA should have account access to ContractB when both deployed to same account")
	})
}
