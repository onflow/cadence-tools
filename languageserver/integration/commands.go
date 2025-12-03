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
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flowkit/v2/arguments"

	"github.com/onflow/cadence-tools/languageserver/server"
)

const (
	CommandSendTransaction     = "cadence.server.flow.sendTransaction"
	CommandExecuteScript       = "cadence.server.flow.executeScript"
	CommandDeployContract      = "cadence.server.flow.deployContract"
	CommandCreateAccount       = "cadence.server.flow.createAccount"
	CommandSwitchActiveAccount = "cadence.server.flow.switchActiveAccount"
	CommandGetAccounts         = "cadence.server.flow.getAccounts"
	CommandReloadConfig        = "cadence.server.flow.reloadConfiguration"
)

type commands struct {
	cfg *ConfigManager
}

// clientForPath returns the client resolved for the given path, or the default client.
func (c *commands) clientForPath(path string) flowClient {
	if c.cfg == nil {
		return nil
	}
	// If no specific path is provided, do not attempt resolution; use the default client
	// (which itself prefers the last-used project when available).
	if path == "" {
		return c.cfg.DefaultClient()
	}
	if cl, err := c.cfg.ResolveClientForPath(path); err == nil && cl != nil {
		return cl
	}
	return c.cfg.DefaultClient()
}

func (c *commands) getAll() []server.Command {
	// Commands always available
	commands := []server.Command{{
		Name:    CommandReloadConfig,
		Handler: c.reloadConfig,
	}}

	// Always register flow commands; handlers will error if client is not initialized
	commands = append(commands, []server.Command{
		{
			Name:    CommandSendTransaction,
			Handler: c.sendTransaction,
		},
		{
			Name:    CommandExecuteScript,
			Handler: c.executeScript,
		},
		{
			Name:    CommandDeployContract,
			Handler: c.deployContract,
		},
		{
			Name:    CommandSwitchActiveAccount,
			Handler: c.switchActiveAccount,
		},
		{
			Name:    CommandCreateAccount,
			Handler: c.createAccount,
		},
		{
			Name:    CommandGetAccounts,
			Handler: c.getAccounts,
		},
	}...)

	return commands
}

// sendTransaction handles submitting a transaction defined in the
// source document in VS Code.
//
// There should be exactly 3 arguments:
//   - the DocumentURI of the file to submit
//   - the arguments, encoded as JSON-CDC
//   - the signer names as list
func (c *commands) sendTransaction(args ...json.RawMessage) (any, error) {
	err := server.CheckCommandArgumentCount(args, 3)
	if err != nil {
		return nil, fmt.Errorf("arguments error: %w", err)
	}

	location, err := parseLocation(args[0])
	if err != nil {
		return nil, err
	}

	var argsJSON string
	err = json.Unmarshal(args[1], &argsJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction arguments: %s", args[1])
	}

	txArgs, err := arguments.ParseJSON(argsJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction arguments cadence encoding format: %s, error: %s", argsJSON, err)
	}

	var signerList []string
	err = json.Unmarshal(args[2], &signerList)
	if err != nil {
		return nil, fmt.Errorf("invalid signer list: %s", args[2])
	}

	// Resolve appropriate client based on the file's closest flow.json
	client := c.clientForPath(location.Path)
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}

	signerAddresses := make([]flow.Address, 0)
	for _, name := range signerList {
		account := client.GetClientAccount(name)
		if account == nil {
			return nil, fmt.Errorf("signer account with name %s doesn't exist", name)
		}

		signerAddresses = append(signerAddresses, account.Address)
	}

	tx, txResult, err := client.SendTransaction(signerAddresses, location, txArgs)
	if err != nil {
		return nil, fmt.Errorf("transaction error: %w", err)
	}

	return fmt.Sprintf(
		"Transaction %s with ID %s. Events: %s",
		txResult.Status,
		tx.ID(),
		txResult.Events,
	), err
}

