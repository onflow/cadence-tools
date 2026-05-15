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

func TestHover(t *testing.T) {
	t.Parallel()

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
			Position:     protocol.Position{Line: 2, Character: 15},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, hover)

	assert.Equal(
		t,
		&protocol.Hover{
			Range: protocol.Range{
				Start: protocol.Position{Line: 2, Character: 14},
				End:   protocol.Position{Line: 2, Character: 17},
			},
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: "**Type**\n\n```cadence\nInt\n```\n",
			},
		},
		hover,
	)
}

func TestHoverType(t *testing.T) {
	t.Parallel()

	server, err := NewServer()
	require.NoError(t, err)

	const code = `
      /// docstring for A
      access(all) contract A {

          /// docstring for S
          access(all) struct S {}

          access(all) fun b(_ s: A.S) {}
      }
    `
	uri := protocol.DocumentURI("file:///test.cdc")

	_, err = server.getDiagnostics(uri, code, 1, func(*protocol.LogMessageParams) {})
	require.NoError(t, err)

	hoverA, err := server.Hover(
		nil,
		&protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 7, Character: 34},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, hoverA)

	assert.Equal(
		t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nA\n```\n\n**Documentation**\n\n docstring for A\n",
		},
		hoverA.Contents,
	)

	hoverS, err := server.Hover(
		nil,
		&protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri},
			Position:     protocol.Position{Line: 7, Character: 36},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, hoverS)

	assert.Equal(
		t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nA.S\n```\n\n**Documentation**\n\n docstring for S\n",
		},
		hoverS.Contents,
	)
}

type staticProjectIdentity string

func (s staticProjectIdentity) ProjectIDForURI(protocol.DocumentURI) string {
	return string(s)
}

func TestHoverTypeImported(t *testing.T) {
	t.Parallel()

	const fooCode = `
      /// docstring for Foo
      access(all) contract Foo {
          /// docstring for S
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

	err = server.SetOptions(
		WithStringImportResolver(
			func(_ string, location common.StringLocation) (string, error) {
				if location == "Foo" {
					return fooCode, nil
				}
				return "", nil
			},
		),
	)
	require.NoError(t, err)

	server.projectIdentity = staticProjectIdentity("foobar")

	counterURI := protocol.DocumentURI("file:///Foobar/cadence/contracts/Counter.cdc")
	_, err = server.getDiagnostics(counterURI, counterCode, 1, func(*protocol.LogMessageParams) {})
	require.NoError(t, err)

	hoverFoo, err := server.Hover(
		nil,
		&protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
			Position:     protocol.Position{Line: 4, Character: 37},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, hoverFoo)

	assert.Equal(t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nFoo\n```\n\n**Documentation**\n\n docstring for Foo\n",
		},
		hoverFoo.Contents,
	)

	hoverS, err := server.Hover(
		nil,
		&protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
			Position:     protocol.Position{Line: 4, Character: 39},
		},
	)
	require.NoError(t, err)
	require.NotNil(t, hoverS)

	assert.Equal(t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nFoo.S\n```\n\n**Documentation**\n\n docstring for S\n",
		},
		hoverS.Contents,
	)
}

func TestHoverMemberImported(t *testing.T) {
	t.Parallel()

	const fooCode = `
      // docstring for Foo
      access(all) contract Foo {

          /// docstring for a
          access(all) let a: Int

          access(all) struct S {

              /// docstring for foo
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

	err = server.SetOptions(
		WithStringImportResolver(func(_ string, location common.StringLocation) (string, error) {
			if location == "Foo" {
				return fooCode, nil
			}
			return "", nil
		}),
	)
	require.NoError(t, err)

	server.projectIdentity = staticProjectIdentity("foobar")

	counterURI := protocol.DocumentURI("file:///Foobar/cadence/contracts/Counter.cdc")
	_, err = server.getDiagnostics(counterURI, counterCode, 1, func(*protocol.LogMessageParams) {})
	require.NoError(t, err)

	getHover := func(line, character uint32) *protocol.Hover {
		t.Helper()

		h, herr := server.Hover(
			nil,
			&protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: counterURI},
				Position:     protocol.Position{Line: line, Character: character},
			},
		)
		require.NoError(t, herr)
		require.NotNil(t, h)
		return h
	}

	assert.Equal(t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nInt\n```\n\n**Documentation**\n\n docstring for a\n",
		},
		getHover(5, 27).Contents,
	)

	assert.Equal(t,
		protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "**Type**\n\n```cadence\nfun ()\n```\n\n**Documentation**\n\n docstring for foo\n",
		},
		getHover(7, 17).Contents,
	)
}
