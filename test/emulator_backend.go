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

package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/flow-emulator/adapters"
	"github.com/onflow/flow-emulator/convert"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-emulator/types"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"
	sdkTest "github.com/onflow/flow-go-sdk/test"
	fvmCrypto "github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

// The "\x00helper/" prefix is used in order to prevent
// conflicts with user-defined scripts/transactions.
const helperFilePrefix = "\x00helper/"

// The number of predefined accounts that are created
// upon initialization of EmulatorBackend.
const initialAccountsNumber = 20

var _ stdlib.Blockchain = &EmulatorBackend{}

type systemClock struct {
	TimeDelta int64
}

func (sc systemClock) Now() time.Time {
	return time.Now().Add(time.Second * time.Duration(sc.TimeDelta)).UTC()
}

func newSystemClock() *systemClock {
	return &systemClock{}
}

// EmulatorBackend is the emulator-backed implementation of the interpreter.TestFramework.
type EmulatorBackend struct {
	blockchain *emulator.Blockchain

	// blockOffset is the offset for the sequence number of the next transaction.
	// This is equal to the number of transactions in the current block.
	// Must be reset once the block is committed.
	blockOffset uint64

	// accountKeys is a mapping of account addresses with their keys.
	accountKeys map[common.Address]map[string]keyInfo

	stdlibHandler stdlib.StandardLibraryHandler

	// logCollection is a hook attached in the server logger, in order
	// to aggregate and expose log messages from the blockchain.
	logCollection *logCollectionHook

	// clock allows manipulating the blockchain's clock.
	clock *systemClock

	// accounts is a mapping of account addresses to the underlying
	// stdlib.Account value.
	accounts map[common.Address]*stdlib.Account

	// fileResolver is used to retrieve the Cadence source code,
	// given a relative path.
	fileResolver FileResolver

	// contracts is a mapping of contract identifiers to their
	// deployed account address.
	contracts map[string]common.Address
}

type keyInfo struct {
	accountKey *sdk.AccountKey
	signer     crypto.Signer
}

var chain = flow.MonotonicEmulator.Chain()

var commonContracts = emulator.NewCommonContracts(chain)

var systemContracts = func() []common.AddressLocation {
	chainContracts := systemcontracts.SystemContractsForChain(chain.ChainID())
	serviceAddress := chain.ServiceAddress().HexWithPrefix()
	contracts := map[string]string{
		"FlowServiceAccount":         serviceAddress,
		"FlowToken":                  chainContracts.FlowToken.Address.HexWithPrefix(),
		"FungibleToken":              chainContracts.FungibleToken.Address.HexWithPrefix(),
		"FungibleTokenMetadataViews": chainContracts.FungibleToken.Address.HexWithPrefix(),
		"FlowFees":                   chainContracts.FlowFees.Address.HexWithPrefix(),
		"FlowStorageFees":            serviceAddress,
		"FlowClusterQC":              serviceAddress,
		"FlowDKG":                    serviceAddress,
		"FlowEpoch":                  serviceAddress,
		"FlowIDTableStaking":         serviceAddress,
		"FlowStakingCollection":      serviceAddress,
		"LockedTokens":               serviceAddress,
		"NodeVersionBeacon":          serviceAddress,
		"StakingProxy":               serviceAddress,
		"NonFungibleToken":           serviceAddress,
		"MetadataViews":              serviceAddress,
		"ViewResolver":               serviceAddress,
		"RandomBeaconHistory":        serviceAddress,
		"EVM":                        serviceAddress,
		"FungibleTokenSwitchboard":   chainContracts.FungibleToken.Address.HexWithPrefix(),
		"Burner":                     serviceAddress,
	}

	locations := make([]common.AddressLocation, 0)
	for name, address := range contracts {
		addr, _ := common.HexToAddress(address)
		locations = append(locations, common.AddressLocation{
			Address: addr,
			Name:    name,
		})
	}

	return locations
}()

