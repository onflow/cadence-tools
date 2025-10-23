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
	"sync"
	"time"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence-tools/languageserver/protocol"
)

// FileWatcher handles watching .cdc files and invalidating caches when they change
type FileWatcher struct {
	server    *Server
	debouncer *debouncer
}

// debouncer handles debouncing of repeated events by key
type debouncer struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	delay  time.Duration
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		timers: make(map[string]*time.Timer),
		delay:  delay,
	}
}

func (d *debouncer) schedule(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if t, ok := d.timers[key]; ok && t != nil {
		t.Stop()
	}

	d.timers[key] = time.AfterFunc(d.delay, func() {
		fn()
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
	})
}

// NewFileWatcher creates a new file watcher for the server
func NewFileWatcher(s *Server) *FileWatcher {
	return &FileWatcher{
		server:    s,
		debouncer: newDebouncer(500 * time.Millisecond),
	}
}

// HandleFileChanged invalidates cached checkers and re-checks dependent files when a .cdc file changes
func (fw *FileWatcher) HandleFileChanged(absPath string) {
	fw.debouncer.schedule(absPath, func() {
		fw.handleFileChangedSync(absPath)
	})
}

func (fw *FileWatcher) handleFileChangedSync(absPath string) {
	uri := protocol.DocumentURI("file://" + absPath)
	proj := fw.server.projectResolver.ProjectIDForURI(uri)
	canonical := fw.server.projectResolver.CanonicalLocation(proj, common.StringLocation(absPath))
	key := CheckerKey{ProjectID: proj, Location: canonical}

	// Clear outgoing edges since imports will be rebuilt, but preserve incoming edges
	fw.server.store.ClearChildren(key)
	fw.server.store.RemoveCheckerOnly(key)

	// Invalidate all transitive parent checkers (they reference types from this file)
	allParents := fw.server.store.AffectedParents(key)
	for _, parent := range allParents {
		fw.server.store.RemoveCheckerOnly(parent)
	}

	// Re-check any open documents that depend on this file
	affected := fw.server.collectAffectedOpenRootURIs(key, "")
	conn := fw.server.conn
	for _, depURI := range affected {
		if doc, ok := fw.server.documents[depURI]; ok {
			fw.server.checkAndPublishDiagnostics(conn, depURI, doc.Text, doc.Version)
		}
	}
}

// HandleProjectFilesChanged re-checks all open files in a project when .cdc files are created/deleted
func (fw *FileWatcher) HandleProjectFilesChanged(projectID string) {
	fw.debouncer.schedule("project:"+projectID, func() {
		fw.handleProjectFilesChangedSync(projectID)
	})
}

func (fw *FileWatcher) handleProjectFilesChangedSync(projectID string) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the server
			// This can happen if state reload fails and checking encounters issues
		}
	}()

	// Re-check all open documents in this project
	conn := fw.server.conn
	for uri, doc := range fw.server.documents {
		proj := fw.server.projectResolver.ProjectIDForURI(uri)
		if proj == projectID {
			fw.server.checkAndPublishDiagnostics(conn, uri, doc.Text, doc.Version)
		}
	}
}

