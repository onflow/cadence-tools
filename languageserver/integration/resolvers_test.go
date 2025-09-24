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
