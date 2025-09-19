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
		resolved, err := resolver.stringImport(nil, common.StringLocation("./test.cdc"))
		assert.NoError(t, err)
		assert.Equal(t, "hello test", resolved)
	})

	t.Run("non existing file", func(t *testing.T) {
		t.Parallel()
		resolved, err := resolver.stringImport(nil, common.StringLocation("./foo.cdc"))
		assert.EqualError(t, err, "open foo.cdc: file does not exist")
		assert.Equal(t, "", resolved)
	})
}

func Test_AddressImport(t *testing.T) {
	mock := &mockFlowClient{}
	resolver := resolvers{
		client: mock,
	}

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
		resolved, err := resolver.addressImport(nil, address)
		assert.NoError(t, err)
		assert.Equal(t, "hello tests", resolved)
	})

	t.Run("non existing contract import", func(t *testing.T) {
		address.Name = "invalid"
		resolved, err := resolver.addressImport(nil, address)
		assert.NoError(t, err)
		assert.Empty(t, resolved)
	})

	t.Run("non existing address", func(t *testing.T) {
		address.Address, _ = common.HexToAddress("2")
		resolved, err := resolver.addressImport(nil, address)
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
	mockFS := afero.NewMemMapFs()
	af := afero.Afero{Fs: mockFS}
	code := `access(all) contract Test {}`
	_ = afero.WriteFile(mockFS, "./test.cdc", []byte(code), 0644)

	mock := &mockFlowState{}
	resolver := resolvers{
		loader: af,
		state:  mock,
	}

	mock.On("GetCodeByName", "Test").Return(code, nil)
	mock.On("GetCodeByName", "Foo").Return("", fmt.Errorf("couldn't find the contract by import identifier: Foo"))

	t.Run("existing import", func(t *testing.T) {
		resolved, err := resolver.stringImport(nil, common.StringLocation("Test"))
		assert.NoError(t, err)
		assert.Equal(t, code, resolved)
	})

	t.Run("non existing import", func(t *testing.T) {
		resolved, err := resolver.stringImport(nil, common.StringLocation("Foo"))
		assert.EqualError(t, err, "couldn't find the contract by import identifier: Foo")
		assert.Empty(t, resolved)
	})
}
