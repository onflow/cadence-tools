/*
 * Cadence lint - The Cadence linter
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

package lint

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/onflow/flow-go-sdk"
	grpcAccess "github.com/onflow/flow-go-sdk/access/grpc"
)

const LoadMode = analysis.NeedTypes | analysis.NeedExtendedElaboration

type Config struct {
	Analyzers  []*analysis.Analyzer
	Silent     bool
	UseColor   bool
	PrintError func(*Linter, error, common.Location)
}

type Linter struct {
	Config             Config
	errorPrettyPrinter pretty.ErrorPrettyPrinter
	Codes              map[common.Location][]byte
}

func NewLinter(config Config) *Linter {
	if config.PrintError == nil {
		config.PrintError = (*Linter).PrettyPrintError
	}

	return &Linter{
		Config:             config,
		errorPrettyPrinter: pretty.NewErrorPrettyPrinter(os.Stdout, config.UseColor),
		Codes:              map[common.Location][]byte{},
	}
}

func (l *Linter) PrettyPrintError(err error, location common.Location) {
	printErr := l.errorPrettyPrinter.PrettyPrintError(err, location, l.Codes)
	if printErr != nil {
		panic(printErr)
	}
}

func (l *Linter) AnalyzeAccount(address string, networkName string) {
	access, err := newFlowAccess(networkName)
	if err != nil {
		panic(err)
	}

	contractNames := map[common.Address][]string{}

	getContracts := func(flowAddress flow.Address) (map[string][]byte, error) {
		account, err := access.GetAccount(context.Background(), flowAddress)
		if err != nil {
			return nil, err
		}

		return account.Contracts, nil
	}

	flowAddress := flow.HexToAddress(address)
	commonAddress := common.Address(flowAddress)

	contracts, err := getContracts(flowAddress)
	if err != nil {
		panic(err)
	}

	locations := make([]common.Location, 0, len(contracts))
	for contractName := range contracts {
		location := common.AddressLocation{
			Address: commonAddress,
			Name:    contractName,
		}
		locations = append(locations, location)
	}

	analysisConfig := analysis.NewSimpleConfig(
		LoadMode,
		l.Codes,
		contractNames,
		func(address common.Address) (map[string][]byte, error) {
			return getContracts(flow.Address(address))
		},
	)

	l.analyze(analysisConfig, locations)
}

func (l *Linter) AnalyzeTransaction(transactionID flow.Identifier, networkName string) {
	access, err := newFlowAccess(networkName)
	if err != nil {
		panic(err)
	}

	contractNames := map[common.Address][]string{}

	getContracts := func(flowAddress flow.Address) (map[string][]byte, error) {
		account, err := access.GetAccount(context.Background(), flowAddress)
		if err != nil {
			return nil, err
		}

		return account.Contracts, nil
	}

	transactionLocation := common.TransactionLocation(transactionID)

	locations := []common.Location{
		transactionLocation,
	}

	transaction, err := access.GetTransaction(context.Background(), transactionID)
	if err != nil {
		panic(err)
	}

	l.Codes[transactionLocation] = transaction.Script

	analysisConfig := analysis.NewSimpleConfig(
		LoadMode,
		l.Codes,
		contractNames,
		func(address common.Address) (map[string][]byte, error) {
			return getContracts(flow.Address(address))
		},
	)
	l.analyze(analysisConfig, locations)
}

func newFlowAccess(networkName string) (*grpcAccess.Client, error) {
	networkMap := map[string]string{
		"mainnet":  grpcAccess.MainnetHost,
		"testnet":  grpcAccess.TestnetHost,
		"emulator": grpcAccess.EmulatorHost,
		"":         grpcAccess.EmulatorHost,
	}

	network := networkMap[networkName]
	if network == "" {
		var names []string
		for name := range networkMap {
			names = append(names, name)
		}
		sort.Strings(names)

		return nil, fmt.Errorf(
			"missing network name. expected one of: %s",
			strings.Join(names, ","),
		)
	}

	return grpcAccess.NewClient(
		network,
		grpcAccess.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(),
		),
	)
}

func (l *Linter) AnalyzeCSV(path string) {

	csvFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(csvFile)

	locations, contractNames := l.readCSV(csvFile)
	analysisConfig := analysis.NewSimpleConfig(
		LoadMode,
		l.Codes,
		contractNames,
		nil,
	)
	l.analyze(analysisConfig, locations)
}

func (l *Linter) AnalyzeDirectory(directory string) {

	entries, err := os.ReadDir(directory)
	if err != nil {
		panic(err)
	}

	locations, contractNames := l.readDirectoryEntries(directory, entries)
	analysisConfig := analysis.NewSimpleConfig(
		LoadMode,
		l.Codes,
		contractNames,
		nil,
	)
	l.analyze(analysisConfig, locations)
}

func (l *Linter) readDirectoryEntries(
	directory string,
	entries []os.DirEntry,
) (
	locations []common.Location,
	contractNames map[common.Address][]string,
) {
	contractNames = map[common.Address][]string{}

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() || path.Ext(name) != ".cdc" {
			continue
		}

		// Strip extension
		typeID := name[:len(name)-len(path.Ext(name))]

		location, qualifiedIdentifier, err := common.DecodeTypeID(nil, typeID)
		if err != nil {
			panic(fmt.Errorf("invalid location in file %q: %w", name, err))
		}

		identifierParts := strings.Split(qualifiedIdentifier, ".")
		if len(identifierParts) > 1 {
			panic(fmt.Errorf(
				"invalid location in file %q: invalid qualified identifier: %s",
				name,
				qualifiedIdentifier,
			))
		}

		rawCode, err := os.ReadFile(path.Join(directory, name))
		if err != nil {
			panic(fmt.Errorf("failed to read file %q: %w", name, err))
		}

		locations = append(locations, location)
		l.Codes[location] = rawCode

		if addressLocation, ok := location.(common.AddressLocation); ok {
			contractNames[addressLocation.Address] = append(
				contractNames[addressLocation.Address],
				addressLocation.Name,
			)
		}
	}

	return
}

func (l *Linter) analyze(
	config *analysis.Config,
	locations []common.Location,
) {
	programs := make(analysis.Programs, len(locations))

	log.Println("Loading ...")

	printErr := l.Config.PrintError

	for _, location := range locations {
		log.Printf("Loading %s", location.Description())

		err := programs.Load(config, location)
		if err != nil {
			if l.Config.Silent {
				log.Printf("Failed: %s", location.Description())
			} else {
				printErr(l, err, location)
			}
		}
	}

	var reportLock sync.Mutex

	report := func(diagnostic analysis.Diagnostic) {
		reportLock.Lock()
		defer reportLock.Unlock()

		printErr(
			l,
			diagnosticErr{diagnostic},
			diagnostic.Location,
		)
	}

	analyzers := l.Config.Analyzers
	if len(analyzers) > 0 {
		for _, location := range locations {
			program := programs[location]
			if program == nil {
				continue
			}

			log.Printf("Analyzing %s", location)

			program.Run(analyzers, report)
		}
	}
}

func (l *Linter) readCSV(
	r io.Reader,
) (
	locations []common.Location,
	contractNames map[common.Address][]string,
) {
	reader := csv.NewReader(r)

	contractNames = map[common.Address][]string{}

	var record []string
	for rowNumber := 1; ; rowNumber++ {
		var err error
		skip := record == nil
		record, err = reader.Read()
		if err == io.EOF {
			break
		}
		if skip {
			continue
		}

		location, qualifiedIdentifier, err := common.DecodeTypeID(nil, record[0])
		if err != nil {
			panic(fmt.Errorf("invalid location in row %d: %w", rowNumber, err))
		}
		identifierParts := strings.Split(qualifiedIdentifier, ".")
		if len(identifierParts) > 1 {
			panic(fmt.Errorf(
				"invalid location in row %d: invalid qualified identifier: %s",
				rowNumber,
				qualifiedIdentifier,
			))
		}

		code := record[1]

		locations = append(locations, location)
		l.Codes[location] = []byte(code)

		if addressLocation, ok := location.(common.AddressLocation); ok {
			contractNames[addressLocation.Address] = append(
				contractNames[addressLocation.Address],
				addressLocation.Name,
			)
		}
	}

	return
}
