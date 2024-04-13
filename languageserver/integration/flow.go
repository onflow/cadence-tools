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
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/onflow/flowkit/accounts"
	"github.com/onflow/flowkit/transactions"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flowkit"
	"github.com/onflow/flowkit/config"
	"github.com/onflow/flowkit/gateway"
	"github.com/onflow/flowkit/output"
)

//go:generate go run github.com/vektra/mockery/cmd/mockery --name flowClient --filename mock_flow_test.go --inpkg
type flowClient interface {
	Initialize(state flowState, numberOfAccounts int) error
	Reload() error
	GetClientAccount(name string) *clientAccount
	GetActiveClientAccount() *clientAccount
	GetClientAccounts() []*clientAccount
	SetActiveClientAccount(name string) error
	ExecuteScript(location *url.URL, args []cadence.Value) (cadence.Value, error)
	DeployContract(address flow.Address, name string, location *url.URL) error
	SendTransaction(
		authorizers []flow.Address,
		location *url.URL,
		args []cadence.Value,
	) (*flow.Transaction, *flow.TransactionResult, error)
	GetAccount(address flow.Address) (*flow.Account, error)
	CreateAccount() (*clientAccount, error)
	getState() flowState
	getConfigPath() string
}

var _ flowClient = &flowkitClient{}

type clientAccount struct {
	*flow.Account
	Name   string
	Active bool
	Key    *accounts.Key
}

var names = []string{
	"Alice", "Bob", "Charlie",
	"Dave", "Eve", "Faythe",
	"Grace", "Heidi", "Ivan",
	"Judy", "Michael", "Niaj",
	"Olivia", "Oscar", "Peggy",
	"Rupert", "Sybil", "Ted",
	"Victor", "Walter",
}

type flowkitClient struct {
	services      flowkit.Services
	loader        flowkit.ReaderWriter
	state         flowState
	accounts      []*clientAccount
	activeAccount *clientAccount
}

func newFlowkitClient(loader flowkit.ReaderWriter) *flowkitClient {
	return &flowkitClient{
		loader: loader,
	}
}

func (f *flowkitClient) Initialize(state flowState, numberOfAccounts int) error {
	f.state = state
	logger := output.NewStdoutLogger(output.NoneLog)

	acc, err := state.getState().EmulatorServiceAccount()
	if err != nil {
		return err
	}

	var emulator gateway.Gateway
	// try connecting to already running local emulator
	emulator, err = gateway.NewGrpcGateway(config.EmulatorNetwork)
	if err != nil || emulator.Ping() != nil { // fallback to hosted emulator if error
		pk, _ := acc.Key.PrivateKey()
		emulator = gateway.NewEmulatorGateway(&gateway.EmulatorKey{
			PublicKey: (*pk).PublicKey(),
			SigAlgo:   acc.Key.SigAlgo(),
			HashAlgo:  acc.Key.HashAlgo(),
		})
	}

	f.services = flowkit.NewFlowkit(state.getState(), config.EmulatorNetwork, emulator, logger)
	if numberOfAccounts > len(names) || numberOfAccounts <= 0 {
		return fmt.Errorf(fmt.Sprintf("only possible to create between 1 and %d accounts", len(names)))
	}

	// create base accounts
	f.accounts = make([]*clientAccount, 0)
	for i := 0; i < numberOfAccounts; i++ {
		_, err := f.CreateAccount()
		if err != nil {
			return err
		}
	}

	f.accounts = append(f.accounts, f.accountsFromState()...)

	f.accounts[0].Active = true // make first active by default
	f.activeAccount = f.accounts[0]

	return nil
}

func (f *flowkitClient) getState() flowState {
	return f.state
}

func (f *flowkitClient) getConfigPath() string {
	return f.state.getConfigPath()
}

func (f *flowkitClient) Reload() error {
	err := f.state.Load(f.getConfigPath())
	if err != nil {
		return err
	}
	return nil
}

func (f *flowkitClient) GetClientAccount(name string) *clientAccount {
	for _, account := range f.accounts {
		if account.Name == name {
			return account
		}
	}
	return nil
}

func (f *flowkitClient) GetClientAccounts() []*clientAccount {
	return f.accounts
}

func (f *flowkitClient) SetActiveClientAccount(name string) error {
	activeAcc := f.GetActiveClientAccount()
	if activeAcc != nil {
		activeAcc.Active = false
	}

	account := f.GetClientAccount(name)
	if account == nil {
		return fmt.Errorf(fmt.Sprintf("account with a name %s not found", name))
	}

	account.Active = true
	f.activeAccount = account
	return nil
}

func (f *flowkitClient) GetActiveClientAccount() *clientAccount {
	return f.activeAccount
}

func (f *flowkitClient) ExecuteScript(
	location *url.URL,
	args []cadence.Value,
) (cadence.Value, error) {
	code, err := f.loader.ReadFile(location.Path)
	if err != nil {
		return nil, err
	}

	codeFilename, err := resolveFilename(f.getConfigPath(), location.Path)
	if err != nil {
		return nil, err
	}

	return f.services.ExecuteScript(
		context.Background(),
		flowkit.Script{
			Code:     code,
			Args:     args,
			Location: codeFilename,
		},
		flowkit.LatestScriptQuery,
	)
}

