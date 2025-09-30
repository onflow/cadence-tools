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

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/flowkit/v2"
)

// ConfigManager manages multiple Flow config instances concurrently and resolves
// the appropriate config/state/client for a given file based on the closest-ancestor flow.json.
type ConfigManager struct {
	mu               sync.RWMutex
	loader           flowkit.ReaderWriter
	enableFlowClient bool
	numberOfAccounts int

	// Init-config path provided via initialization options. This acts as a global default/override
	// for command handlers that don't provide an explicit document path (i.e., DefaultClient selection).
	// It does NOT change per-document resolution, which always uses the nearest ancestor flow.json.
	initConfigPath string

	// lastUsedConfigPath remembers the most recently resolved config path
	// for prioritizing a "current" project when no path is provided.
	lastUsedConfigPath string

	// Maps a config file path to a state/client instance
	states  map[string]flowState
	clients map[string]flowClient

	// file watchers per config path
	watchers map[string]*fsnotify.Watcher

	// cache mapping document file path -> resolved config path
	docToConfig map[string]string

	// directory watchers to detect creation/removal of flow.json
	dirWatchers map[string]*fsnotify.Watcher

	// loadErrors keeps the last load/reload error per config path (abs)
	loadErrors map[string]string
}

func NewConfigManager(loader flowkit.ReaderWriter, enableFlowClient bool, numberOfAccounts int, initConfigPath string) *ConfigManager {
	return &ConfigManager{
		loader:           loader,
		enableFlowClient: enableFlowClient,
		numberOfAccounts: numberOfAccounts,
		initConfigPath:   initConfigPath,
		states:           make(map[string]flowState),
		clients:          make(map[string]flowClient),
		watchers:         make(map[string]*fsnotify.Watcher),
		docToConfig:      make(map[string]string),
		dirWatchers:      make(map[string]*fsnotify.Watcher),
		loadErrors:       make(map[string]string),
	}
}

// ResolveStateForChecker returns the state associated with the closest flow.json for the given checker.
func (m *ConfigManager) ResolveStateForChecker(checker *sema.Checker) (flowState, error) {
	if checker == nil || checker.Location == nil {
		return m.resolveStateForPath("")
	}
	return m.resolveStateForPath(checker.Location.String())
}

// ResolveClientForChecker returns the client associated with the closest flow.json for the given checker.
func (m *ConfigManager) ResolveClientForChecker(checker *sema.Checker) (flowClient, error) {
	if !m.enableFlowClient {
		return nil, nil
	}
	if checker == nil || checker.Location == nil {
		return m.resolveClientForPath("")
	}
	return m.resolveClientForPath(checker.Location.String())
}

