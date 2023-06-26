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
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
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

// LogCollectionHook can be attached to zerolog.Logger objects, in order
// to aggregate the log messages in a string slice, containing only the
// string message.
type LogCollectionHook struct {
	Logs []string
}

var _ zerolog.Hook = &LogCollectionHook{}

// NewLogCollectionHook initializes and returns a *LogCollectionHook
func NewLogCollectionHook() *LogCollectionHook {
	return &LogCollectionHook{
		Logs: make([]string, 0),
	}
}

func (h *LogCollectionHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
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

// ImportResolver is used to resolve and get the source code for imports.
// Must be provided by the user of the TestRunner.
type ImportResolver func(location common.Location) (string, error)

// FileResolver is used to resolve and get local files.
// Returns the content of the file as a string.
// Must be provided by the user of the TestRunner.
type FileResolver func(path string) (string, error)

// TestRunner runs tests.
type TestRunner struct {

	// importResolver is used to resolve imports of the *test script*.
	// Note: This doesn't resolve the imports for the code that is being tested.
	// i.e: the code that is submitted to the blockchain.
	// Users need to use configurations to set the import mapping for the testing code.
	importResolver ImportResolver

	// fileResolver is used to resolve local files.
	//
	fileResolver FileResolver

	testRuntime runtime.Runtime

	coverageReport *runtime.CoverageReport

	// logger is injected as the program logger for the script
	// environment.
	logger zerolog.Logger

	// logCollection is a hook attached in the program logger of
	// the script environment, in order to aggregate and expose
	// log messages from test cases and contracts.
	logCollection *LogCollectionHook

	backend *EmulatorBackend
}

func NewTestRunner() *TestRunner {
	logCollectionHook := NewLogCollectionHook()
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	output.FormatMessage = func(i interface{}) string {
		msg := i.(string)
		return strings.Replace(
			msg,
			"Cadence log:",
			aurora.Colorize("LOG:", aurora.BlueFg|aurora.BoldFm).String(),
			1,
		)
	}
	logger := zerolog.New(output).With().Timestamp().Logger().Hook(logCollectionHook)
	return &TestRunner{
		testRuntime:   runtime.NewInterpreterRuntime(runtime.Config{}),
		logCollection: logCollectionHook,
		logger:        logger,
	}
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

	for _, funcDecl := range program.Program.FunctionDeclarations() {
		funcName := funcDecl.Identifier.Identifier

		if !strings.HasPrefix(funcName, testFunctionPrefix) {
			continue
		}

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
	return r.logCollection.Logs
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

func (r *TestRunner) parseCheckAndInterpret(script string) (*interpreter.Program, *interpreter.Interpreter, error) {
	config := runtime.Config{
		CoverageReport: r.coverageReport,
	}
	env := runtime.NewBaseInterpreterEnvironment(config)

	ctx := runtime.Context{
		Interface:   newScriptEnvironment(r.logger),
		Location:    testScriptLocation,
		Environment: env,
	}
	if r.coverageReport != nil {
		r.coverageReport.ExcludeLocation(stdlib.CryptoCheckerLocation)
		r.coverageReport.ExcludeLocation(stdlib.TestContractLocation)
		r.coverageReport.ExcludeLocation(testScriptLocation)
		ctx.CoverageReport = r.coverageReport
	}

	// Checker configs
	env.CheckerConfig.ImportHandler = r.checkerImportHandler(ctx)
	env.CheckerConfig.ContractValueHandler = contractValueHandler

	// Interpreter configs
	env.InterpreterConfig.ImportLocationHandler = r.interpreterImportHandler(ctx)

	// It is safe to use the test-runner's environment as the standard library handler
	// in the test framework, since it is only used for value conversions (i.e: values
	// returned from blockchain to the test script)
	env.InterpreterConfig.ContractValueHandler = r.interpreterContractValueHandler(env)

	// TODO: The default injected fields handler only supports 'address' locations.
	//   However, during tests, it is possible to get non-address locations. e.g: file paths.
	//   Thus, need to properly handle them. Make this nil for now.
	env.InterpreterConfig.InjectedCompositeFieldsHandler = nil

	program, err := r.testRuntime.ParseAndCheckProgram([]byte(script), ctx)
	if err != nil {
		return nil, nil, err
	}

	// TODO: validate test function signature
	//   e.g: no return values, no arguments, etc.

	// Set the storage after checking, because `ParseAndCheckProgram` clears the storage.
	env.InterpreterConfig.Storage = runtime.NewStorage(ctx.Interface, nil)

	_, inter, err := env.Interpret(
		ctx.Location,
		program,
		nil,
	)

	if err != nil {
		return nil, nil, err
	}

	return program, inter, nil
}

func (r *TestRunner) checkerImportHandler(ctx runtime.Context) sema.ImportHandlerFunc {
	return func(
		checker *sema.Checker,
		importedLocation common.Location,
		importRange ast.Range,
	) (sema.Import, error) {
		var elaboration *sema.Elaboration
		switch importedLocation {
		case stdlib.CryptoCheckerLocation:
			cryptoChecker := stdlib.CryptoChecker()
			elaboration = cryptoChecker.Elaboration

		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			elaboration = testChecker.Elaboration

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
	constructorType, constructorArgumentLabels := sema.CompositeLikeConstructorType(
		checker.Elaboration,
		declaration,
		compositeType,
	)

	return stdlib.StandardLibraryValue{
		Name:           declaration.Identifier.Identifier,
		Type:           constructorType,
		DocString:      declaration.DocString,
		Kind:           declaration.DeclarationKind(),
		Position:       &declaration.Identifier.Pos,
		ArgumentLabels: constructorArgumentLabels,
	}
}

func (r *TestRunner) interpreterContractValueHandler(
	stdlibHandler stdlib.StandardLibraryHandler,
) interpreter.ContractValueHandlerFunc {
	return func(
		inter *interpreter.Interpreter,
		compositeType *sema.CompositeType,
		constructorGenerator func(common.Address) *interpreter.HostFunctionValue,
		invocationRange ast.Range,
	) interpreter.ContractValue {

		switch compositeType.Location {
		case stdlib.CryptoCheckerLocation:
			contract, err := stdlib.NewCryptoContract(
				inter,
				constructorGenerator(common.Address{}),
				invocationRange,
			)
			if err != nil {
				panic(err)
			}
			return contract

		case stdlib.TestContractLocation:
			r.backend = NewEmulatorBackend(
				r.fileResolver,
				stdlibHandler,
				r.coverageReport,
			)
			contract, err := stdlib.GetTestContractType().
				NewTestContract(
					inter,
					r.backend,
					constructorGenerator(common.Address{}),
					invocationRange,
				)
			if err != nil {
				panic(err)
			}
			return contract

		default:
			// During tests, imported contracts can be constructed using the constructor,
			// similar to structs. Therefore, generate a constructor function.
			return constructorGenerator(common.Address{})
		}
	}
}

func (r *TestRunner) interpreterImportHandler(ctx runtime.Context) interpreter.ImportLocationHandlerFunc {
	return func(inter *interpreter.Interpreter, location common.Location) interpreter.Import {
		switch location {
		case stdlib.CryptoCheckerLocation:
			cryptoChecker := stdlib.CryptoChecker()
			program := interpreter.ProgramFromChecker(cryptoChecker)
			subInterpreter, err := inter.NewSubInterpreter(program, location)
			if err != nil {
				panic(err)
			}
			return interpreter.InterpreterImport{
				Interpreter: subInterpreter,
			}

		case stdlib.TestContractLocation:
			testChecker := stdlib.GetTestContractType().Checker
			program := interpreter.ProgramFromChecker(testChecker)
			subInterpreter, err := inter.NewSubInterpreter(program, location)
			if err != nil {
				panic(err)
			}
			return interpreter.InterpreterImport{
				Interpreter: subInterpreter,
			}

		default:
			addressLocation, ok := location.(common.AddressLocation)
			if ok {
				account, _ := r.backend.blockchain.GetAccount(
					flow.Address(addressLocation.Address),
				)
				programCode := account.Contracts[addressLocation.Name]
				env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})
				newCtx := runtime.Context{
					Interface:   newScriptEnvironment(zerolog.Nop()),
					Location:    addressLocation,
					Environment: env,
				}
				env.CheckerConfig.ImportHandler = func(
					checker *sema.Checker,
					importedLocation common.Location,
					importRange ast.Range,
				) (sema.Import, error) {
					addressLoc, ok := importedLocation.(common.AddressLocation)
					if !ok {
						return nil, fmt.Errorf("no address location given")
					}
					account, _ := r.backend.blockchain.GetAccount(
						flow.Address(addressLoc.Address),
					)
					programCode := account.Contracts[addressLoc.Name]
					program, err := env.ParseAndCheckProgram(
						programCode, addressLoc, true,
					)
					if err != nil {
						panic(err)
					}

					return sema.ElaborationImport{
						Elaboration: program.Elaboration,
					}, nil
				}
				program, err := r.testRuntime.ParseAndCheckProgram(programCode, newCtx)
				if err != nil {
					panic(err)
				}

				subInterpreter, err := inter.NewSubInterpreter(program, addressLocation)
				if err != nil {
					panic(err)
				}
				return interpreter.InterpreterImport{
					Interpreter: subInterpreter,
				}
			}
			importedProgram, importedElaboration, err := r.parseAndCheckImport(location, ctx)
			if err != nil {
				panic(err)
			}

			program := &interpreter.Program{
				Program:     importedProgram,
				Elaboration: importedElaboration,
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
}

// newScriptEnvironment creates an environment for test scripts to run.
// Leverages the functionality of FVM.
func newScriptEnvironment(logger zerolog.Logger) environment.Environment {
	vm := fvm.NewVirtualMachine()
	ctx := fvm.NewContext(fvm.WithLogger(zerolog.Nop()))
	snapshotTree := testutil.RootBootstrappedLedger(vm, ctx)
	environmentParams := environment.DefaultEnvironmentParams()
	environmentParams.ProgramLoggerParams = environment.ProgramLoggerParams{
		Logger:                logger,
		CadenceLoggingEnabled: true,
		MetricsReporter:       environment.NoopMetricsReporter{},
	}

	return environment.NewScriptEnvironmentFromStorageSnapshot(
		environmentParams,
		snapshotTree,
	)
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

	code, err := r.importResolver(location)
	if err != nil {
		return nil, nil, err
	}

	// Create a new (child) context, with new environment.

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	ctx := runtime.Context{
		Interface:   startCtx.Interface,
		Location:    location,
		Environment: env,
	}

	env.CheckerConfig.ImportHandler = func(
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
			return nil, fmt.Errorf("nested imports are not supported")
		}
	}

	env.CheckerConfig.ContractValueHandler = contractValueHandler

	program, err := r.testRuntime.ParseAndCheckProgram([]byte(code), ctx)

	if err != nil {
		return nil, nil, err
	}

	return program.Program, program.Elaboration, nil
}

// PrettyPrintResults is a utility function to pretty print the test results.
func PrettyPrintResults(results Results, scriptPath string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Test results: %q\n", scriptPath)
	for _, result := range results {
		sb.WriteString(PrettyPrintResult(result.TestName, result.Error))
		sb.WriteRune('\n')
	}
	return sb.String()
}

func PrettyPrintResult(funcName string, err error) string {
	if err == nil {
		return fmt.Sprintf("- PASS: %s", funcName)
	}

	// Indent the error messages
	errString := strings.ReplaceAll(err.Error(), "\n", "\n\t\t\t")

	return fmt.Sprintf("- FAIL: %s\n\t\t%s", funcName, errString)
}
