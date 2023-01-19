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

package integration

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/flow-cli/pkg/flowkit"
	"github.com/onflow/flow-cli/pkg/flowkit/config"
	"github.com/onflow/flow-go-sdk"
	"path/filepath"
	"strings"
)

type resolvers struct {
	client *flowkitClient
	loader flowkit.ReaderWriter
}

// fileImport loads the code for a string location.
func (r *resolvers) fileImport(location common.StringLocation) (string, error) {
	filename := cleanWindowsPath(location.String())

	data, err := r.loader.ReadFile(filename)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// addressImport loads the code for an address location.
func (r *resolvers) addressImport(location common.AddressLocation) (string, error) {
	account, err := r.client.GetAccount(flow.HexToAddress(location.Address.String()))
	if err != nil {
		return "", err
	}

	return string(account.Contracts[location.Name]), nil
}

// addressContractNames returns a slice of all the contract names on the address location.
func (r *resolvers) addressContractNames(address common.Address) ([]string, error) {
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
	contracts, err := r.client.state.DeploymentContractsByNetwork(config.DefaultEmulatorNetwork().Name)
	if err != nil {
		return false
	}

	var checkerAccount, memberAccount string
	// go over contracts and match contract by the location of checker and member and assign the account name for later check
	for _, c := range contracts {
		// get absolute path of the contract relative to flow.json
		absLocation, _ := filepath.Abs(filepath.Join(filepath.Dir(r.client.configPath), c.Source))

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
	if strings.Contains(path, ":") {
		path = path[1:]
	}
	return path
}