// ReloadAll reloads all known states/clients.
func (m *ConfigManager) ReloadAll() error {
	m.mu.RLock()
	states := make([]flowState, 0, len(m.states))
	for _, st := range m.states {
		states = append(states, st)
	}
	m.mu.RUnlock()

	for _, st := range states {
		if st != nil && st.IsLoaded() {
			if err := st.Reload(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Internal helpers

func (m *ConfigManager) resolveStateForPath(filePath string) (flowState, error) {
	cfgPath := m.lookupOrFindConfig(filePath)
	if cfgPath == "" {
		return nil, nil
	}
	m.mu.RLock()
	st, ok := m.states[cfgPath]
	m.mu.RUnlock()
	if ok && st != nil && st.IsLoaded() {
		m.setLastUsed(cfgPath)
		return st, nil
	}
	st, err := m.loadState(cfgPath)
	if err == nil && st != nil {
		m.setLastUsed(cfgPath)
	}
	return st, err
}

func (m *ConfigManager) resolveClientForPath(filePath string) (flowClient, error) {
	if !m.enableFlowClient {
		return nil, nil
	}
	cfgPath := m.lookupOrFindConfig(filePath)
	if cfgPath == "" {
		return nil, nil
	}
	m.mu.RLock()
	cl, ok := m.clients[cfgPath]
	m.mu.RUnlock()
	if ok && cl != nil {
		m.setLastUsed(cfgPath)
		return cl, nil
	}
	// Ensure state exists first
	st, err := m.loadState(cfgPath)
	if err != nil {
		return nil, err
	}
	cl, err = m.loadClient(cfgPath, st)
	if err == nil && cl != nil {
		m.setLastUsed(cfgPath)
	}
	return cl, err
}

func (m *ConfigManager) loadState(cfgPath string) (flowState, error) {
	absCfgPath, err := filepath.Abs(cleanWindowsPath(cfgPath))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.states[absCfgPath]; ok && st != nil && st.IsLoaded() {
		return st, nil
	}
	st := newFlowkitState(m.loader)
	if err := st.Load(absCfgPath); err != nil {
		m.loadErrors[absCfgPath] = err.Error()
		return nil, err
	}
	m.states[absCfgPath] = st
	m.ensureWatcherLocked(absCfgPath)
	// clear last error on success
	delete(m.loadErrors, absCfgPath)
	return st, nil
}

func (m *ConfigManager) loadClient(cfgPath string, st flowState) (flowClient, error) {
	if !m.enableFlowClient {
		return nil, nil
	}
	absCfgPath, err := filepath.Abs(cleanWindowsPath(cfgPath))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cl, ok := m.clients[absCfgPath]; ok && cl != nil {
		return cl, nil
	}
	cl := newFlowkitClient(m.loader)
	if err := cl.Initialize(st, m.numberOfAccounts); err != nil {
		m.loadErrors[absCfgPath] = err.Error()
		return nil, err
	}
	m.clients[absCfgPath] = cl
	delete(m.loadErrors, absCfgPath)
	return cl, nil
}

// findNearestFlowJSON walks up from the file's directory to find the closest flow.json.
// Returns an empty string if none is found or filePath is empty.
func (m *ConfigManager) findNearestFlowJSON(filePath string) string {
	if filePath == "" {
		return ""
	}
	p := cleanWindowsPath(filePath)
	dir := filepath.Dir(p)
	prev := ""
	for dir != prev {
		candidate := filepath.Join(dir, "flow.json")
		// Use loader to check for existence
		if _, err := m.loader.Stat(candidate); err == nil {
			return candidate
		}
		prev = dir
		dir = filepath.Dir(dir)
	}
	return ""
}

// lookupOrFindConfig gets cached config path for filePath or finds and caches it.
func (m *ConfigManager) lookupOrFindConfig(filePath string) string {
	m.mu.RLock()
	if cfg, ok := m.docToConfig[filePath]; ok {
		m.mu.RUnlock()
		return cfg
	}
	m.mu.RUnlock()
	cfg := m.findNearestFlowJSON(filePath)
	if cfg != "" {
		m.mu.Lock()
		m.docToConfig[filePath] = cfg
		m.mu.Unlock()
	}
	return cfg
}

// Public wrappers for path-based resolution
func (m *ConfigManager) ResolveStateForPath(filePath string) (flowState, error) {
	return m.resolveStateForPath(filePath)
}

func (m *ConfigManager) ResolveClientForPath(filePath string) (flowClient, error) {
	return m.resolveClientForPath(filePath)
}

// ConfigPathForProject normalizes a project ID (flow.json path) to an absolute config path
func (m *ConfigManager) ConfigPathForProject(projectID string) string {
	if projectID == "" {
		return ""
	}
	// Support project IDs with a cache-busting suffix, e.g., "/path/flow.json@<mtime>"
	if i := strings.Index(projectID, "@"); i >= 0 {
		projectID = projectID[:i]
	}
	cfgPath := cleanWindowsPath(projectID)
	cfgPath = deURI(cfgPath)
	absCfgPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return cfgPath
	}
	// If a directory was provided, prefer flow.json within it
	if filepath.Base(absCfgPath) != "flow.json" {
		candidate := filepath.Join(absCfgPath, "flow.json")
		if _, err := m.loader.Stat(candidate); err == nil {
			return candidate
		}
	}
	return absCfgPath
}

// IsPathInProject reports whether absPath is inside the project root directory of projectID
func (m *ConfigManager) IsPathInProject(projectID string, absPath string) bool {
	cfgPath := m.ConfigPathForProject(projectID)
	if cfgPath == "" || absPath == "" {
		return false
	}
	absRoot, _ := filepath.Abs(filepath.Dir(cfgPath))
	absFile, _ := filepath.Abs(absPath)
	if rel, err := filepath.Rel(absRoot, absFile); err == nil {
		return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return false
}

// IsSameProject reports whether absPath resolves to the same flow.json as projectID
func (m *ConfigManager) IsSameProject(projectID string, absPath string) bool {
	cfgPath := m.ConfigPathForProject(projectID)
	if cfgPath == "" {
		return true
	}
	dst := m.NearestConfigPath(absPath)
	if dst == "" {
		return true
	}
	absDst, _ := filepath.Abs(cleanWindowsPath(dst))
	return absDst == cfgPath
}

// GetContractSourceForProject reads the project's flow.json and returns the code for the given contract name if mapped
func (m *ConfigManager) GetContractSourceForProject(projectID string, name string) (string, error) {
	cfgPath := m.ConfigPathForProject(projectID)
	if cfgPath == "" || name == "" {
		return "", nil
	}
	data, err := m.loader.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Contracts map[string]string `json:"contracts"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}
	rel, ok := parsed.Contracts[name]
	if !ok || rel == "" {
		return "", nil
	}
	dir := filepath.Dir(cfgPath)
	path := filepath.Join(dir, rel)
	code, err := m.loader.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(code), nil
}

// ResolveStateForProject returns the state associated with the given project ID (flow.json path)
func (m *ConfigManager) ResolveStateForProject(projectID string) (flowState, error) {
	if projectID == "" {
		// Fallback to last used project if available
		m.mu.RLock()
		fallback := m.lastUsedConfigPath
		m.mu.RUnlock()
		if fallback != "" {
			return m.loadState(fallback)
		}
		return nil, nil
	}
	absCfgPath := m.ConfigPathForProject(projectID)
	m.mu.RLock()
	st, ok := m.states[absCfgPath]
	m.mu.RUnlock()
	if ok && st != nil && st.IsLoaded() {
		m.setLastUsed(absCfgPath)
		return st, nil
	}
	st, err := m.loadState(absCfgPath)
	if err == nil && st != nil {
		m.setLastUsed(absCfgPath)
	}
	return st, err
}

// ResolveClientForProject returns the Flow client for the given project ID (flow.json path)
func (m *ConfigManager) ResolveClientForProject(projectID string) (flowClient, error) {
	if !m.enableFlowClient {
		return nil, nil
	}
	if projectID == "" {
		m.mu.RLock()
		fallback := m.lastUsedConfigPath
		m.mu.RUnlock()
		if fallback != "" {
			st, err := m.loadState(fallback)
			if err != nil {
				return nil, err
			}
			return m.loadClient(fallback, st)
		}
		return nil, nil
	}
	absCfgPath := m.ConfigPathForProject(projectID)
	m.mu.RLock()
	cl, ok := m.clients[absCfgPath]
	m.mu.RUnlock()
	if ok && cl != nil {
		m.setLastUsed(absCfgPath)
		return cl, nil
	}
	st, err := m.loadState(absCfgPath)
	if err != nil {
		return nil, err
	}
	cl, err = m.loadClient(absCfgPath, st)
	if err == nil && cl != nil {
		m.setLastUsed(absCfgPath)
	}
	return cl, err
}

// NearestConfigPath returns the closest ancestor flow.json for the given file path, or empty if none.
func (m *ConfigManager) NearestConfigPath(filePath string) string {
	return m.findNearestFlowJSON(filePath)
}

// DefaultClient returns any initialized client if available (first encountered).
// Returns nil if no clients are initialized.
func (m *ConfigManager) DefaultClient() flowClient {
	if !m.enableFlowClient {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Prefer the init-config (passed via init options) as an override
	if m.initConfigPath != "" {
		if cl, ok := m.clients[m.initConfigPath]; ok && cl != nil {
			return cl
		}
	}
	// Then prefer the last used config's client
	if m.lastUsedConfigPath != "" {
		if cl, ok := m.clients[m.lastUsedConfigPath]; ok && cl != nil {
			return cl
		}
	}
	// Otherwise, return any initialized client
	for _, cl := range m.clients {
		if cl != nil {
			return cl
		}
	}
	return nil
}

func (m *ConfigManager) setLastUsed(cfgPath string) {
	abs, err := filepath.Abs(cleanWindowsPath(cfgPath))
	if err != nil {
		abs = cfgPath
	}
	m.mu.Lock()
	m.lastUsedConfigPath = abs
	m.mu.Unlock()
}

// SetInitConfigPath sets the init-config override used as the global default for commands
// that don't provide an explicit document path. Ensures a watcher exists for the override.
func (m *ConfigManager) SetInitConfigPath(cfgPath string) {
	absCfgPath, _ := filepath.Abs(cleanWindowsPath(cfgPath))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initConfigPath = absCfgPath
	if absCfgPath != "" {
		m.ensureWatcherLocked(absCfgPath)
	}
}

// SetDefaultClientForPath seeds the manager with a ready client for the given config path.
// Useful to make a default client available before any resolution happens.
func (m *ConfigManager) SetDefaultClientForPath(cfgPath string, cl flowClient) {
	if cfgPath == "" || cl == nil {
		return
	}
	absCfgPath, _ := filepath.Abs(cleanWindowsPath(cfgPath))
	m.mu.Lock()
	m.clients[absCfgPath] = cl
	m.lastUsedConfigPath = absCfgPath
	m.mu.Unlock()
}

// EnsureDirWatcher adds a watcher for the given directory to detect flow.json creation/removal.
func (m *ConfigManager) EnsureDirWatcher(dirPath string) {
	absDir, _ := filepath.Abs(cleanWindowsPath(dirPath))
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.dirWatchers[absDir]; ok {
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	if err := watcher.Add(absDir); err != nil {
		_ = watcher.Close()
		return
	}
	m.dirWatchers[absDir] = watcher
	go m.watchDirLoop(absDir, watcher)
}

// ensureWatcherLocked assumes m.mu is locked and creates a watcher for cfgPath if needed.
func (m *ConfigManager) ensureWatcherLocked(cfgPath string) {
	if _, ok := m.watchers[cfgPath]; ok {
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	// Watch the config file and its parent directory to catch atomic saves
	_ = watcher.Add(cfgPath)
	_ = watcher.Add(filepath.Dir(cfgPath))
	m.watchers[cfgPath] = watcher

	go m.watchLoop(cfgPath, watcher)
}

func (m *ConfigManager) watchLoop(cfgPath string, watcher *fsnotify.Watcher) {
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}
	const debounceWindow = 200 * time.Millisecond

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Trigger on writes to current config path
			if ev.Name == cfgPath {
				debounce.Reset(debounceWindow)
				continue
			}
			// Handle rename/create of flow.json in the same directory (atomic save semantics)
			dir := filepath.Dir(cfgPath)
			if filepath.Dir(ev.Name) == dir && filepath.Base(ev.Name) == "flow.json" {
				newCfg := filepath.Join(dir, "flow.json")
				if newCfg != cfgPath {
					m.mu.Lock()
					// Move state/client to new key if present
					if st, ok := m.states[cfgPath]; ok {
						delete(m.states, cfgPath)
						m.states[newCfg] = st
					}
					if cl, ok := m.clients[cfgPath]; ok {
						delete(m.clients, cfgPath)
						m.clients[newCfg] = cl
					}
					// Replace watcher mapping
					delete(m.watchers, cfgPath)
					m.watchers[newCfg] = watcher
					cfgPath = newCfg
					m.mu.Unlock()
					// Ensure we're watching the new path
					_ = watcher.Add(newCfg)
					debounce.Reset(debounceWindow)
				}
			}
		case <-debounce.C:
			// Reload state and client
			m.mu.RLock()
			st := m.states[cfgPath]
			cl := m.clients[cfgPath]
			m.mu.RUnlock()
			if st != nil {
				if err := st.Reload(); err != nil {
					m.mu.Lock()
					m.loadErrors[cfgPath] = err.Error()
					// drop broken state/client so next access tries re-load and reports error
					delete(m.states, cfgPath)
					if c := m.clients[cfgPath]; c != nil {
						delete(m.clients, cfgPath)
					}
					m.mu.Unlock()
				} else {
					m.mu.Lock()
					delete(m.loadErrors, cfgPath)
					m.mu.Unlock()
				}
			}
			if cl != nil {
				_ = cl.Reload()
			}
			// Invalidate cached file->config mapping for this config
			m.invalidateIndexForConfig(cfgPath)
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
			// ignore errors for now
		}
	}
}

// LastLoadError returns the last captured load/reload error for a given projectID (flow.json path).
func (m *ConfigManager) LastLoadError(projectID string) string {
	abs := m.ConfigPathForProject(projectID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loadErrors[abs]
}

// invalidateIndexForConfig removes any doc cache entries pointing at cfgPath
func (m *ConfigManager) invalidateIndexForConfig(cfgPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for file, cfg := range m.docToConfig {
		if cfg == cfgPath {
			delete(m.docToConfig, file)
		}
	}
}

// watchDirLoop listens for flow.json create/remove/rename in a directory and invalidates mappings
// for any documents under that directory. It also ensures config state is (re)loaded on creation.
func (m *ConfigManager) watchDirLoop(dir string, watcher *fsnotify.Watcher) {
	const debounceWindow = 200 * time.Millisecond
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only react to flow.json in this directory
			base := filepath.Base(ev.Name)
			if base != "flow.json" || filepath.Dir(ev.Name) != dir {
				continue
			}
			debounce.Reset(debounceWindow)
		case <-debounce.C:
			cfgPath := filepath.Join(dir, "flow.json")
			// If file exists now, reload state/client and attach file watcher
			if _, err := m.loader.Stat(cfgPath); err == nil {
				m.mu.RLock()
				st := m.states[cfgPath]
				cl := m.clients[cfgPath]
				m.mu.RUnlock()
				if st != nil {
					_ = st.Reload()
				} else {
					// ensure a state is loaded and watcher added
					_, _ = m.resolveStateForPath(cfgPath)
				}
				if cl != nil {
					_ = cl.Reload()
				}
				// Invalidate any cached doc->config pointing under this dir
				m.invalidateIndexForConfig(cfgPath)
			} else {
				// flow.json removed: invalidate caches and drop state/client
				m.invalidateIndexForConfig(cfgPath)
				m.mu.Lock()
				delete(m.states, cfgPath)
				delete(m.clients, cfgPath)
				m.mu.Unlock()
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}
