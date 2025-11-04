/*
 * Cadence test - The Cadence test framework
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

package test

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/ast"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/parser"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"
	"github.com/onflow/flow-emulator/emulator"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence-tools/test/helpers"
)

// This Provides utility methods to easily run test-scripts.
// Example use-case:
//   - To run all tests in a script:
//         RunTests("source code")
//   - To run a single test method in a script:
//         RunTest("source code", "testMethodName")
//
// It is assumed that all test methods start with the 'test' prefix.

const testFunctionPrefix = "test"

const setupFunctionName = "setup"

const tearDownFunctionName = "tearDown"

const beforeEachFunctionName = "beforeEach"

const afterEachFunctionName = "afterEach"

var testScriptLocation = common.NewScriptLocation(nil, []byte("test"))

var quotedLog = regexp.MustCompile("\"(.*)\"")

type Results []Result

type Result struct {
	TestName string
	Error    error
}

// logCollectionHook can be attached to zerolog.Logger objects, in order
// to aggregate the log messages in a string slice, containing only the
// string message.
type logCollectionHook struct {
	Logs []string
}

var _ zerolog.Hook = &logCollectionHook{}

// newLogCollectionHook initializes and returns a *LogCollectionHook
func newLogCollectionHook() *logCollectionHook {
	return &logCollectionHook{
		Logs: make([]string, 0),
	}
}

func (h *logCollectionHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if level != zerolog.NoLevel {
		logMsg := strings.Replace(
			msg,
			"LOG:",
			"",
			1,
		)
		match := quotedLog.FindStringSubmatch(logMsg)
		// Only logs with strings are quoted, eg:
		// DBG LOG: "setup successful"
		// We strip the quotes, to keep only the raw value.
		// Other logs may not contain quotes, eg:
		// DBG LOG: flow.AccountCreated(address: 0x01cf0e2f2f715450)
		if len(match) > 0 {
			logMsg = match[1]
		}
		h.Logs = append(h.Logs, logMsg)
	}
}

// ImportResolver resolves and returns the source code for imports.
// Receives the current ChainID to allow network-aware resolution.
type ImportResolver func(chainID flow.ChainID, location common.Location) (string, error)

// FileResolver is used to resolve and get local files.
// Returns the content of the file as a string.
// Must be provided by the user of the TestRunner.
type FileResolver func(path string) (string, error)

// TestRunner runs tests.
type TestRunner struct {
	logger zerolog.Logger

	// importResolver is used to resolve imports of the *test script*.
	// Note: This doesn't resolve the imports for the code that is being tested.
	// i.e: the code that is submitted to the blockchain.
	// Users need to use configurations to set the import mapping for the testing code.
	importResolver ImportResolver

	// fileResolver is used to resolve local files.
	fileResolver FileResolver

	testRuntime runtime.Runtime

	coverageReport *runtime.CoverageReport

	// randomSeed is used for randomized test case execution.
	randomSeed int64

	contracts map[string]common.Address

	testFramework stdlib.TestFramework

	backend *EmulatorBackend

	// fork configuration
	forkConfig *ForkConfig
}

func NewTestRunner() *TestRunner {
	return &TestRunner{
		logger:    zerolog.Nop(),
		contracts: baseContracts(),
	}
}

func (r *TestRunner) WithLogger(logger zerolog.Logger) *TestRunner {
	r.logger = logger
	return r
}

func (r *TestRunner) WithImportResolver(importResolver ImportResolver) *TestRunner {
	r.importResolver = importResolver
	return r
}

func (r *TestRunner) WithFileResolver(fileResolver FileResolver) *TestRunner {
	r.fileResolver = fileResolver
	return r
}

func (r *TestRunner) WithCoverageReport(coverageReport *runtime.CoverageReport) *TestRunner {
	r.coverageReport = coverageReport
	return r
}

func (r *TestRunner) WithRandomSeed(seed int64) *TestRunner {
	r.randomSeed = seed
	return r
}

func (r *TestRunner) WithContracts(contracts map[string]common.Address) *TestRunner {
	for contract, address := range contracts {
		// We do not want to override the base configuration,
		// which includes the mapping for system/common contracts.
		r.contracts[contract] = address
	}
	return r
}

// ForkConfig configures a single forked environment for the entire test run.
type ForkConfig struct {
	ForkHost   string
	ForkHeight uint64       // Block height to fork from (lastest sealed if empty)
	ChainID    flow.ChainID // Chain ID to use (optional, will auto-detect if empty)
}

// WithFork enables fork mode with the given configuration.
func (r *TestRunner) WithFork(cfg ForkConfig) *TestRunner {
	r.forkConfig = &cfg
	return r
}

// RunTest runs a single test in the provided test script.
func (r *TestRunner) RunTest(script string, funcName string) (result *Result, err error) {
	defer func() {
		recoverPanics(func(internalErr error) {
			err = internalErr
		})
	}()

	_, inter, err := r.parseCheckAndInterpret(script)
	if err != nil {
		return nil, err
	}

	// Run test `setup()` before running the test function.
	err = r.runTestSetup(inter)
	if err != nil {
		return nil, err
	}

	// Run `beforeEach()` before running the test function.
	err = r.runBeforeEach(inter)
	if err != nil {
		return nil, err
	}

	_, testResult := inter.Invoke(funcName)

	// Run `afterEach()` after running the test function.
	err = r.runAfterEach(inter)
	if err != nil {
		return nil, err
	}

	// Run test `tearDown()` once running all test functions are completed.
	err = r.runTestTearDown(inter)

	return &Result{
		TestName: funcName,
		Error:    testResult,
	}, err
}

// RunTests runs all the tests in the provided test script.
func (r *TestRunner) RunTests(script string) (results Results, err error) {
	defer func() {
		recoverPanics(func(internalErr error) {
			err = internalErr
		})
	}()

	program, inter, err := r.parseCheckAndInterpret(script)
	if err != nil {
		return nil, err
	}

	results = make(Results, 0)

	// Run test `setup()` before test functions
	err = r.runTestSetup(inter)
	if err != nil {
		return nil, err
	}

	testCases := make([]*ast.FunctionDeclaration, 0)

	for _, funcDecl := range program.Program.FunctionDeclarations() {
		funcName := funcDecl.Identifier.Identifier

		if strings.HasPrefix(funcName, testFunctionPrefix) {
			testCases = append(testCases, funcDecl)
		}
	}
	if r.randomSeed > 0 {
		rng := rand.New(rand.NewSource(r.randomSeed))
		rng.Shuffle(len(testCases), func(i, j int) {
			testCases[i], testCases[j] = testCases[j], testCases[i]
		})
	}

	for _, funcDecl := range testCases {
		funcName := funcDecl.Identifier.Identifier

		// Run `beforeEach()` before running the test function.
		err = r.runBeforeEach(inter)
		if err != nil {
			return nil, err
		}

		testErr := r.invokeTestFunction(inter, funcName)

		// Run `afterEach()` after running the test function.
		err = r.runAfterEach(inter)
		if err != nil {
			return nil, err
		}

		results = append(results, Result{
			TestName: funcName,
			Error:    testErr,
		})
	}

	// Run test `tearDown()` once running all test functions are completed.
	err = r.runTestTearDown(inter)

	return results, err
}

func (r *TestRunner) GetTests(script string) ([]string, error) {
	program, _, err := r.parseCheckAndInterpret(script)
	if err != nil {
		return nil, err
	}

	tests := make([]string, 0)

	for _, funcDecl := range program.Program.FunctionDeclarations() {
		funcName := funcDecl.Identifier.Identifier

		if strings.HasPrefix(funcName, testFunctionPrefix) {
			tests = append(tests, funcName)
		}
	}

	return tests, nil
}

func (r *TestRunner) replaceImports(code string) string {
	return r.backend.replaceImports(code)
}

func (r *TestRunner) runTestSetup(inter *interpreter.Interpreter) error {
	if !hasSetup(inter) {
		return nil
	}

	return r.invokeTestFunction(inter, setupFunctionName)
}

func hasSetup(inter *interpreter.Interpreter) bool {
	return inter.Globals.Contains(setupFunctionName)
}

func (r *TestRunner) runTestTearDown(inter *interpreter.Interpreter) error {
	if !hasTearDown(inter) {
		return nil
	}

	return r.invokeTestFunction(inter, tearDownFunctionName)
}

func hasTearDown(inter *interpreter.Interpreter) bool {
	return inter.Globals.Contains(tearDownFunctionName)
}

func (r *TestRunner) runBeforeEach(inter *interpreter.Interpreter) error {
	if !hasBeforeEach(inter) {
		return nil
	}

	return r.invokeTestFunction(inter, beforeEachFunctionName)
}

func hasBeforeEach(inter *interpreter.Interpreter) bool {
	return inter.Globals.Contains(beforeEachFunctionName)
}

func (r *TestRunner) runAfterEach(inter *interpreter.Interpreter) error {
	if !hasAfterEach(inter) {
		return nil
	}

	return r.invokeTestFunction(inter, afterEachFunctionName)
}

func hasAfterEach(inter *interpreter.Interpreter) bool {
	return inter.Globals.Contains(afterEachFunctionName)
}

func (r *TestRunner) invokeTestFunction(inter *interpreter.Interpreter, funcName string) (err error) {
	// Individually fail each test-case for any internal error.
	defer func() {
		recoverPanics(func(internalErr error) {
			err = internalErr
		})
	}()

	_, err = inter.Invoke(funcName)
	return err
}

// Logs returns all the log messages from the script environment that
// test cases run. Unit tests run in this environment too, so the
// logs from their respective contracts, also appear in the resulting
// string slice.
func (r *TestRunner) Logs() []string {
	return r.backend.Logs()
}

func recoverPanics(onError func(error)) {
	r := recover()
	switch r := r.(type) {
	case nil:
		return
	case error:
		onError(r)
	default:
		onError(fmt.Errorf("%s", r))
	}
}

func (r *TestRunner) parseCheckAndInterpret(script string) (
	*interpreter.Program,
	*interpreter.Interpreter,
	error,
) {
	// TODO: move this eventually to the `NewTestRunner`
	env, ctx := r.initializeEnvironment()

	astProgram, err := parser.ParseProgram(nil, []byte(script), parser.Config{})
	if err != nil {
		return nil, nil, err
	}

	for _, funcDecl := range astProgram.FunctionDeclarations() {
		funcName := funcDecl.Identifier.Identifier

		if !strings.HasPrefix(funcName, testFunctionPrefix) {
			continue
		}

		if !funcDecl.ParameterList.IsEmpty() {
			return nil, nil, fmt.Errorf("test functions should have no arguments")
		}

		if funcDecl.ReturnTypeAnnotation != nil {
			return nil, nil, fmt.Errorf("test functions should have no return values")
		}
	}

	script = r.replaceImports(script)

	program, err := env.ParseAndCheckProgram([]byte(script), ctx.Location, false)
	if err != nil {
		return nil, nil, err
	}

	interpreterEnvironment := env.(*runtime.InterpreterEnvironment)
	_, inter, err := interpreterEnvironment.Interpret(
		ctx.Location,
		program,
		nil,
	)

	if err != nil {
		return nil, nil, err
	}

	return program, inter, nil
}

func (r *TestRunner) initializeEnvironment() (
	runtime.Environment,
	runtime.Context,
) {
	config := runtime.Config{
		CoverageReport: r.coverageReport,
	}

	env := runtime.NewBaseInterpreterEnvironment(config)

	r.testRuntime = runtime.NewRuntime(config)

	var backendOptions *BackendOptions
	if r.forkConfig != nil {
		backendOptions = &BackendOptions{
			ForkHost:   r.forkConfig.ForkHost,
			ForkHeight: r.forkConfig.ForkHeight,
			ChainID:    r.forkConfig.ChainID,
		}
	}

	r.testFramework = NewTestFrameworkProvider(
		r.logger,
		r.fileResolver,
		env,
		r.coverageReport,
		backendOptions,
	)
	backend, ok := r.testFramework.EmulatorBackend().(*EmulatorBackend)
	if !ok {
		panic(fmt.Errorf("failed to retrieve EmulatorBackend"))
	}
	backend.fileResolver = r.fileResolver
	backend.contracts = r.contracts
	r.backend = backend

	fvmEnv := r.backend.blockchain.NewScriptEnvironment()
	ctx := runtime.Context{
		Interface:   fvmEnv,
		Location:    testScriptLocation,
		Environment: env,
	}
	if r.coverageReport != nil {
		// Ensure system/common contracts are excluded before any parsing/instrumentation
		excludeCommonLocationsForChain(r.coverageReport, r.backend.chain)
		r.coverageReport.ExcludeLocation(stdlib.CryptoContractLocation)
		r.coverageReport.ExcludeLocation(stdlib.TestContractLocation)
		r.coverageReport.ExcludeLocation(helpers.BlockchainHelpersLocation)
		r.coverageReport.ExcludeLocation(testScriptLocation)
	}

	// Checker configs
	env.CheckingEnvironment.Config.ImportHandler = r.checkerImportHandler(ctx)
	env.CheckingEnvironment.Config.ContractValueHandler = contractValueHandler

	// Interpreter configs
	env.InterpreterConfig.ImportLocationHandler = r.interpreterImportHandler(ctx)

	// It is safe to use the test-runner's environment as the standard library handler
	// in the test framework, since it is only used for value conversions (i.e: values
	// returned from blockchain to the test script)
	env.InterpreterConfig.ContractValueHandler = r.interpreterContractValueHandler(env)

	env.Configure(
		ctx.Interface,
		runtime.NewCodesAndPrograms(),
		runtime.NewStorage(ctx.Interface.(environment.Environment), nil, nil, runtime.StorageConfig{}),
		nil,
		nil,
	)

	err := setupEVMEnvironment(r.backend.chain, fvmEnv, env)
	if err != nil {
		panic(err)
	}

	return env, ctx
}

func (r *TestRunner) checkerImportHandler(ctx runtime.Context) sema.ImportHandlerFunc {
	return func(
		checker *sema.Checker,
		importedLocation common.Location,
		importRange ast.Range,
	) (sema.Import, error) {
		var elaboration *sema.Elaboration
		switch importedLocation {

		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			elaboration = testChecker.Elaboration

		case helpers.BlockchainHelpersLocation:
			helpersChecker := helpers.BlockchainHelpersChecker()
			elaboration = helpersChecker.Elaboration

		default:
			_, importedElaboration, err := r.parseAndCheckImport(importedLocation, ctx)
			if err != nil {
				return nil, err
			}

			elaboration = importedElaboration
		}

		return sema.ElaborationImport{
			Elaboration: elaboration,
		}, nil
	}
}

func contractValueHandler(
	checker *sema.Checker,
	declaration *ast.CompositeDeclaration,
	compositeType *sema.CompositeType,
) sema.ValueDeclaration {
	_, constructorArgumentLabels := sema.CompositeLikeConstructorType(
		checker.Elaboration,
		declaration,
		compositeType,
	)

	// For composite types (e.g. contracts) that are deployed on
	// EmulatorBackend's blockchain, we have to declare the
	// value declaration as a composite. This is needed to access
	// nested types that are defined in the composite type,
	// e.g events / structs / resources / enums etc.
	return stdlib.StandardLibraryValue{
		Name:           declaration.Identifier.Identifier,
		Type:           compositeType,
		DocString:      declaration.DocString,
		Kind:           declaration.DeclarationKind(),
		Position:       &declaration.Identifier.Pos,
		ArgumentLabels: constructorArgumentLabels,
	}
}

func (r *TestRunner) interpreterContractValueHandler(
	_ stdlib.StandardLibraryHandler,
) interpreter.ContractValueHandlerFunc {
	return func(
		inter *interpreter.Interpreter,
		compositeType *sema.CompositeType,
		constructorGenerator func(common.Address) *interpreter.HostFunctionValue,
	) interpreter.ContractValue {

		switch compositeType.Location {
		case stdlib.TestContractLocation:
			constructor := constructorGenerator(common.Address{})
			contract, err := stdlib.GetTestContractType().
				NewTestContract(
					inter,
					r.testFramework,
					constructor,
				)
			if err != nil {
				panic(err)
			}
			return contract

		default:
			var storedValue interpreter.Value

			switch location := compositeType.Location.(type) {
			case common.AddressLocation:
				// All contracts are deployed on EmulatorBackend's
				// blockchain, so we construct a storage based on
				// its ledger.
				blockchainStorage := runtime.NewStorage(
					r.backend.blockchain.NewScriptEnvironment(),
					nil,
					nil,
					runtime.StorageConfig{},
				)
				storageMap := blockchainStorage.GetDomainStorageMap(
					inter,
					location.Address,
					common.StorageDomainContract,
					false,
				)
				if storageMap != nil {
					storedValue = storageMap.ReadValue(
						inter,
						interpreter.StringStorageMapKey(location.Name),
					)
				}

				// We need to store every slab of `blockchainStorage`
				// to the current environment's storage, so that
				// we can access fields & types.
				iterator, err := blockchainStorage.SlabIterator()
				if err != nil {
					panic(err)
				}
				storage := inter.Storage().(*runtime.Storage)

				for {
					id, slab := iterator()
					if id == atree.SlabIDUndefined {
						break
					}
					err := storage.Store(id, slab)
					if err != nil {
						panic(err)
					}
				}

				err = storage.Commit(inter, true)
				if err != nil {
					panic(err)
				}
			}

			if storedValue == nil {
				panic(
					errors.NewDefaultUserError(
						"failed to load contract: %s",
						compositeType.Location,
					),
				)
			}

			return storedValue.(*interpreter.CompositeValue)
		}
	}
}

func (r *TestRunner) interpreterImportHandler(ctx runtime.Context) interpreter.ImportLocationHandlerFunc {
	return func(inter *interpreter.Interpreter, location common.Location) interpreter.Import {
		var program *interpreter.Program
		switch location {

		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			program = interpreter.ProgramFromChecker(testChecker)

		case helpers.BlockchainHelpersLocation:
			helpersChecker := helpers.BlockchainHelpersChecker()
			program = interpreter.ProgramFromChecker(helpersChecker)

		default:
			importedProgram, importedElaboration, err := r.parseAndCheckImport(location, ctx)
			if err != nil {
				panic(err)
			}

			program = &interpreter.Program{
				Program:     importedProgram,
				Elaboration: importedElaboration,
			}
		}

		subInterpreter, err := inter.NewSubInterpreter(program, location)
		if err != nil {
			panic(err)
		}
		return interpreter.InterpreterImport{
			Interpreter: subInterpreter,
		}
	}
}

func (r *TestRunner) parseAndCheckImport(
	location common.Location,
	startCtx runtime.Context,
) (
	*ast.Program,
	*sema.Elaboration,
	error,
) {
	if r.importResolver == nil {
		return nil, nil, ImportResolverNotProvidedError{}
	}

	// Resolve code using current chain ID
	code, err := r.importResolver(r.backend.chain.ChainID(), location)
	if err != nil {
		addressLocation, ok := location.(common.AddressLocation)
		if ok {
			// System-defined contracts are obtained from
			// the blockchain.
			account, err := r.backend.blockchain.GetAccount(
				flow.Address(addressLocation.Address),
			)
			if err != nil {
				return nil, nil, err
			}
			code = string(account.Contracts[addressLocation.Name])
		} else {
			return nil, nil, err
		}
	}

	// Create a new (child) context, reuse the provided interface

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	ctx := runtime.Context{
		Interface:   startCtx.Interface,
		Location:    location,
		Environment: env,
	}

	env.CheckingEnvironment.Config.ImportHandler = func(
		checker *sema.Checker,
		importedLocation common.Location,
		importRange ast.Range,
	) (sema.Import, error) {
		switch importedLocation {
		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			elaboration := testChecker.Elaboration
			return sema.ElaborationImport{
				Elaboration: elaboration,
			}, nil

		default:
			addressLoc, ok := importedLocation.(common.AddressLocation)
			if !ok {
				return nil, fmt.Errorf("unable to import location: %s", importedLocation)
			}

			_, elaboration, err := r.parseAndCheckImport(addressLoc, ctx)
			if err != nil {
				return nil, err
			}

			return sema.ElaborationImport{
				Elaboration: elaboration,
			}, nil
		}
	}

	env.CheckingEnvironment.Config.ContractValueHandler = contractValueHandler

	fvmEnv, ok := startCtx.Interface.(environment.Environment)
	if !ok {
		panic(fmt.Errorf("failed to retrieve FVM Environment"))
	}
	err = setupEVMEnvironment(r.backend.chain, fvmEnv, env)
	if err != nil {
		panic(err)
	}

	code = r.replaceImports(code)
	program, err := r.testRuntime.ParseAndCheckProgram([]byte(code), ctx)

	if err != nil {
		return nil, nil, err
	}

	return program.Program, program.Elaboration, nil
}

func setupEVMEnvironment(
	ch flow.Chain,
	fvmEnv environment.Environment,
	runtimeEnv runtime.Environment,
) error {
	return evm.SetupEnvironment(
		ch.ChainID(),
		fvmEnv,
		runtimeEnv,
	)
}

func baseContracts() map[string]common.Address {
	contracts := make(map[string]common.Address, 0)
	ch := flow.MonotonicEmulator.Chain()
	serviceAddress := common.Address(ch.ServiceAddress())
	contracts["NonFungibleToken"] = serviceAddress
	contracts["MetadataViews"] = serviceAddress
	contracts["ViewResolver"] = serviceAddress
	for _, addressLocation := range systemAddressLocationsForChain(ch) {
		contract := addressLocation.Name
		address := addressLocation.Address
		contracts[contract] = address
	}
	for _, contractDescription := range emulator.NewCommonContracts(ch) {
		contract := contractDescription.Name
		address := common.Address(contractDescription.Address)
		contracts[contract] = address
	}

	return contracts
}

// PrettyPrintResults is a utility function to pretty print the test results.
func PrettyPrintResults(results Results, scriptPath string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Test results: %q\n", scriptPath)
	for _, result := range results {
		sb.WriteString(PrettyPrintResult(scriptPath, result.TestName, result.Error))
		sb.WriteRune('\n')
	}
	return sb.String()
}

func PrettyPrintResult(scriptPath, funcName string, err error) string {
	if err == nil {
		return fmt.Sprintf("- PASS: %s", funcName)
	}

	interErr := err.(interpreter.Error)

	// Replace script ID with actual file path
	errString := strings.ReplaceAll(
		err.Error(),
		interErr.Location.String(),
		scriptPath,
	)

	// Indent the error messages
	errString = strings.ReplaceAll(errString, "\n", "\n\t\t\t")

	return fmt.Sprintf("- FAIL: %s\n\t\t%s", funcName, errString)
}
