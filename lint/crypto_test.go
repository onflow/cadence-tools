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

package lint_test

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/cadence/common"

	"github.com/onflow/cadence-tools/lint"
)

// TestCryptoContractCode checks that the copy of the Crypto contract in this repository
// is still identical to the contract in the required flow-core-contracts version.
//
// The contract is copied instead of loaded from the flow-core-contracts Go package,
// because that package depends on the Flow Go SDK,
// which transitively depends on github.com/onflow/crypto,
// which requires cgo, which is not available for WASM.
func TestCryptoContractCode(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		string(coreContracts.Crypto()),
		lint.CryptoContractCode,
	)
}

func TestAnalyzeCryptoContractImport(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	const code = `
      import Crypto

      access(all)
      contract Foo {

          access(all)
          fun hash(_ data: [UInt8]): [UInt8] {
              return Crypto.hash(data, algorithm: HashAlgorithm.SHA3_256)
          }
      }
    `

	err := os.WriteFile(
		path.Join(directory, "A.0000000000000001.Foo.cdc"),
		[]byte(code),
		0o644,
	)
	require.NoError(t, err)

	var errs []error

	linter := lint.NewLinter(lint.Config{
		PrintError: func(_ *lint.Linter, err error, _ common.Location) {
			errs = append(errs, err)
		},
	})

	linter.AnalyzeDirectory(directory)

	require.Empty(t, errs)
}
