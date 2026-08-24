//go:build !wasm
// +build !wasm

// The network-based analysis needs the Flow SDK's gRPC access client,
// which transitively depends on github.com/onflow/crypto.
// That package requires cgo, which is not available for WASM.

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
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/onflow/flow-go-sdk"
	grpcAccess "github.com/onflow/flow-go-sdk/access/grpc"
)

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
