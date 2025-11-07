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
	neturl "net/url"
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
	loader     flowkit.ReaderWriter
	cfgManager *ConfigManager
}

// deURI normalizes a possibly URI-formatted path (e.g., file:///...) and decodes percent-escapes.
func deURI(path string) string {
	path = strings.TrimPrefix(path, "file://")
	// Decode percent-escapes like %20
	if unescaped, err := neturl.PathUnescape(path); err == nil {
		path = unescaped
	}
	// Windows drive letter normalization already handled by cleanWindowsPath for %3A, keep this as last step
	return path
}

// stringImport loads the code for a string location that can either be file path or contract identifier.
// projectID identifies the project scope (e.g., abs flow.json path) established by the server
func (r *resolvers) stringImport(projectID string, location common.StringLocation) (string, error) {
	name := location.String()
	if !strings.Contains(name, ".cdc") {
		return r.resolveStringIdentifierImport(projectID, name)
	}
	return r.resolveFileImport(projectID, name)
}

// resolveStringIdentifierImport resolves a string identifier import within the given project.
// e.g. import "Contract"
func (r *resolvers) resolveStringIdentifierImport(projectID string, name string) (string, error) {
	if r.cfgManager == nil || projectID == "" {
		return "", fmt.Errorf("no project context available for identifier import")
	}
	if st, _ := r.cfgManager.ResolveStateForProject(projectID); st != nil {
		if code, err := st.GetCodeByName(name); err == nil {
			return code, nil
		}
	}
	if mapped, _ := r.cfgManager.GetContractSourceForProject(projectID, name); mapped != "" {
		return mapped, nil
	}
	return "", fmt.Errorf("failed to resolve project state")
}

