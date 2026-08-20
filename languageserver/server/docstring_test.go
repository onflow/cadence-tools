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
)

func TestFormatDocString(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", formatDocString(""))
		assert.Equal(t, "", formatDocString("   "))
	})

	t.Run("no annotations", func(t *testing.T) {
		assert.Equal(t,
			"Some description of a function.",
			formatDocString("Some description of a function."),
		)
	})

	t.Run("only params", func(t *testing.T) {
		input := `@param foo: Description of foo
@param bar: Description of bar`
		expected := "**Parameters**\n\n- `foo`: Description of foo\n- `bar`: Description of bar"
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("only return", func(t *testing.T) {
		input := `@return Description of result`
		expected := "**Returns** Description of result"
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("content with params and return", func(t *testing.T) {
		input := `Does something useful.

@param foo: Description of foo
@param bar: Description of bar
@return The result`
		expected := `Does something useful.

**Parameters**

- ` + "`foo`" + `: Description of foo
- ` + "`bar`" + `: Description of bar

**Returns** The result`
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("params interspersed in content", func(t *testing.T) {
		input := `First line.
@param foo: Description of foo
Some middle text.
@param bar: Description of bar
@return The result`
		expected := `First line.
Some middle text.

**Parameters**

- ` + "`foo`" + `: Description of foo
- ` + "`bar`" + `: Description of bar

**Returns** The result`
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("param without colon treated as content", func(t *testing.T) {
		input := `@param noColon
@param valid: has description`
		expected := "@param noColon\n\n**Parameters**\n\n- `valid`: has description"
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("param without description", func(t *testing.T) {
		input := `@param foo:`
		expected := "**Parameters**\n\n- `foo`"
		assert.Equal(t, expected, formatDocString(input))
	})

	t.Run("consecutive blank lines collapsed", func(t *testing.T) {
		input := "First line.\n\n\n\nSecond line."
		expected := "First line.\n\nSecond line."
		assert.Equal(t, expected, formatDocString(input))
	})
}
