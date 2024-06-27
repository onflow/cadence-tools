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
	"strings"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/rs/zerolog"
)

var _ stdlib.TestFramework = &TestFrameworkProvider{}

type TestFrameworkProvider struct {
	// fileResolver is used to resolve local files.
	fileResolver FileResolver

	stdlibHandler stdlib.StandardLibraryHandler

	coverageReport *runtime.CoverageReport

	emulatorBackend *EmulatorBackend
}

func (tf *TestFrameworkProvider) ReadFile(path string) (string, error) {
	// These are the scripts/transactions used by the
	// BlockchainHelpers file.
	if strings.HasPrefix(path, helperFilePrefix) {
		filename := strings.TrimPrefix(path, helperFilePrefix)
		switch filename {
		case "mint_flow.cdc":
			return string(MintFlowTransaction), nil
		case "get_flow_balance.cdc":
			return string(GetFlowBalance), nil
		case "get_current_block_height.cdc":
			return string(GetCurrentBlockHeight), nil
		case "burn_flow.cdc":
			return string(BurnFlow), nil
		}
	}

	if tf.fileResolver == nil {
		return "", FileResolverNotProvidedError{}
	}

	return tf.fileResolver(path)
}

func (tf *TestFrameworkProvider) EmulatorBackend() stdlib.Blockchain {
	return tf.emulatorBackend
}

func NewTestFrameworkProvider(
	logger zerolog.Logger,
	fileResolver FileResolver,
	stdlibHandler stdlib.StandardLibraryHandler,
	coverageReport *runtime.CoverageReport,
) stdlib.TestFramework {
	return &TestFrameworkProvider{
		fileResolver:   fileResolver,
		stdlibHandler:  stdlibHandler,
		coverageReport: coverageReport,
		emulatorBackend: NewEmulatorBackend(
			logger,
			stdlibHandler,
			coverageReport,
		),
	}
}