func NewEmulatorBackend(
	logger zerolog.Logger,
	stdlibHandler stdlib.StandardLibraryHandler,
	coverageReport *runtime.CoverageReport,
) *EmulatorBackend {
	logCollectionHook := newLogCollectionHook()
	var blockchain *emulator.Blockchain
	if coverageReport != nil {
		excludeCommonLocations(coverageReport)
		blockchain = newBlockchain(
			logger,
			logCollectionHook,
			emulator.WithCoverageReport(coverageReport),
		)
	} else {
		blockchain = newBlockchain(
			logger,
			logCollectionHook,
		)
	}
	clock := newSystemClock()
	blockchain.SetClock(clock)

	emulatorBackend := &EmulatorBackend{
		blockchain:    blockchain,
		blockOffset:   0,
		accountKeys:   map[common.Address]map[string]keyInfo{},
		stdlibHandler: stdlibHandler,
		logCollection: logCollectionHook,
		clock:         clock,
		contracts:     map[string]common.Address{},
		accounts:      map[common.Address]*stdlib.Account{},
	}
	emulatorBackend.bootstrapAccounts()

	return emulatorBackend
}

func (e *EmulatorBackend) RunScript(
	inter *interpreter.Interpreter,
	code string,
	args []interpreter.Value,
) *stdlib.ScriptResult {

	arguments := make([][]byte, 0, len(args))
	for _, arg := range args {
		exportedValue, err := runtime.ExportValue(arg, inter, interpreter.EmptyLocationRange)
		if err != nil {
			return &stdlib.ScriptResult{
				Error: err,
			}
		}

		encodedArg, err := json.Encode(exportedValue)
		if err != nil {
			return &stdlib.ScriptResult{
				Error: err,
			}
		}

		arguments = append(arguments, encodedArg)
	}

	code = e.replaceImports(code)

	result, err := e.blockchain.ExecuteScript([]byte(code), arguments)
	if err != nil {
		return &stdlib.ScriptResult{
			Error: err,
		}
	}

	if result.Error != nil {
		return &stdlib.ScriptResult{
			Error: result.Error,
		}
	}

	staticType := runtime.ImportType(inter, result.Value.Type())
	expectedType, err := inter.ConvertStaticToSemaType(staticType)
	if err != nil {
		return &stdlib.ScriptResult{
			Error: err,
		}
	}
	value, err := runtime.ImportValue(
		inter,
		interpreter.EmptyLocationRange,
		e.stdlibHandler,
		result.Value,
		expectedType,
	)
	if err != nil {
		return &stdlib.ScriptResult{
			Error: err,
		}
	}

	return &stdlib.ScriptResult{
		Value: value,
	}
}

func (e *EmulatorBackend) ServiceAccount() (*stdlib.Account, error) {
	serviceKey := e.blockchain.ServiceKey()
	serviceAddress := serviceKey.Address
	serviceSigner, err := serviceKey.Signer()
	if err != nil {
		return nil, err
	}

	publicKey := serviceSigner.PublicKey().Encode()
	encodedPublicKey := string(publicKey)
	accountKey := serviceKey.AccountKey()

	// Store the generated key and signer info.
	// This info is used to sign transactions.
	e.accountKeys[common.Address(serviceAddress)] = map[string]keyInfo{
		encodedPublicKey: {
			accountKey: accountKey,
			signer:     serviceSigner,
		},
	}

	return &stdlib.Account{
		Address: common.Address(serviceAddress),
		PublicKey: &stdlib.PublicKey{
			PublicKey: publicKey,
			SignAlgo: fvmCrypto.CryptoToRuntimeSigningAlgorithm(
				serviceSigner.PublicKey().Algorithm(),
			),
		},
	}, nil
}

func (e *EmulatorBackend) CreateAccount() (*stdlib.Account, error) {
	// Also generate the keys. So that users don't have to do this in two steps.
	// Store the generated keys, so that it could be looked-up, given the address.

	keyGen := sdkTest.AccountKeyGenerator()
	accountKey, signer := keyGen.NewWithSigner()

	sdkAdapter := adapters.NewSDKAdapter(zerolog.DefaultContextLogger, e.blockchain)
	address, err := sdkAdapter.CreateAccount(context.Background(), []*sdk.AccountKey{accountKey}, nil)
	if err != nil {
		return nil, err
	}

	publicKey := accountKey.PublicKey.Encode()
	encodedPublicKey := string(publicKey)

	// Store the generated key and signer info.
	// This info is used to sign transactions.
	e.accountKeys[common.Address(address)] = map[string]keyInfo{
		encodedPublicKey: {
			accountKey: accountKey,
			signer:     signer,
		},
	}

	account := &stdlib.Account{
		Address: common.Address(address),
		PublicKey: &stdlib.PublicKey{
			PublicKey: publicKey,
			SignAlgo:  fvmCrypto.CryptoToRuntimeSigningAlgorithm(accountKey.PublicKey.Algorithm()),
		},
	}
	e.accounts[account.Address] = account

	return account, nil
}

