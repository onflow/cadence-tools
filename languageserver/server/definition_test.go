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

	getDefinition := func(line, character uint32) *protocol.Location {
		t.Helper()

		location, err := server.Definition(
			nil,
			&protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
				Position: protocol.Position{
					Line:      line,
					Character: character,
				},
			},
		)
		require.NoError(t, err)
		return location
	}

	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 27},
				End:   protocol.Position{Line: 1, Character: 30},
			},
		},
		getDefinition(3, 34),
	)

	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 29},
				End:   protocol.Position{Line: 2, Character: 30},
			},
		},
		getDefinition(3, 38),
	)
}

func TestDefinitionMemberImported(t *testing.T) {
	t.Parallel()

	const fooCode = `
      access(all) contract Foo {
          access(all) let a: Int

          access(all) struct S {
              access(all) fun foo() {}
          }

          init() {
              self.a = 1
          }
      }
    `

	const counterCode = `
      import "Foo"

      access(all) contract Counter {
          access(all) fun run() {
              let a = Foo.a
              let s = Foo.S()
              s.foo()
          }
      }
    `

	server, err := NewServer()
	require.NoError(t, err)

	const fooURI = protocol.DocumentURI("file:///Foobar/cadence/contracts/Foo.cdc")

	err = server.SetOptions(
		WithStringImportResolver(func(_ string, location common.StringLocation) (string, error) {
			if location == "Foo" {
				return fooCode, nil
			}
			return "", nil
		}),
		WithLocationToURIResolver(func(_ string, location common.Location) protocol.DocumentURI {
			if loc, ok := location.(common.StringLocation); ok && loc == "Foo" {
				return fooURI
			}
			return ""
		}),
	)
	require.NoError(t, err)

	server.projectIdentity = staticProjectIdentity("foobar")

	counterURI := protocol.DocumentURI("file:///Foobar/cadence/contracts/Counter.cdc")
	_, err = server.getDiagnostics(counterURI, counterCode, 1, func(*protocol.LogMessageParams) {})
	require.NoError(t, err)

	getDefinition := func(line, character uint32) *protocol.Location {
		t.Helper()

		location, err := server.Definition(
			nil,
			&protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
				Position: protocol.Position{
					Line:      line,
					Character: character,
				},
			},
		)
		require.NoError(t, err)
		return location
	}

	// `a` in `let a = Foo.a` on line 5
	// =>
	// `a` in `let a: Int` in Foo.cdc line 2
	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 26},
				End:   protocol.Position{Line: 2, Character: 27},
			},
		},
		getDefinition(5, 26),
	)

	// `foo` in `s.foo()` on line 7
	// =>
	// `foo` in `fun foo()` in Foo.cdc line 5
	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 5, Character: 30},
				End:   protocol.Position{Line: 5, Character: 33},
			},
		},
		getDefinition(7, 16),
	)

	// `Foo` in `let s = Foo.S()` on line 6
	// =>
	// `Foo` in `contract Foo` in Foo.cdc line 1
	assert.Equal(t,
		&protocol.Location{
			URI: fooURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 1, Character: 27},
				End:   protocol.Position{Line: 1, Character: 30},
			},
		},
		getDefinition(6, 22),
	)

	// `s` in `let s = Foo.S()` on line 6
	// =>
	// `s` in `let s = Foo.S()` in Counter.cdc line 6
	assert.Equal(t,
		&protocol.Location{
			URI: counterURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 6, Character: 18},
				End:   protocol.Position{Line: 6, Character: 19},
			},
		},
		getDefinition(6, 18),
	)

	// `s` in `s.foo()` on line 7
	// =>
	// `s` in `let s = Foo.S()` in Counter.cdc line 6
	assert.Equal(t,
		&protocol.Location{
			URI: counterURI,
			Range: protocol.Range{
				Start: protocol.Position{Line: 6, Character: 18},
				End:   protocol.Position{Line: 6, Character: 19},
			},
		},
		getDefinition(7, 14),
	)
}
