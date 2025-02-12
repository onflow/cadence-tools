//go:build wasm
// +build wasm

/*
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"syscall/js"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/cadence-tools/languageserver/server"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
)

const globalFunctionNamePrefix = "CADENCE_LANGUAGE_SERVER"

func globalFunctionName(id int, suffix string) string {
	return fmt.Sprintf("__%s_%d_%s__", globalFunctionNamePrefix, id, suffix)
}

func main() {
	done := make(chan struct{}, 0)
	id := 0
	startFunctionName := fmt.Sprintf("__%s_start__", globalFunctionNamePrefix)

	js.Global().Set(
		startFunctionName,
		js.FuncOf(func(this js.Value, args []js.Value) any {
			id += 1
			go start(id)
			return id
		}),
	)
	<-done
}

func start(id int) {
	logger := log.New(os.Stderr, fmt.Sprintf("CLS %d: ", id), log.LstdFlags)

	defer func() {
		if r := recover(); r != nil {
			logger.Printf("Recovered: %s\n", r)
			panic(r)
		}
	}()

	logger.Println("Starting ...")

	global := js.Global()

	writeObject := func(obj any) error {
		// The object / message is sent to the JS environment
		// by serializing it to JSON and calling a global function

		serialized, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		res := global.Call(globalFunctionName(id, "toClient"), string(serialized))
		if !(res.IsNull() || res.IsUndefined()) {
			return fmt.Errorf("CLS %d: toClient failed: %s", id, res)
		}

		return nil
	}

	readObject := func(v any) (err error) {
		// Set up a wait group which allows blocking this function
		// until the JS environment calls back

		var wg sync.WaitGroup
		wg.Add(1)

		var result string

		// Provide the JS environment a function it can call once
		// to write to the language server: Set the function,
		// and ensure it is removed when it is called

		toServerFunctionName := globalFunctionName(id, "toServer")
		global.Set(
			toServerFunctionName,
			js.FuncOf(func(this js.Value, args []js.Value) any {
				defer func() {
					global.Delete(toServerFunctionName)
					wg.Done()
				}()

				errValue := args[0]
				if !(errValue.IsNull() || errValue.IsUndefined()) {
					err = fmt.Errorf("CLS %d: toServer failed: %s", id, errValue)
					return nil
				}

				result = args[1].String()

				return nil
			}),
		)

		// Wait until the callback function above was called by the JS environment
		// to resolve the read

		wg.Wait()

		if err != nil {
			return err
		}

		// The JS environment sent an object / message as a JSON string,
		// deserialize it

		return json.Unmarshal([]byte(result), v)
	}

	onServerClose := func() error {
		res := global.Call(globalFunctionName(id, "onServerClose"))
		if !(res.IsNull() || res.IsUndefined()) {
			return fmt.Errorf("CLS %d: onServerClose failed: %s", id, res)
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	global.Set(
		globalFunctionName(id, "onClientClose"),
		js.FuncOf(func(this js.Value, args []js.Value) any {
			cancel()
			return nil
		}),
	)

	stream := server.NewObjectStream(
		writeObject,
		readObject,
		onServerClose,
	)

	addressImportResolver := func(location common.AddressLocation) (code string, err error) {
		getAddressCodeFunc := global.Get(globalFunctionName(id, "getAddressCode"))
		if getAddressCodeFunc.IsNull() || getAddressCodeFunc.IsUndefined() {
			return "", fmt.Errorf("CLS %d: getAddressCode not defined", id)
		}

		res := getAddressCodeFunc.Invoke(location.String())
		if res.IsNull() || res.IsUndefined() {
			return "", fmt.Errorf("CLS %d: getAddressCode failed: %s", id, res)
		}
		return res.String(), nil
	}

	stringImportResolver := func(location common.StringLocation) (code string, err error) {
		getStringCodeFunc := global.Get(globalFunctionName(id, "getStringCode"))
		if getStringCodeFunc.IsNull() || getStringCodeFunc.IsUndefined() {
			return "", fmt.Errorf("CLS %d: getStringCode not defined", id)
		}

		res := getStringCodeFunc.Invoke(location.String())
		if res.IsNull() || res.IsUndefined() {
			return "", fmt.Errorf("CLS %d: getStringCode failed: %s", id, res)
		}
		return res.String(), nil
	}

	identifierImportResolver := func(location common.IdentifierLocation) (code string, err error) {
		if location == stdlib.CryptoContractLocation {
			return string(coreContracts.Crypto()), nil
		}
		return "", fmt.Errorf("CLS %d: unknown identifier location: %s", id, location)
	}

	languageServer, err := server.NewServer()
	if err != nil {
		panic(err)
	}

	err = languageServer.SetOptions(
		server.WithAddressImportResolver(addressImportResolver),
		server.WithStringImportResolver(stringImportResolver),
		server.WithIdentifierImportResolver(identifierImportResolver),
	)
	if err != nil {
		panic(err)
	}

	select {
	case <-languageServer.Start(stream):
		logger.Println("Disconnected")
	case <-ctx.Done():
		err = languageServer.Stop()
		if err != nil {
			logger.Printf("Cancellation failed: %s", err)
			return
		}
		logger.Println("Cancelled successfully")
	}
}