func (e *EmulatorBackend) GetAccount(
	address interpreter.AddressValue,
) (*stdlib.Account, error) {
	account, ok := e.accounts[address.ToAddress()]
	if !ok {
		return nil, fmt.Errorf("account with address: %s not found", address.Hex())
	}
	return account, nil
}

func (e *EmulatorBackend) AddTransaction(
	inter *interpreter.Interpreter,
	code string,
	authorizers []common.Address,
	signers []*stdlib.Account,
	args []interpreter.Value,
) error {

	code = e.replaceImports(code)

	tx := e.newTransaction(code, authorizers)

	for _, arg := range args {
		exportedValue, err := runtime.ExportValue(arg, inter, interpreter.EmptyLocationRange)
		if err != nil {
			return err
		}

		err = tx.AddArgument(exportedValue)
		if err != nil {
			return err
		}
	}

	err := e.signTransaction(tx, signers)
	if err != nil {
		return err
	}

	flowTx := convert.SDKTransactionToFlow(*tx)
	err = e.blockchain.AddTransaction(*flowTx)
	if err != nil {
		return err
	}

	// Increment the transaction sequence number offset for the current block.
	e.blockOffset++

	return nil
}

func (e *EmulatorBackend) ExecuteNextTransaction() *stdlib.TransactionResult {
	result, err := e.blockchain.ExecuteNextTransaction()

	if err != nil {
		// If the returned error is `emulator.PendingBlockTransactionsExhaustedError`,
		// that means there are no transactions to execute.
		// Hence, return a nil result.
		if _, ok := err.(*types.PendingBlockTransactionsExhaustedError); ok {
			return nil
		}

		return &stdlib.TransactionResult{
			Error: err,
		}
	}

	if result.Error != nil {
		return &stdlib.TransactionResult{
			Error: result.Error,
		}
	}

	return &stdlib.TransactionResult{}
}

func (e *EmulatorBackend) CommitBlock() error {
	// Reset the transaction offset for the current block.
	e.blockOffset = 0

	_, err := e.blockchain.CommitBlock()
	return err
}

func (e *EmulatorBackend) DeployContract(
	inter *interpreter.Interpreter,
	name string,
	path string,
	args []interpreter.Value,
) error {

	const deployContractTransactionTemplate = `
        transaction(%s) {
            prepare(signer: auth(AddContract) &Account) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex()%s)
            }
        }
	`

	// Retrieve the contract source code, by using the given path.
	code, err := e.fileResolver(path)
	if err != nil {
		panic(err)
	}
	code = e.replaceImports(code)

	hexEncodedCode := hex.EncodeToString([]byte(code))

	cadenceArgs := make([]cadence.Value, 0, len(args))

	var txArgsBuilder, addArgsBuilder strings.Builder

	for i, arg := range args {
		cadenceArg, err := runtime.ExportValue(arg, inter, interpreter.EmptyLocationRange)
		if err != nil {
			return err
		}

		if i > 0 {
			txArgsBuilder.WriteString(", ")
		}

		txArgsBuilder.WriteString(fmt.Sprintf("arg%d: %s", i, cadenceArg.Type().ID()))
		addArgsBuilder.WriteString(fmt.Sprintf(", arg%d", i))

		cadenceArgs = append(cadenceArgs, cadenceArg)
	}

	script := fmt.Sprintf(
		deployContractTransactionTemplate,
		txArgsBuilder.String(),
		name,
		hexEncodedCode,
		addArgsBuilder.String(),
	)

	address, ok := e.contracts[name]
	if !ok {
		return fmt.Errorf("could not find the address of contract: %s", name)
	}
	account, ok := e.accounts[address]
	if !ok {
		return fmt.Errorf("could not find an account with address: %s", address)
	}
	tx := e.newTransaction(script, []common.Address{account.Address})

	for _, arg := range cadenceArgs {
		err := tx.AddArgument(arg)
		if err != nil {
			return err
		}
	}

	err = e.signTransaction(tx, []*stdlib.Account{account})
	if err != nil {
		return err
	}

	flowTx := convert.SDKTransactionToFlow(*tx)
	err = e.blockchain.AddTransaction(*flowTx)
	if err != nil {
		return err
	}

	// Increment the transaction sequence number offset for the current block.
	e.blockOffset++

	result := e.ExecuteNextTransaction()
	if result.Error != nil {
		return result.Error
	}

	return e.CommitBlock()
}

