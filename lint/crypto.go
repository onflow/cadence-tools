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

import _ "embed"

// CryptoContractCode is the source code of the Crypto contract.
//
// It is a verbatim copy of the contract in flow-core-contracts,
// https://github.com/onflow/flow-core-contracts/blob/master/contracts/Crypto.cdc
//
// The contract is copied instead of loaded from the flow-core-contracts Go package,
// because that package depends on the Flow Go SDK,
// which transitively depends on github.com/onflow/crypto,
// which requires cgo, which is not available for WASM.
//
// TestCryptoContractCode checks that this copy is still identical
// to the contract in the required flow-core-contracts version.
//
//go:embed Crypto.cdc
var CryptoContractCode string