// resolveFileImport resolves a file import path relative to the project root if necessary.
// e.g. import "./contract.cdc"
func (r *resolvers) resolveFileImport(projectID string, name string) (string, error) {
	filename := deURI(cleanWindowsPath(name))

	if data, err := r.loader.ReadFile(filename); err == nil {
		if r.cfgManager != nil && projectID != "" {
			if !r.cfgManager.IsPathInProject(projectID, filename) || !r.cfgManager.IsSameProject(projectID, filename) {
				return "", fmt.Errorf("import path crosses outside project root")
			}
		}
		return string(data), nil
	}

	if r.cfgManager != nil && projectID != "" && !filepath.IsAbs(filename) {
		if cfg := r.cfgManager.ConfigPathForProject(projectID); cfg != "" {
			candidate := filepath.Join(filepath.Dir(cfg), filename)
			if !r.cfgManager.IsPathInProject(projectID, candidate) || !r.cfgManager.IsSameProject(projectID, candidate) {
				return "", fmt.Errorf("import path crosses outside project root")
			}
			if data, err := r.loader.ReadFile(candidate); err == nil {
				return string(data), nil
			}
		}
	}

	data, err := r.loader.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// addressImport loads the code for an address location.
func (r *resolvers) addressImport(projectID string, location common.AddressLocation) (string, error) {
	if r.cfgManager == nil || projectID == "" {
		return "", errors.New("client is not initialized")
	}
	cl, err := r.cfgManager.ResolveClientForProject(projectID)
	if err != nil || cl == nil {
		return "", errors.New("client is not initialized")
	}
	if cl == nil {
		return "", errors.New("client is not initialized")
	}

	account, err := cl.GetAccount(flow.HexToAddress(location.Address.String()))
	if err != nil {
		return "", err
	}

	return string(account.Contracts[location.Name]), nil
}

// identifierImport resolves the code for an identifier location.
func (r *resolvers) identifierImportProject(projectID string, location common.IdentifierLocation) (string, error) {
	if location == stdlib.CryptoContractLocation {
		return string(coreContracts.Crypto()), nil
	}
	return "", fmt.Errorf("unknown identifier location: %s", location)
}

// addressContractNames returns a slice of all the contract names on the address location.
func (r *resolvers) addressContractNames(address common.Address) ([]string, error) {
	if r.cfgManager == nil {
		return nil, errors.New("client is not initialized")
	}
	cl := r.cfgManager.DefaultClient()
	if cl == nil {
		return nil, errors.New("client is not initialized")
	}
	account, err := cl.GetAccount(flow.HexToAddress(address.String()))
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

// buildNameToAddressByNetwork builds per network: contract name -> address
func (r *resolvers) buildNameToAddressByNetwork(state flowState) map[string]map[string]flow.Address {
	nameToAddressByNetwork := make(map[string]map[string]flow.Address)
	if state == nil || state.getState() == nil {
		return nameToAddressByNetwork
	}

	stateNetworks := state.getState().Networks()
	if stateNetworks == nil {
		return nameToAddressByNetwork
	}

	networks := *stateNetworks
	for _, network := range networks {
		networkName := network.Name
		contractNameToAddress := make(map[string]flow.Address)

		// Add aliases first
		stateContracts := state.getState().Contracts()
		if stateContracts != nil {
			for _, contract := range *stateContracts {
				alias := contract.Aliases.ByNetwork(networkName)
				if alias != nil {
					contractNameToAddress[contract.Name] = alias.Address
				}
			}
		}

		// Add deployments (overwrites aliases, giving deployments priority)
		deployedContracts, err := state.getState().DeploymentContractsByNetwork(network)
		if err == nil {
			for _, deployedContract := range deployedContracts {
				contract, err := state.getState().Contracts().ByName(deployedContract.Name)
				if err == nil {
					address, err := state.getState().ContractAddress(contract, network)
					if err == nil && address != nil {
						contractNameToAddress[deployedContract.Name] = *address
					}
				}
			}
		}

		if len(contractNameToAddress) > 0 {
			nameToAddressByNetwork[networkName] = contractNameToAddress
		}
	}

	return nameToAddressByNetwork
}

// normalizeAbs converts p to an absolute, symlink-resolved path. If base is non-empty
// and p is relative, it is joined to base first.
func normalizeAbs(base, p string) string {
	if p == "" {
		return ""
	}
	p = deURI(cleanWindowsPath(p))
	if base != "" && !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	if ap, err := filepath.Abs(p); err == nil {
		p = ap
	}
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		p = rp
	}
	return p
}

// buildFileToContractName builds a map from absolute file paths to contract names
func (r *resolvers) buildFileToContractName(state flowState) map[string]string {
	filePathToContractName := make(map[string]string)
	if state == nil || state.getState() == nil {
		return filePathToContractName
	}

	stateContracts := state.getState().Contracts()
	if stateContracts == nil {
		return filePathToContractName
	}

	configDir := filepath.Dir(state.getConfigPath())
	for _, contract := range *stateContracts {
		absolutePath := normalizeAbs(configDir, contract.Location)
		if absolutePath != "" {
			filePathToContractName[absolutePath] = contract.Name
		}
	}

	return filePathToContractName
}

// accountAccess checks if checker and member are at the same address on at least one network
func (r *resolvers) accountAccess(projectID string, checker *sema.Checker, memberLocation common.Location) bool {
	// If both are AddressLocation, directly compare addresses
	// Otherwise, both must be StringLocation and we need to resolve the contract -> account mappings per network.
	if checkerAddr, ok := checker.Location.(common.AddressLocation); ok {
		if memberAddr, ok := memberLocation.(common.AddressLocation); ok {
			return checkerAddr.Address == memberAddr.Address
		}
	}

	if r.cfgManager == nil {
		return false
	}

	state, err := r.cfgManager.ResolveStateForProject(projectID)
	if err != nil || state == nil {
		return false
	}

	// Build mappings: contract name to address per network
	contractNameToAddressByNetwork := r.buildNameToAddressByNetwork(state)
	if len(contractNameToAddressByNetwork) == 0 {
		return false
	}

	// Build mapping: file path → contract name
	filePathToContractName := r.buildFileToContractName(state)

	// Helper to resolve contract name from StringLocation
	resolveContractName := func(location common.StringLocation) string {
		locationString := location.String()

		// If it's a contract identifier, return it as is
		if !strings.Contains(locationString, ".cdc") {
			return locationString
		}

		// Otherwise, resolve file path and check for known contract name
		absolutePath := normalizeAbs("", locationString)
		if contractName, exists := filePathToContractName[absolutePath]; exists {
			return contractName
		}

		return ""
	}

	// Get checker contract name from location (only support StringLocation)
	checkerLocation, ok := checker.Location.(common.StringLocation)
	if !ok {
		return false
	}
	checkerContractName := resolveContractName(checkerLocation)
	if checkerContractName == "" {
		return false
	}

	// Get member contract name from location (only support StringLocation)
	memberStringLocation, ok := memberLocation.(common.StringLocation)
	if !ok {
		return false
	}
	memberContractName := resolveContractName(memberStringLocation)
	if memberContractName == "" {
		return false
	}

	// Check if they're at the same address on at least one network
	for _, contractNameToAddress := range contractNameToAddressByNetwork {
		checkerAddress, checkerExists := contractNameToAddress[checkerContractName]
		memberAddress, memberExists := contractNameToAddress[memberContractName]
		if checkerExists && memberExists && checkerAddress == memberAddress {
			return true
		}
	}
	return false
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
