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
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func Test_FindNearestFlowJSON(t *testing.T) {
	fs := afero.NewMemMapFs()
	loader := &afero.Afero{Fs: fs}

	// workspace layout:
	// /w/flow.json
	// /w/a/flow.json
	// /w/a/b/c/file.cdc

	_ = loader.MkdirAll("/w/a/b/c", 0o755)
	_ = loader.WriteFile("/w/flow.json", []byte("{}"), 0o644)
	_ = loader.WriteFile("/w/a/flow.json", []byte("{}"), 0o644)
	_ = loader.WriteFile("/w/a/b/c/file.cdc", []byte("pub fun main() {}"), 0o644)

	mgr := NewConfigManager(loader, false, 0, "")

	file := "/w/a/b/c/file.cdc"
	cfg := mgr.findNearestFlowJSON(file)
	assert.Equal(t, "/w/a/flow.json", filepath.ToSlash(cfg))

	// If file at top level, choose top-level flow.json
	topFile := "/w/z.cdc"
	_ = loader.WriteFile(topFile, []byte("pub fun main() {}"), 0o644)
	cfg = mgr.findNearestFlowJSON(topFile)
	assert.Equal(t, "/w/flow.json", filepath.ToSlash(cfg))
}

func Test_DocToConfigCacheAndInvalidation(t *testing.T) {
	fs := afero.NewMemMapFs()
	loader := &afero.Afero{Fs: fs}

	_ = loader.MkdirAll("/p/x/y", 0o755)
	_ = loader.WriteFile("/p/flow.json", []byte("{}"), 0o644)
	_ = loader.WriteFile("/p/x/flow.json", []byte("{}"), 0o644)
	_ = loader.WriteFile("/p/x/y/file.cdc", []byte("pub fun main() {}"), 0o644)

	mgr := NewConfigManager(loader, false, 0, "")

	file := "/p/x/y/file.cdc"
	// First lookup should discover and cache
	cfg1 := mgr.lookupOrFindConfig(file)
	assert.Equal(t, "/p/x/flow.json", filepath.ToSlash(cfg1))

	// Remove the flow.json file; cache should still return old mapping until invalidated
	_ = loader.Remove("/p/x/flow.json")
	cfg2 := mgr.lookupOrFindConfig(file)
	assert.Equal(t, "/p/x/flow.json", filepath.ToSlash(cfg2))

	// Invalidate and ensure it falls back to parent config
	mgr.invalidateIndexForConfig("/p/x/flow.json")
	cfg3 := mgr.lookupOrFindConfig(file)
	assert.Equal(t, "/p/flow.json", filepath.ToSlash(cfg3))
}
