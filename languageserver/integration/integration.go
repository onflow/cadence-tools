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

	"path/filepath"
	"strings"

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

	// Always create a config manager so per-file config discovery works even without init options
	integration.cfgManager = NewConfigManager(loader, enableFlowClient, 0, "")

	resolve := resolvers{
		loader:     loader,
		state:      state,
		cfgManager: integration.cfgManager,
	}

    options := []server.Option{
		server.WithDiagnosticProvider(diagnostics),
		server.WithStringImportResolver(resolve.stringImport),
		server.WithInitializationOptionsHandler(integration.initialize),
		server.WithExtendedStandardLibraryValues(FVMStandardLibraryValues()...),
		server.WithIdentifierImportResolver(resolve.identifierImport),
	}

	if enableFlowClient {
		client := newFlowkitClient(loader)
		integration.client = client
		resolve.client = client

        options = append(options,
			server.WithCodeLensProvider(integration.codeLenses),
			server.WithAddressImportResolver(resolve.addressImport),
			server.WithAddressContractNamesResolver(resolve.addressContractNames),
            server.WithMemberAccountAccessHandler(resolve.accountAccess),
		)
	}

    comm := commands{cfg: integration.cfgManager}
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
	cfgManager       *ConfigManager
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
		// Keep existing config manager; no default path, multi-config resolution only
		return nil
	}

	// Load the config state if provided
	configPath = cleanWindowsPath(configPath)
	err := i.state.Load(configPath)
	if err != nil {
		return err
	}

	// Initialize ConfigManager with provided default config path
	numberOfAccounts := 0
	if i.enableFlowClient {
		if numberOfAccountsString, ok := optsMap["numberOfAccounts"].(string); ok && numberOfAccountsString != "" {
			if n, convErr := strconv.Atoi(numberOfAccountsString); convErr == nil {
				numberOfAccounts = n
			}
		}
	}
	// Reuse existing manager instance to avoid stale references
	i.cfgManager.enableFlowClient = i.enableFlowClient
	i.cfgManager.numberOfAccounts = numberOfAccounts
	i.cfgManager.SetDefaultConfigPath(configPath)

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
        // Seed default client into ConfigManager to be available for commands without path
        if i.cfgManager != nil {
            i.cfgManager.SetDefaultClientForPath(configPath, i.client)
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

	// Ensure we watch the document directory for flow.json creation/removal
	if i.cfgManager != nil {
		// Only for file:// URIs
		u := string(uri)
		if strings.HasPrefix(u, "file://") {
			path := cleanWindowsPath(strings.TrimPrefix(u, "file://"))
			dir := filepath.Dir(path)
			i.cfgManager.EnsureDirWatcher(dir)
		}
	}

	// Resolve per-document client if available
	clientToUse := i.client
	if i.cfgManager != nil {
		if cl, err := i.cfgManager.ResolveClientForChecker(checker); err == nil && cl != nil {
			clientToUse = cl
		}
	}

	// Add code lenses for contracts and contract interfaces
	contract := i.contractInfo[uri]
	if contract == nil {
		contract = &contractInfo{} // create new
		i.contractInfo[uri] = contract
	}
	contract.update(uri, version, checker)
	actions = append(actions, contract.codelens(clientToUse)...)

	// Add code lenses for scripts and transactions
	entryPoint := i.entryPointInfo[uri]
	if entryPoint == nil {
		entryPoint = &entryPointInfo{}
		i.entryPointInfo[uri] = entryPoint
	}
	entryPoint.update(uri, version, checker)
	actions = append(actions, entryPoint.codelens(clientToUse)...)

	return actions, nil
}
