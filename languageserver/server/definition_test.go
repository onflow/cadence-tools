/*
 * Cadence languageserver - The Cadence language server
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

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"

	"github.com/onflow/cadence-tools/languageserver/protocol"
)

func TestDefinition(t *testing.T) {
	t.Parallel()

	const fooCode = `
      access(all) contract Foo {
          access(all) struct S {}
      }
    `

	const counterCode = `
      import "Foo"
      access(all) contract Counter {
          access(all) fun foo(_ : Foo.S) {}
      }
    `

	server, err := NewServer()
	require.NoError(t, err)

	const fooURI = protocol.DocumentURI("file:///Foobar/cadence/contracts/Foo.cdc")

	err = server.SetOptions(
		WithStringImportResolver(
			func(_ string, location common.StringLocation) (string, error) {
				if location == "Foo" {
					return fooCode, nil
				}
				return "", nil
			},
		),
		WithLocationToURIResolver(
			func(_ string, location common.Location) protocol.DocumentURI {
				if loc, ok := location.(common.StringLocation); ok && loc == "Foo" {
					return fooURI
				}
				return ""
			},
		),
	)
	require.NoError(t, err)

	server.projectIdentity = staticProjectIdentity("foobar")

	counterURI := protocol.DocumentURI("file:///Foobar/cadence/contracts/Counter.cdc")
	_, err = server.getDiagnostics(counterURI, counterCode, 1, func(*protocol.LogMessageParams) {})
	require.NoError(t, err)

	fooLoc, err := server.Definition(nil, &protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
		Position:     protocol.Position{Line: 3, Character: 34},
	})
	require.NoError(t, err)

	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 27},
				End:   protocol.Position{Line: 1, Character: 30},
			},
		},
		fooLoc,
	)

	sLoc, err := server.Definition(nil, &protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
		Position:     protocol.Position{Line: 3, Character: 38},
	})
	require.NoError(t, err)

	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 29},
				End:   protocol.Position{Line: 2, Character: 30},
			},
		},
		sLoc,
	)
}