// Logs returns all the log messages from the blockchain.
func (e *EmulatorBackend) Logs() []string {
	return e.logCollection.Logs
}

func (e *EmulatorBackend) StandardLibraryHandler() stdlib.StandardLibraryHandler {
	return e.stdlibHandler
}

func (e *EmulatorBackend) Reset(height uint64) {
	err := e.blockchain.RollbackToBlockHeight(height)
	if err != nil {
		panic(err)
	}

	// Reset the transaction offset.
	e.blockOffset = 0
}

// Events returns all the emitted events up until the latest block,
// optionally filtered by event type.
func (e *EmulatorBackend) Events(
	inter *interpreter.Interpreter,
	eventType interpreter.StaticType,
) interpreter.Value {
	latestBlock, err := e.blockchain.GetLatestBlock()
	if err != nil {
		panic(err)
	}

	latestBlockHeight := latestBlock.Header.Height
	height := uint64(0)
	values := make([]interpreter.Value, 0)

	var eventTypeString string
	switch eventType := eventType.(type) {
	case nil:
		eventTypeString = ""
	case *interpreter.CompositeStaticType:
		eventTypeString = eventType.String()
	default:
		panic(errors.NewUnreachableError())
	}

	for height <= latestBlockHeight {
		events, err := e.blockchain.GetEventsByHeight(height, eventTypeString)
		if err != nil {
			panic(err)
		}

		sdkEvents, err := convert.FlowEventsToSDK(events)
		if err != nil {
			panic(err)
		}

		for _, event := range sdkEvents {
			value, err := runtime.ImportValue(
				inter,
				interpreter.EmptyLocationRange,
				e.stdlibHandler,
				event.Value,
				nil,
			)
			if err != nil {
				panic(err)
			}
			values = append(values, value)

		}
		height += 1
	}

	arrayType := interpreter.NewVariableSizedStaticType(
		inter,
		interpreter.NewPrimitiveStaticType(
			inter,
			interpreter.PrimitiveStaticTypeAnyStruct,
		),
	)

	return interpreter.NewArrayValue(
		inter,
		interpreter.EmptyLocationRange,
		arrayType,
		common.ZeroAddress,
		values...,
	)
}

// MoveTime Moves the time of the Blockchain's clock, by the
// given time delta, in the form of seconds.
func (e *EmulatorBackend) MoveTime(timeDelta int64) {
	e.clock.TimeDelta += timeDelta
	e.blockchain.SetClock(e.clock)

	err := e.CommitBlock()
	if err != nil {
		panic(err)
	}
}

// CreateSnapshot Creates a snapshot of the blockchain, at the
// current ledger state, with the given name.
func (e *EmulatorBackend) CreateSnapshot(name string) error {
	return e.blockchain.CreateSnapshot(name)
}

// LoadSnapshot Loads a snapshot of the blockchain, with the
// given name, and updates the current ledger
// state.
func (e *EmulatorBackend) LoadSnapshot(name string) error {
	return e.blockchain.LoadSnapshot(name)
}

// Creates the number of predefined accounts that will be used
// for deploying the contracts under testing.
func (e *EmulatorBackend) bootstrapAccounts() {
	serviceAcc, err := e.ServiceAccount()
	if err != nil {
		panic(err)
	}
	e.accounts[serviceAcc.Address] = serviceAcc

	for i := 0; i < initialAccountsNumber; i++ {
		_, err := e.CreateAccount()
		if err != nil {
			panic(err)
		}
	}
}

