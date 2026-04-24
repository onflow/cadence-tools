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

func TestCompletionDocString(t *testing.T) {
	t.Parallel()

	t.Run("member completion", func(t *testing.T) {
		server, err := NewServer()
		require.NoError(t, err)

		// Struct with a documented method; trigger member completion on an instance.
		// Use "f.b" so the parser creates a member access expression.
		const code = `
          access(all) struct Foo {
              /// Does something.
              ///
              /// @param x: The input
              /// @return The output
              access(all) fun bar(x: Int): String {
                  return ""
              }
          }

          access(all) fun test() {
              let f = Foo()
              f.b
          }
        `
		// Line 13 (0-indexed): "              f.b"
		// 14 spaces + "f.b": dot is at column 15, "b" at column 16.
		// Completion is requested after the dot = column 16.

		uri := protocol.DocumentURI("file:///completion_member.cdc")
		server.documents[uri] = Document{Text: code, Version: 1}
		_, err = server.getDiagnostics(uri, code, 1, func(*protocol.LogMessageParams) {})
		require.NoError(t, err)

		items, err := server.Completion(
			nil,
			&protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri},
					Position:     protocol.Position{Line: 13, Character: 16},
				},
			},
		)
		require.NoError(t, err)

		// Find the "bar" completion item
		var barItem *protocol.CompletionItem
		for _, item := range items {
			if item.Label == "bar" {
				barItem = item
				break
			}
		}
		require.NotNil(t, barItem, "expected 'bar' in completion items")

		resolved, err := server.ResolveCompletionItem(nil, barItem)
		require.NoError(t, err)
		require.NotNil(t, resolved.Documentation)

		assert.Equal(
			t,
			&protocol.Or_CompletionItem_documentation{
				Value: protocol.MarkupContent{
					Kind: "markdown",
					Value: "Does something.\n\n" +
						"**Parameters**\n\n" +
						"- `x`: The input\n\n" +
						"**Returns** The output",
				},
			},
			resolved.Documentation,
		)
	})

	t.Run("range completion", func(t *testing.T) {
		server, err := NewServer()
		require.NoError(t, err)

		// A documented function; trigger range (non-member) completion inside another function body.
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
              ad
          }
        `
		// "ad" is on line 12, columns 14-15. Request completion at column 16 (end of "ad").

		uri := protocol.DocumentURI("file:///completion_range.cdc")
		server.documents[uri] = Document{Text: code, Version: 1}
		// There will be checker errors since "ad" is not a valid expression,
		// but we only need the ranges to be populated.
		_, _ = server.getDiagnostics(uri, code, 1, func(*protocol.LogMessageParams) {})

		items, err := server.Completion(
			nil,
			&protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri},
					Position:     protocol.Position{Line: 12, Character: 16},
				},
			},
		)
		require.NoError(t, err)

		// Find the "add" completion item
		var addItem *protocol.CompletionItem
		for _, item := range items {
			if item.Label == "add" {
				addItem = item
				break
			}
		}
		require.NotNil(t, addItem, "expected 'add' in completion items")

		resolved, err := server.ResolveCompletionItem(nil, addItem)
		require.NoError(t, err)
		require.NotNil(t, resolved.Documentation)

		assert.Equal(
			t,
			&protocol.Or_CompletionItem_documentation{
				Value: "Adds two numbers.\n\n" +
					"**Parameters**\n\n" +
					"- `a`: The first number\n" +
					"- `b`: The second number\n\n" +
					"**Returns** The sum",
			},
			resolved.Documentation,
		)
	})
}
