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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flowkit/v2"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
)

type resolvers struct {
	state  flowState
	client flowClient
	loader flowkit.ReaderWriter
}

// stringImport loads the code for a string location that can either be file path or contract identifier.
func (r *resolvers) stringImport(location common.StringLocation) (string, error) {
	// if the location is not a cadence file try getting the code by identifier
	if !strings.Contains(location.String(), ".cdc") {
		return r.state.GetCodeByName(location.String())
	}

	filename := cleanWindowsPath(location.String())
	data, err := r.loader.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// addressImport loads the code for an address location.
func (r *resolvers) addressImport(location common.AddressLocation) (string, error) {
	if r.client == nil {
		return "", errors.New("client is not initialized")
	}

	account, err := r.client.GetAccount(flow.HexToAddress(location.Address.String()))
	if err != nil {
		return "", err
	}

	return string(account.Contracts[location.Name]), nil
}

// identifierImport resolves the code for an identifier location.
func (r *resolvers) identifierImport(location common.IdentifierLocation) (string, error) {
	if location == stdlib.CryptoContractLocation {
		return string(coreContracts.Crypto()), nil
	}
	return "", fmt.Errorf("unknown identifier location: %s", location)
}

// addressContractNames returns a slice of all the contract names on the address location.
func (r *resolvers) addressContractNames(address common.Address) ([]string, error) {
	if r.client == nil {
		return nil, errors.New("client is not initialized")
	}

	account, err := r.client.GetAccount(flow.HexToAddress(address.String()))
	if err != nil {
		return nil, err
	}

	names := make([]string, len(account.Contracts))
	var index int
	for name := range account.Contracts {
		names[index] = name
		index++
	}

	return names, nil
}

// accountAccess checks whether the current program location and accessed program location were deployed to the same account.
//
// if the contracts were deployed on the same account then it returns true and hence allows the access, false otherwise.
func (r *resolvers) accountAccess(checker *sema.Checker, memberLocation common.Location) bool {
	if r.client == nil {
		return false
	}

	if checker.Location == nil || memberLocation == nil {
		return false
	}

	state := r.client.getState().getState()
	if state == nil {
		return false
	}

	// If checker and member locations are both address locations, we can directly compare them.
	checkerAddressLocation, ok := checker.Location.(common.AddressLocation)
	if ok {
		memberAddressLocation, ok := memberLocation.(common.AddressLocation)
		if ok {
			return checkerAddressLocation.Address == memberAddressLocation.Address
		}
	}

	// Otherwise, both locations must be string locations and we will compare their addresses for each network.
	checkerStringLocation, ok := checker.Location.(common.StringLocation)
	if !ok {
		return false
	}
	memberStringLocation, ok := memberLocation.(common.StringLocation)
	if !ok {
		return false
	}

	// Resolve account address for every configured network for both checker and member locations.
	// They should have the same addresses for every deployed network.
	checkerAddressesByNetwork, err := resolveLocationAddresses(
		state,
		r.client.getState().getConfigPath(),
		checkerStringLocation,
	)
	if err != nil {
		return false
	}
	memberAddressesByNetwork, err := resolveLocationAddresses(
		state,
		r.client.getState().getConfigPath(),
		memberStringLocation,
	)
	if err != nil {
		return false
	}

	// Check that member location matches for all of checker's networks.
	for network, checkerAddress := range checkerAddressesByNetwork {
		memberAddress, exists := memberAddressesByNetwork[network]
		if !exists || checkerAddress != memberAddress {
			return false
		}
	}
	
	return true
}

func resolveLocationAddresses(
	state *flowkit.State,
	configPath string,
	location common.StringLocation,
) (map[string]flow.Address, error) {
	if strings.Contains(location.String(), ".cdc") {
		return resolvePathLocationAddresses(state, configPath, location)
	} else {
		return resolveStringImportAddresses(state, location)
	}
}

func resolvePathLocationAddresses(
	state *flowkit.State,
	configPath string,
	location common.StringLocation,
) (map[string]flow.Address, error) {
	addresses := make(map[string]flow.Address)
	
	for _, contract := range *state.Contracts() {
		contractAbsLocation, err := filepath.Abs(filepath.Join(filepath.Dir(configPath), contract.Location))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path for contract %s: %w", contract.Name, err)
		}
		if contractAbsLocation == location.String() {
			for _, alias := range contract.Aliases {
				addresses[alias.Network] = alias.Address
			}
			return addresses, nil
		}
	}

	for _, network := range state.Config().Networks {
		contracts, err := state.DeploymentContractsByNetwork(network)
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment contracts for network %s: %w", network.Name, err)
		}
		for _, contract := range contracts {
			contractAbsLocation, err := filepath.Abs(filepath.Join(filepath.Dir(configPath), contract.Location()))
			if err != nil {
				return nil, fmt.Errorf("failed to resolve absolute path for contract %s: %w", contract.Name, err)
			}
			if contractAbsLocation == location.String() {
				addresses[network.Name] = contract.AccountAddress
				return addresses, nil
			}
		}
	}
	return addresses, nil
}

func resolveStringImportAddresses(
	state *flowkit.State,
	location common.StringLocation,
) (map[string]flow.Address, error) {
	addresses := make(map[string]flow.Address)

	for _, contract := range *state.Contracts() {
		if contract.Name == location.String() {
			for _, alias := range contract.Aliases {
				addresses[alias.Network] = alias.Address
			}
		}
	}

	for _, network := range state.Config().Networks {
		contracts, err := state.DeploymentContractsByNetwork(network)
		if err != nil {
			return nil, fmt.Errorf("failed to get deployment contracts for network %s: %w", network.Name, err)
		}
		for _, contract := range contracts {
			if contract.Name == location.String() {
				addresses[network.Name] = contract.AccountAddress
				break
			}
		}
	}

	return addresses, nil
}

// workaround for Windows files being sent with prefixed '/' which is /c:/test/foo
// we remove the first / for Windows files, so they are valid, also replace encoded column sign.
func cleanWindowsPath(path string) string {
	path = strings.ReplaceAll(path, "%3A", ":")
	if strings.Contains(path, ":") && strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	return path
}