func (f *flowkitClient) DeployContract(
	address flow.Address,
	_ string,
	location *url.URL,
) error {
	code, err := f.loader.ReadFile(location.Path)
	if err != nil {
		return err
	}

	codeFilename, err := resolveFilename(f.getConfigPath(), location.Path)
	if err != nil {
		return err
	}

	signer, err := f.createSigner(address)
	if err != nil {
		return err
	}

	// TODO: Remove this code
	// This is a temporary workaround to prevent an error thrown by flowkit
	// Changes in https://github.com/onflow/flow-cli/pull/1247 add contracts
	// to the state when AddContract is called, but this causes an error to
	// be thrown here because LS-managed accounts are not in the state.
	//
	// This code adds the account to the state before deploying the contract
	f.state.getState().Accounts().AddOrUpdate(signer)

	_, _, err = f.services.AddContract(
		context.Background(),
		signer,
		flowkit.Script{
			Code:     code,
			Location: codeFilename,
		},
		flowkit.UpdateExistingContract(true),
	)
	return err
}

func (f *flowkitClient) SendTransaction(
	authorizers []flow.Address,
	location *url.URL,
	args []cadence.Value,
) (*flow.Transaction, *flow.TransactionResult, error) {
	code, err := f.loader.ReadFile(location.Path)
	if err != nil {
		return nil, nil, err
	}

	service, err := f.state.getState().EmulatorServiceAccount()
	if err != nil {
		return nil, nil, err
	}

	codeFilename, err := resolveFilename(f.getConfigPath(), location.Path)
	if err != nil {
		return nil, nil, err
	}

	authAccs := make([]accounts.Account, len(authorizers))
	for i, auth := range authorizers {
		signer, err := f.createSigner(auth)
		if err != nil {
			return nil, nil, err
		}

		authAccs[i] = *signer
		if err != nil {
			return nil, nil, err
		}
	}

	return f.services.SendTransaction(
		context.Background(),
		transactions.AccountRoles{
			Proposer:    *service,
			Authorizers: authAccs,
			Payer:       *service,
		},
		flowkit.Script{
			Code:     code,
			Args:     args,
			Location: codeFilename,
		},
		flow.DefaultTransactionGasLimit,
	)
}

func (f *flowkitClient) GetAccount(address flow.Address) (*flow.Account, error) {
	return f.services.GetAccount(context.Background(), address)
}

func (f *flowkitClient) CreateAccount() (*clientAccount, error) {
	service, err := f.state.getState().EmulatorServiceAccount()
	if err != nil {
		return nil, err
	}
	serviceKey, err := service.Key.PrivateKey()
	if err != nil {
		return nil, err
	}

	account, _, err := f.services.CreateAccount(
		context.Background(),
		service,
		[]accounts.PublicKey{{
			Public:   (*serviceKey).PublicKey(),
			Weight:   flow.AccountKeyWeightThreshold,
			SigAlgo:  crypto.ECDSA_P256,
			HashAlgo: crypto.SHA3_256,
		}},
	)
	if err != nil {
		return nil, err
	}

	nextIndex := len(f.GetClientAccounts())
	if nextIndex > len(names) {
		return nil, fmt.Errorf(fmt.Sprintf("account limit of %d reached", len(names)))
	}

	clientAccount := &clientAccount{
		Account: account,
		Name:    names[nextIndex],
	}
	f.accounts = append(f.accounts, clientAccount)

	return clientAccount, nil
}

// accountsFromState extracts all the account defined by user in configuration.
// if account doesn't exist on the chain we are connecting to
// we skip it since we don't have a way to automatically create it.
func (f *flowkitClient) accountsFromState() []*clientAccount {
	accounts := make([]*clientAccount, 0)
	for _, acc := range *f.state.getState().Accounts() {
		account, err := f.services.GetAccount(context.Background(), acc.Address)
		if err != nil {
			// we skip user configured accounts that weren't already created on-chain
			// by user because we can't guarantee addresses are available
			continue
		}

		key := acc.Key
		accounts = append(accounts, &clientAccount{
			Account: account,
			Name:    fmt.Sprintf("%s [flow.json]", acc.Name),
			Key:     &key,
		})
	}

	return accounts
}

// createSigner creates a new flowkit account used for signing but using the key of the existing account.
func (f *flowkitClient) createSigner(address flow.Address) (*accounts.Account, error) {
	var account *clientAccount
	for _, acc := range f.accounts {
		if acc.Address == address {
			account = acc
		}
	}
	if account == nil {
		return nil, fmt.Errorf(fmt.Sprintf("account with address %s not found in the list of accounts", address))
	}

	var accountKey accounts.Key
	if account.Key != nil {
		accountKey = *account.Key
	} else { // default to service account if key not set
		service, err := f.state.getState().EmulatorServiceAccount()
		if err != nil {
			return nil, err
		}
		accountKey = service.Key
	}

	return &accounts.Account{
		Address: address,
		Key:     accountKey,
		Name:    fmt.Sprintf("%s [LS]", account.Name),
	}, nil
}

// Helpers
//

// resolveFilename helper converts the transaction file to a relative location to config file
func resolveFilename(configPath string, path string) (string, error) {
	if filepath.Dir(configPath) == "." { // if flow.json is passed as relative use current dir
		workPath, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Rel(workPath, path)
	}

	filename, err := filepath.Rel(filepath.Dir(configPath), path)
	if err != nil {
		return "", err
	}

	return filename, nil
}
