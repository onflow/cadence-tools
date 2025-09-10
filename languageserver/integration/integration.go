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
	"strconv"

	"github.com/onflow/cadence/sema"

	"github.com/onflow/flowkit/v2"
	"github.com/spf13/afero"

	"github.com/onflow/cadence-tools/languageserver/protocol"
	"github.com/onflow/cadence-tools/languageserver/server"
)

func NewFlowIntegration(s *server.Server, enableFlowClient bool) (*FlowIntegration, error) {
	loader := &afero.Afero{Fs: afero.NewOsFs()}
	state := newFlowkitState(loader)

	integration := &FlowIntegration{
		entryPointInfo:   map[protocol.DocumentURI]*entryPointInfo{},
		contractInfo:     map[protocol.DocumentURI]*contractInfo{},
		enableFlowClient: enableFlowClient,
		loader:           loader,
		state:            state,
	}

	resolve := resolvers{
		loader: loader,
		state:  state,
	}

	options := []server.Option{
		server.WithDiagnosticProvider(diagnostics),
		server.WithStringImportResolver(resolve.stringImport),
		server.WithInitializationOptionsHandler(integration.initialize),
		server.WithExtendedStandardLibraryValues(FVMStandardLibraryValues()...),
		server.WithIdentifierImportResolver(resolve.identifierImport),
		server.WithMemberAccountAccessHandler(resolve.accountAccess),
	}

	if enableFlowClient {
		client := newFlowkitClient(loader)
		integration.client = client
		resolve.client = client

		options = append(options,
			server.WithCodeLensProvider(integration.codeLenses),
			server.WithAddressImportResolver(resolve.addressImport),
			server.WithAddressContractNamesResolver(resolve.addressContractNames),
		)
	}

	comm := commands{client: integration.client, state: integration.state}
	for _, command := range comm.getAll() {
		options = append(options, server.WithCommand(command))
	}

	err := s.SetOptions(options...)
	if err != nil {
		return nil, err
	}

	return integration, nil
}

type FlowIntegration struct {
	entryPointInfo map[protocol.DocumentURI]*entryPointInfo
	contractInfo   map[protocol.DocumentURI]*contractInfo

	enableFlowClient bool
	client           flowClient
	state            *flowkitState
	loader           flowkit.ReaderWriter
}

func (i *FlowIntegration) initialize(initializationOptions any) error {
	optsMap, ok := initializationOptions.(map[string]any)
	if !ok {
		// If client is enabled, initialization options are required
		if i.enableFlowClient {
			return errors.New("invalid initialization options")
		}
		return nil
	}

	configPath, ok := optsMap["configPath"].(string)
	if !ok || configPath == "" {
		// If client is enabled, config path is required, otherwise it's optional
		if i.enableFlowClient {
			return errors.New("initialization options: invalid config path")
		}
		return nil
	}

	// Load the config state if provided
	configPath = cleanWindowsPath(configPath)
	err := i.state.Load(configPath)
	if err != nil {
		return err
	}

	// If client is enabled, initialize the client
	if i.enableFlowClient {
		numberOfAccountsString, ok := optsMap["numberOfAccounts"].(string)
		if !ok || numberOfAccountsString == "" {
			return errors.New("initialization options: invalid account number value, should be passed as a string")
		}
		numberOfAccounts, err := strconv.Atoi(numberOfAccountsString)
		if err != nil {
			return errors.New("initialization options: invalid account number value")
		}

		err = i.client.Initialize(i.state, numberOfAccounts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (i *FlowIntegration) codeLenses(
	uri protocol.DocumentURI,
	version int32,
	checker *sema.Checker,
) (
	[]*protocol.CodeLens,
	error,
) {
	var actions []*protocol.CodeLens

	// todo refactor - define codelens provider interface and merge both into one

	// Add code lenses for contracts and contract interfaces
	contract := i.contractInfo[uri]
	if contract == nil {
		contract = &contractInfo{} // create new
		i.contractInfo[uri] = contract
	}
	contract.update(uri, version, checker)
	actions = append(actions, contract.codelens(i.client)...)

	// Add code lenses for scripts and transactions
	entryPoint := i.entryPointInfo[uri]
	if entryPoint == nil {
		entryPoint = &entryPointInfo{}
		i.entryPointInfo[uri] = entryPoint
	}
	entryPoint.update(uri, version, checker)
	actions = append(actions, entryPoint.codelens(i.client)...)

	return actions, nil
}