func (e *EmulatorBackend) newTransaction(code string, authorizers []common.Address) *sdk.Transaction {
	serviceKey := e.blockchain.ServiceKey()

	sequenceNumber := serviceKey.SequenceNumber + e.blockOffset

	tx := sdk.NewTransaction().
		SetScript([]byte(code)).
		SetProposalKey(serviceKey.Address, serviceKey.Index, sequenceNumber).
		SetPayer(serviceKey.Address)

	for _, authorizer := range authorizers {
		tx = tx.AddAuthorizer(sdk.Address(authorizer))
	}

	return tx
}

func (e *EmulatorBackend) signTransaction(
	tx *sdk.Transaction,
	signerAccounts []*stdlib.Account,
) error {

	// Sign transaction with each signer
	// Note: Following logic is borrowed from the flow-ft.

	serviceKey := e.blockchain.ServiceKey()

	for i := len(signerAccounts) - 1; i >= 0; i-- {
		signerAccount := signerAccounts[i]
		if signerAccount.Address == common.Address(serviceKey.Address) {
			// skip payload signing for service account, since we always
			// sign the envelope with the service account just below
			continue
		}

		publicKey := signerAccount.PublicKey.PublicKey
		accountKeys := e.accountKeys[signerAccount.Address]
		keyInfo := accountKeys[string(publicKey)]

		err := tx.SignPayload(sdk.Address(signerAccount.Address), 0, keyInfo.signer)
		if err != nil {
			return err
		}
	}

	serviceSigner, err := serviceKey.Signer()
	if err != nil {
		return err
	}

	err = tx.SignEnvelope(serviceKey.Address, 0, serviceSigner)
	if err != nil {
		return err
	}

	return nil
}

func (e *EmulatorBackend) replaceImports(code string) string {
	program, err := parser.ParseProgram(nil, []byte(code), parser.Config{})
	if err != nil {
		panic(err)
	}

	sb := strings.Builder{}
	importDeclEnd := 0
	for _, importDeclaration := range program.ImportDeclarations() {
		prevImportDeclEnd := importDeclEnd
		importDeclEnd = importDeclaration.EndPos.Offset + 1

		location, ok := importDeclaration.Location.(common.StringLocation)
		if !ok {
			// keep the import statement it as-is
			sb.WriteString(code[prevImportDeclEnd:importDeclEnd])
			continue
		}

		var address common.Address
		var found bool
		if len(importDeclaration.Identifiers) > 0 {
			address, found = e.contracts[importDeclaration.Identifiers[0].Identifier]
		} else {
			address, found = e.contracts[location.String()]
		}
		if !found {
			// keep import statement it as-is
			sb.WriteString(code[prevImportDeclEnd:importDeclEnd])
			continue
		}

		var importStr string
		if strings.Contains(importDeclaration.String(), "from") {
			importStr = fmt.Sprintf("0x%s", address)
		} else {
			// Imports of the form `import "FungibleToken"` should be
			// expanded to `import FungibleToken from 0xee82856bf20e2aa6`
			importStr = fmt.Sprintf("%s from 0x%s", location, address)
		}

		locationStart := importDeclaration.LocationPos.Offset

		sb.WriteString(code[prevImportDeclEnd:locationStart])
		sb.WriteString(importStr)
	}

	sb.WriteString(code[importDeclEnd:])

	return sb.String()
}

// newBlockchain returns an emulator blockchain for testing.
func newBlockchain(
	logger zerolog.Logger,
	hook *logCollectionHook,
	opts ...emulator.Option,
) *emulator.Blockchain {
	testLogger := logger.With().Timestamp().
		Logger().Hook(hook).Level(zerolog.InfoLevel)

	b, err := emulator.New(
		append(
			[]emulator.Option{
				emulator.WithStorageLimitEnabled(false),
				emulator.WithServerLogger(testLogger),
				emulator.Contracts(commonContracts),
				emulator.WithChainID(chain.ChainID()),
			},
			opts...,
		)...,
	)
	if err != nil {
		panic(err)
	}

	return b
}

// excludeCommonLocations excludes the common contracts from appearing
// in the coverage report, as they skew the coverage metrics.
func excludeCommonLocations(coverageReport *runtime.CoverageReport) {
	for _, location := range systemContracts {
		coverageReport.ExcludeLocation(location)
	}
	for _, contract := range commonContracts {
		address, _ := common.HexToAddress(contract.Address.String())
		location := common.AddressLocation{
			Address: address,
			Name:    contract.Name,
		}
		coverageReport.ExcludeLocation(location)
	}
}
