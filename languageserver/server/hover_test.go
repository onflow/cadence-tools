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

	"github.com/onflow/cadence-tools/languageserver/protocol"
)

func TestHover(t *testing.T) {
	t.Parallel()

	t.Run("no docstring", func(t *testing.T) {
		server, err := NewServer()
		require.NoError(t, err)

		const code = `
          access(all) fun test() {
              let foo = 1
          }
        `

		uri := protocol.DocumentURI("file:///test.cdc")

		_, err = server.getDiagnostics(uri, code, 1, func(*protocol.LogMessageParams) {})
		require.NoError(t, err)

		hover, err := server.Hover(
			nil,
			&protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: 2, Character: 19},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, hover)

		assert.Equal(
			t,
			&protocol.Hover{
				Range: protocol.Range{
					Start: protocol.Position{Line: 2, Character: 18},
					End:   protocol.Position{Line: 2, Character: 21},
				},
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: "**Type**\n\n```cadence\nInt\n```\n",
				},
			},
			hover,
		)
	})

	t.Run("docstring with param and return annotations", func(t *testing.T) {
		server, err := NewServer()
		require.NoError(t, err)

		const code = `
          /// Adds two numbers.
          ///
          /// @param a: The first number
          /// @param b: The second number
          /// @return The sum
          access(all) fun add(a: Int, b: Int): Int {
              return a + b
          }

          access(all) fun test() {
              let result = add(a: 1, b: 2)
          }
        `

		uri := protocol.DocumentURI("file:///test2.cdc")

		_, err = server.getDiagnostics(uri, code, 1, func(*protocol.LogMessageParams) {})
		require.NoError(t, err)

		// Hover over the `add` call
		// Line 11 (0-indexed): "              let result = add(a: 1, b: 2)"
		// "add" starts at column 27
		hover, err := server.Hover(
			nil,
			&protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri},
				Position:     protocol.Position{Line: 11, Character: 27},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, hover)

		assert.Equal(
			t,
			protocol.MarkupContent{
				Kind: protocol.Markdown,
				Value: "**Type**\n\n```cadence\n" +
					"fun (a: Int, b: Int): Int\n```\n" +
					"\n**Documentation**\n\n" +
					"Adds two numbers.\n\n" +
					"**Parameters**\n\n" +
					"- `a`: The first number\n" +
					"- `b`: The second number\n\n" +
					"**Returns** The sum\n",
			},
			hover.Contents,
		)
	})
}
