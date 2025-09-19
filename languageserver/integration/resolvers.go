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
	state      flowState
	client     flowClient
	loader     flowkit.ReaderWriter
	cfgManager *ConfigManager
}

//

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
func (r *resolvers) stringImport(checker *sema.Checker, location common.StringLocation) (string, error) {
	// if the location is not a cadence file try getting the code by identifier
	if !strings.Contains(location.String(), ".cdc") {
		st := r.state
		originStr := ""
		if checker != nil && checker.Location != nil {
			originStr = deURI(checker.Location.String())
		}
		if r.cfgManager != nil && originStr != "" {
			if resolved, err := r.cfgManager.ResolveStateForPath(originStr); err == nil && resolved != nil {
				st = resolved
			}
		}
		code, err := st.GetCodeByName(location.String())
		if err != nil {
			return code, err
		}
		return code, nil
	}

	filename := cleanWindowsPath(location.String())
	filename = deURI(filename)
	// Resolve relative path imports against the originating file's directory
	if checker != nil && checker.Location != nil && !filepath.IsAbs(filename) {
		base := filepath.Dir(deURI(checker.Location.String()))
		filename = filepath.Join(base, filename)
	}

	// Disallow crossing outside the origin project's root (closest flow.json directory),
	// and also disallow importing from another project's config coverage
	if r.cfgManager != nil && checker != nil && checker.Location != nil {
		originPath := deURI(checker.Location.String())
		srcCfg := r.cfgManager.NearestConfigPath(originPath)

		// Normalize absolute paths
		absFile := filename
		if !filepath.IsAbs(absFile) {
			if af, err := filepath.Abs(absFile); err == nil {
				absFile = af
			}
		}
		if srcCfg != "" {
			projectRoot := filepath.Dir(srcCfg)
			absRoot := projectRoot
			if !filepath.IsAbs(absRoot) {
				if ar, err := filepath.Abs(absRoot); err == nil {
					absRoot = ar
				}
			}
			if rel, err := filepath.Rel(absRoot, absFile); err == nil {
				// If the relative path starts with "..", the file is outside the project root
				if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return "", fmt.Errorf("import path crosses outside project root")
				}
			}
		}

		// If destination also resolves to a different config, error
		dstCfg := r.cfgManager.NearestConfigPath(absFile)
		if srcCfg != "" && dstCfg != "" && srcCfg != dstCfg {
			return "", fmt.Errorf("import path crosses into a different project/config")
		}
	}
	data, err := r.loader.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// addressImport loads the code for an address location.
func (r *resolvers) addressImport(checker *sema.Checker, location common.AddressLocation) (string, error) {
	cl := r.client
	if r.cfgManager != nil && checker != nil {
		if resolved, err := r.cfgManager.ResolveClientForChecker(checker); err == nil && resolved != nil {
			cl = resolved
		}
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
func (r *resolvers) identifierImport(_ *sema.Checker, location common.IdentifierLocation) (string, error) {
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
	cl := r.client
	if r.cfgManager != nil {
		if resolved, err := r.cfgManager.ResolveClientForChecker(checker); err == nil && resolved != nil {
			cl = resolved
		}
	}
	if cl == nil {
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