// executeScript handles executing a script defined in the source document.
//
// There should be exactly 2 arguments:
//   - the DocumentURI of the file to submit
//   - the arguments, encoded as JSON-CDC
func (c *commands) executeScript(args ...json.RawMessage) (any, error) {
	err := server.CheckCommandArgumentCount(args, 2)
	if err != nil {
		return nil, fmt.Errorf("arguments error: %w", err)
	}

	location, err := parseLocation(args[0])
	if err != nil {
		return nil, err
	}

	var argsJSON string
	err = json.Unmarshal(args[1], &argsJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid script arguments: %s", args[1])
	}

	scriptArgs, err := arguments.ParseJSON(argsJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid script arguments cadence encoding format: %s, error: %s", argsJSON, err)
	}

	client := c.clientForPath(location.Path)
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}

	scriptResult, err := client.ExecuteScript(location, scriptArgs)
	if err != nil {
		return nil, fmt.Errorf("script error: %w", err)
	}

	return fmt.Sprintf("Result: %s", scriptResult.String()), nil
}

// switchActiveAccount sets the account that is currently active and could be used
// when submitting transactions.
//
// There should be 1 argument:
//   - name of the new active account
func (c *commands) switchActiveAccount(args ...json.RawMessage) (any, error) {
	err := server.CheckCommandArgumentCount(args, 1)
	if err != nil {
		return nil, fmt.Errorf("arguments error: %w", err)
	}

	var name string
	err = json.Unmarshal(args[0], &name)
	if err != nil {
		return nil, fmt.Errorf("invalid name argument value: %s", args[0])
	}

	client := c.clientForPath("")
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}
	err = client.SetActiveClientAccount(name)
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("Account switched to %s", name), nil
}

// reloadConfig when the client detects changes in flow.json so we have an updated state.
func (c *commands) reloadConfig(_ ...json.RawMessage) (any, error) {
	if c.cfg != nil {
		return nil, c.cfg.ReloadAll()
	}
	return nil, nil
}

// getAccounts return the client account list with information about the active client.
func (c *commands) getAccounts(_ ...json.RawMessage) (any, error) {
	client := c.clientForPath("")
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}
	return client.GetClientAccounts(), nil
}

// createAccount creates a new account and returns its address.
func (c *commands) createAccount(_ ...json.RawMessage) (any, error) {
	client := c.clientForPath("")
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}
	account, err := client.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("create account error: %w", err)
	}

	return account, nil
}

// deployContract deploys the contract to the configured account with the code of the given
// file.
//
// There should be exactly 3 arguments:
//   - the DocumentURI of the file to submit
//   - the name of the contract
//   - the signer names as list
func (c *commands) deployContract(args ...json.RawMessage) (any, error) {
	err := server.CheckCommandArgumentCount(args, 3)
	if err != nil {
		return nil, fmt.Errorf("arguments error: %w", err)
	}

	location, err := parseLocation(args[0])
	if err != nil {
		return nil, err
	}

	var name string
	err = json.Unmarshal(args[1], &name)
	if err != nil {
		return nil, fmt.Errorf("invalid name argument: %s", args[1])
	}

	var signerName string
	err = json.Unmarshal(args[2], &signerName)
	if err != nil {
		return nil, fmt.Errorf("invalid signer name: %s", args[2])
	}

	// Resolve client for this location
	client := c.clientForPath(location.Path)
	if client == nil {
		return nil, fmt.Errorf("flow client is not initialized")
	}

	var account *clientAccount
	if signerName == "" { // choose default active account
		account = client.GetActiveClientAccount()
	} else {
		account = client.GetClientAccount(signerName)
		if account == nil {
			return nil, fmt.Errorf("signer account with name %s doesn't exist", signerName)
		}
	}

	deployError := client.DeployContract(account.Address, name, location)
	if deployError != nil {
		return nil, fmt.Errorf("error deploying contract: %w", deployError)
	}

	return fmt.Sprintf("Contract %s has been deployed to account %s", name, account.Name), err
}

func parseLocation(arg []byte) (*url.URL, error) {
	var uri string
	err := json.Unmarshal(arg, &uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI argument: %s", arg)
	}

	location, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid path argument: %s", uri)
	}

	location.Path = cleanWindowsPath(location.Path)

	return location, nil
}
