/*
 * Cadence - The resource-oriented smart contract programming language
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
	"net/url"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CommandsRoutePerClosestFlowJSON(t *testing.T) {
	fs := afero.NewMemMapFs()
	loader := &afero.Afero{Fs: fs}

	require.NoError(t, loader.MkdirAll("/w/a", 0o755))
	require.NoError(t, loader.MkdirAll("/w/b", 0o755))
	require.NoError(t, loader.WriteFile("/w/a/flow.json", []byte("{}"), 0o644))
	require.NoError(t, loader.WriteFile("/w/b/flow.json", []byte("{}"), 0o644))
	require.NoError(t, loader.WriteFile("/w/a/script.cdc", []byte("pub fun main() {}"), 0o644))
	require.NoError(t, loader.WriteFile("/w/b/script.cdc", []byte("pub fun main() {}"), 0o644))

	mgr := NewConfigManager(loader, true, 0, "")

	// Pre-insert mock clients keyed by config path so ResolveClientForPath returns them without loading
	mockA := &mockFlowClient{}
	mockB := &mockFlowClient{}
	mgr.mu.Lock()
	mgr.clients["/w/a/flow.json"] = mockA
	mgr.clients["/w/b/flow.json"] = mockB
	mgr.mu.Unlock()

	// Integration instance not required for commands; ensure no dead code remains
	cmds := commands{cfg: mgr}

	// executeScript in /w/a should use mockA
	aURL, _ := url.Parse("file:///w/a/script.cdc")
	val, _ := cadence.NewString("ok-a")
	// commands.executeScript expects a JSON string containing JSON-CDC
	jsonCdc := `[{ "type": "String", "value": "x" }]`
	marshaledArg, _ := json.Marshal(jsonCdc)
	var argRaw = json.RawMessage(marshaledArg)
	xVal, _ := cadence.NewString("x")
	mockA.On("ExecuteScript", aURL, []cadence.Value{xVal}).Return(val, nil)

	locA, _ := json.Marshal("file:///w/a/script.cdc")
	res, err := cmds.executeScript(locA, argRaw)
	assert.NoError(t, err)
	assert.Equal(t, "Result: "+val.String(), res)

	// deployContract in /w/b should use mockB
	bURL, _ := url.Parse("file:///w/b/script.cdc")
	nameJSON, _ := json.Marshal("NFT")
	signerJSON, _ := json.Marshal("")
	// Active account on B
	mockB.On("GetActiveClientAccount").Return(&clientAccount{Account: &flow.Account{Address: flow.HexToAddress("0x1")}, Name: "bob"})
	mockB.On("DeployContract", flow.HexToAddress("0x1"), "NFT", bURL).Return(nil)

	locB, _ := json.Marshal("file:///w/b/script.cdc")
	out, err := cmds.deployContract(locB, nameJSON, signerJSON)
	assert.NoError(t, err)
	assert.Equal(t, "Contract NFT has been deployed to account bob", out)
}
