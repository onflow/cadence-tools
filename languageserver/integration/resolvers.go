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
	"github.com/onflow/flowkit/v2/config"

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
	// State-backed resolution only to ensure canonicalization and imports share identity
	st, stateErr := r.cfgManager.ResolveStateForProject(projectID)
	if stateErr != nil {
		return "", fmt.Errorf("failed to load project state for %q: %w", projectID, stateErr)
	}
	if st == nil {
		return "", fmt.Errorf("project state is nil for %q", projectID)
	}
	code, err := st.GetCodeByName(name)
	if err != nil {
		return "", fmt.Errorf("failed to get code for %q: %w", name, err)
	}
	return code, nil
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
	// cl already checked; no need to re-check nil

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
	// Resolve via state
	if r.cfgManager != nil && projectID != "" {
		if st, err := r.cfgManager.ResolveStateForProject(projectID); err == nil && st != nil {
			if code, err := st.GetCodeByName(string(location)); err == nil {
				return code, nil
			}
		}
		return "", fmt.Errorf("failed to resolve identifier location %q from project state", location)
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

// accountAccess checks whether the current program location and accessed program location were deployed to the same account.
//
// if the contracts were deployed on the same account then it returns true and hence allows the access, false otherwise.
func (r *resolvers) accountAccess(checker *sema.Checker, memberLocation common.Location) bool {
	if r.cfgManager == nil {
		return false
	}
	cl, err := r.cfgManager.ResolveClientForChecker(checker)
	if err != nil || cl == nil {
		return false
	}

	contracts, err := cl.getState().getState().DeploymentContractsByNetwork(config.EmulatorNetwork)
	if err != nil {
		return false
	}

	var checkerAccount, memberAccount string
	// go over contracts and match contract by the location of checker and member and assign the account name for later check
	for _, c := range contracts {
		// get absolute path of the contract relative to the dir where flow.json is (working env)
		absLocation, _ := filepath.Abs(filepath.Join(filepath.Dir(cl.getConfigPath()), c.Location()))

		if memberLocation.String() == absLocation {
			memberAccount = c.AccountName
		}
		if checker.Location.String() == absLocation {
			checkerAccount = c.AccountName
		}
	}

	return checkerAccount == memberAccount && checkerAccount != "" && memberAccount != ""
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
