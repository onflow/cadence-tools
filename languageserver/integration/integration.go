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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flowkit/v2"
	"github.com/spf13/afero"

	"github.com/onflow/cadence-tools/languageserver/protocol"
	"github.com/onflow/cadence-tools/languageserver/server"

	"path/filepath"
	"strings"
)

func (i *FlowIntegration) didOpenInitHook(s *server.Server) func(protocol.Conn, protocol.DocumentURI, string) {
	return func(conn protocol.Conn, uri protocol.DocumentURI, _ string) {
		u := string(uri)
		if !strings.HasPrefix(u, "file://") || !strings.HasSuffix(strings.ToLower(u), ".cdc") {
			return
		}
		path := deURI(cleanWindowsPath(strings.TrimPrefix(u, "file://")))
		if _, statErr := i.loader.Stat(path); statErr != nil {
			return
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if real, err := filepath.EvalSymlinks(path); err == nil {
			path = real
		}
		if i.cfgManager == nil {
			return
		}
		// Do not validate init-config here; rely on ConfigManager load errors instead
		if cfg := i.cfgManager.NearestConfigPath(path); cfg != "" {
			// Attempt to load; if flowkit fails, remember and show the error and skip prompt
			// Use ResolveStateForProject since cfg is already a config path (flow.json)
			if _, err := i.cfgManager.ResolveStateForProject(cfg); err != nil {
				conn.ShowMessage(&protocol.ShowMessageParams{
					Type:    protocol.Error,
					Message: fmt.Sprintf("Failed to load flow.json: %s", err.Error()),
				})
			}
			return
		}
		// No nearest flow.json: if an init-config override exists and had load errors, surface them now.
		if i.cfgManager.initConfigPath != "" {
			if msg := i.cfgManager.LastLoadError(i.cfgManager.initConfigPath); msg != "" {
				conn.ShowMessage(&protocol.ShowMessageParams{
					Type:    protocol.Error,
					Message: fmt.Sprintf("Invalid flow.json: %s", msg),
				})
				return
			}
		}
		if !i.promptShown.CompareAndSwap(false, true) {
			return
		}
		dir := filepath.Dir(path)
		if root := s.WorkspaceFolderRootForPath(path); root != "" {
			dir = root
		}
		// If we've already prompted for this root in this session, skip
		skip := false
		func() {
			i.promptedRootsMu.Lock()
			defer i.promptedRootsMu.Unlock()
			if _, seen := i.promptedRoots[dir]; seen {
				skip = true
				return
			}
			// mark as seen early to avoid races with concurrent didOpen on same root
			i.promptedRoots[dir] = struct{}{}
		}()
		if skip {
			return
		}
		go func() {
			action, err := conn.ShowMessageRequest(&protocol.ShowMessageRequestParams{
				Type:    protocol.Info,
				Message: "No Flow project detected. Initialize a new Flow project?",
				Actions: []protocol.MessageActionItem{{Title: "Create flow.json"}, {Title: "Ignore"}},
			})
			if err != nil || action == nil || action.Title != "Create flow.json" {
				// keep promptedRoots mark so we don't re-prompt this root until restart
				i.promptShown.Store(false)
				return
			}
			target := filepath.Join(dir, "flow.json")
			cmd := exec.Command("flow", "init", "--config-only")
			cmd.Dir = dir
			runErr := cmd.Run()
			if runErr != nil || !func() bool { _, err := os.Stat(target); return err == nil }() {
				conn.ShowMessage(&protocol.ShowMessageParams{
					Type:    protocol.Error,
					Message: fmt.Sprintf("Failed to initialize Flow project: %s", runErr.Error()),
				})
				return
			}
			if _, loadErr := i.cfgManager.ResolveStateForPath(target); loadErr != nil {
				conn.ShowMessage(&protocol.ShowMessageParams{
					Type:    protocol.Error,
					Message: fmt.Sprintf("Failed to load initialized Flow project: %s", loadErr.Error()),
				})
				return
			}
			i.promptShown.Store(false)
			conn.ShowMessage(&protocol.ShowMessageParams{
				Type:    protocol.Info,
				Message: "Initialized Flow project: " + dir,
			})
		}()
	}
}

func NewFlowIntegration(s *server.Server, enableFlowClient bool) (*FlowIntegration, error) {
	loader := &afero.Afero{Fs: afero.NewOsFs()}
	state := newFlowkitState(loader)

	integration := &FlowIntegration{
		entryPointInfo:     map[protocol.DocumentURI]*entryPointInfo{},
		contractInfo:       map[protocol.DocumentURI]*contractInfo{},
		enableFlowClient:   enableFlowClient,
		loader:             loader,
		state:              state,
		promptedRoots:      make(map[string]struct{}),
		invalidWarnedRoots: make(map[string]struct{}),
		server:             s,
	}

	// Setup config manager with file change handlers
	integration.cfgManager = NewConfigManager(loader, enableFlowClient, 0, "")
	
	// Debounce file changes to coalesce rapid successive events
	fileDebouncer := newDebouncer(500 * time.Millisecond)
	integration.cfgManager.SetOnFileChanged(func(absPath string) {
		fileDebouncer.schedule(absPath, func() {
			integration.handleFileChanged(absPath)
		})
	})
	
	// Debounce project-wide changes (file creation/deletion)
	projectDebouncer := newDebouncer(500 * time.Millisecond)
	integration.cfgManager.SetOnProjectFilesChanged(func(cfgPath string) {
		projectDebouncer.schedule(cfgPath, func() {
			integration.handleProjectFilesChanged(cfgPath)
		})
	})

	// Provide a project resolver for identity and location canonicalization
	projectResolver := flowProjectResolver{cfg: integration.cfgManager}

	resolve := resolvers{
		loader:     loader,
		cfgManager: integration.cfgManager,
	}

	options := []server.Option{
		server.WithDiagnosticProvider(diagnostics),
		server.WithStringImportResolver(resolve.stringImport),
		server.WithInitializationOptionsHandler(integration.initialize),
		server.WithExtendedStandardLibraryValues(FVMStandardLibraryValues()...),
		server.WithIdentifierImportResolver(resolve.identifierImportProject),
		server.WithProjectResolver(projectResolver),
	}

	// Prompt to create flow.json when opening an existing .cdc file without a config.
	options = append(options, server.WithDidOpenHook(integration.didOpenInitHook(s)))

	if enableFlowClient {
		client := newFlowkitClient(loader)
		integration.client = client

		options = append(options,
			server.WithCodeLensProvider(integration.codeLenses),
			server.WithAddressImportResolver(resolve.addressImport),
			server.WithAddressContractNamesResolver(resolve.addressContractNames),
			server.WithMemberAccountAccessHandler(resolve.accountAccess),
		)
	}

	// Register Flow commands only when the Flow client is enabled.
	// This preserves the previous behavior where Flow features are unavailable
	// unless the server is started with --enable-flow-client=true.
	if enableFlowClient {
		comm := commands{cfg: integration.cfgManager}
		for _, command := range comm.getAll() {
			options = append(options, server.WithCommand(command))
		}
	}

	err := s.SetOptions(options...)
	if err != nil {
		return nil, err
	}

	return integration, nil
}

type FlowIntegration struct {
	entryPointInfo map[protocol.DocumentURI]*entryPointInfo
	contractInfo   map[protocol.DocumentURI]*contractInfo

	enableFlowClient   bool
	client             flowClient
	state              *flowkitState
	loader             flowkit.ReaderWriter
	cfgManager         *ConfigManager
	promptShown        atomic.Bool
	promptedRootsMu    sync.Mutex
	promptedRoots      map[string]struct{}
	invalidWarnedMu    sync.Mutex
	invalidWarnedRoots map[string]struct{}
	server             *server.Server
}

// debouncer handles debouncing of repeated events by key
type debouncer struct {
	mu      sync.Mutex
	timers  map[string]*time.Timer
	delay   time.Duration
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

// handleFileChanged invalidates cached checkers and re-checks dependent files when a .cdc file changes
func (i *FlowIntegration) handleFileChanged(absPath string) {
	if i.server == nil {
		return
	}
	
	uri := protocol.DocumentURI("file://" + absPath)
	proj := i.server.ProjectResolver().ProjectIDForURI(uri)
	canonical := i.server.ProjectResolver().CanonicalLocation(proj, common.StringLocation(absPath))
	key := server.CheckerKey{ProjectID: proj, Location: canonical}
	
	// Clear outgoing edges since imports will be rebuilt, but preserve incoming edges
	i.server.Store().ClearChildren(key)
	i.server.Store().RemoveCheckerOnly(key)
	
	// Invalidate all transitive parent checkers (they reference types from this file)
	allParents := i.server.Store().AffectedParents(key)
	for _, parent := range allParents {
		i.server.Store().RemoveCheckerOnly(parent)
	}
	
	// Re-check any open documents that depend on this file
	affected := i.server.CollectAffectedOpenRootURIs(key, "")
	for _, depURI := range affected {
		if doc, ok := i.server.GetDocument(depURI); ok {
			i.server.CheckAndPublishDiagnostics(i.server.Conn(), depURI, doc.Text, doc.Version)
		}
	}
}

// handleProjectFilesChanged re-checks all open files in a project when .cdc files are created/deleted
func (i *FlowIntegration) handleProjectFilesChanged(cfgPath string) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the server
			// This can happen if state reload fails and checking encounters issues
		}
	}()
	
	if i.server == nil {
		return
	}
	
	// Re-check all open documents in this project
	allDocs := i.server.GetAllDocuments()
	for uri, doc := range allDocs {
		proj := i.server.ProjectResolver().ProjectIDForURI(uri)
		if proj == cfgPath {
			i.server.CheckAndPublishDiagnostics(i.server.Conn(), uri, doc.Text, doc.Version)
		}
	}
}

// flowProjectResolver implements server.ProjectResolver
type flowProjectResolver struct{ cfg *ConfigManager }

// ProjectIDForURI returns the project ID for a given URI
func (p flowProjectResolver) ProjectIDForURI(uri protocol.DocumentURI) string {
	if p.cfg == nil {
		return ""
	}
	// If an init-config override is set, always scope by that config (with mtime suffix)
	if p.cfg.initConfigPath != "" {
		return stableProjectID(p.cfg.loader, p.cfg.initConfigPath)
	}
	// Otherwise, find the nearest flow.json and use its path (with mtime suffix)
	path := deURI(cleanWindowsPath(strings.TrimPrefix(string(uri), "file://")))
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if cfgPath := p.cfg.NearestConfigPath(path); cfgPath != "" {
		// Ensure state is loaded for this project synchronously so downstream
		// canonicalization can rely on cache/state without races.
		_, _ = p.cfg.ResolveStateForProject(cfgPath)
		return stableProjectID(p.cfg.loader, cfgPath)
	}
	return ""
}

// CanonicalLocation returns a canonical file-backed location for string imports
func (p flowProjectResolver) CanonicalLocation(projectID string, location common.Location) common.Location {
	if p.cfg == nil {
		return location
	}
	switch loc := location.(type) {
	case common.StringLocation:
		name := string(loc)
		// If it looks like a file path, normalize to absolute path
		if strings.Contains(name, ".cdc") {
			filename := deURI(cleanWindowsPath(name))
			abs, err := filepath.Abs(filename)
			if err != nil {
				// If absolute path resolution fails, return original location
				return location
			}
			// Resolve symlinks for consistent canonical file identity (e.g. /var -> /private/var on macOS)
			if real, err := filepath.EvalSymlinks(abs); err == nil {
				abs = real
			}
			return common.StringLocation(abs)
		}
		// String Identifier: resolve via loaded state only
		if projectID != "" {
			if st, err := p.cfg.ResolveStateForProject(projectID); err == nil && st != nil {
				if c, err := st.getState().Contracts().ByName(name); err == nil && c.Location != "" {
					abs := filepath.Join(filepath.Dir(st.getConfigPath()), c.Location)
					if absPath, err := filepath.Abs(abs); err == nil {
						abs = absPath
					}
					if real, err := filepath.EvalSymlinks(abs); err == nil {
						abs = real
					}
					return common.StringLocation(abs)
				}
			}
		}
		return location
	case common.IdentifierLocation:
		// Convert identifier to file-backed path using loaded state only
		if projectID != "" {
			name := string(loc)
			if st, err := p.cfg.ResolveStateForProject(projectID); err == nil && st != nil {
				if c, err := st.getState().Contracts().ByName(name); err == nil && c.Location != "" {
					abs := filepath.Join(filepath.Dir(st.getConfigPath()), c.Location)
					if absPath, err := filepath.Abs(abs); err == nil {
						abs = absPath
					}
					if real, err := filepath.EvalSymlinks(abs); err == nil {
						abs = real
					}
					return common.StringLocation(abs)
				}
			}
		}
		return location
	default:
		return location
	}
}

func (i *FlowIntegration) initialize(initializationOptions any) error {
	optsMap, ok := initializationOptions.(map[string]any)
	if !ok {
		// If client is enabled, initialization options are required
		if i.enableFlowClient {
			return errors.New("invalid initialization options")
		}
		return nil
	}

	configPath, ok := optsMap["configPath"].(string)
	if !ok || configPath == "" {
		// If client is enabled, config path is required, otherwise it's optional
		if i.enableFlowClient {
			return errors.New("initialization options: invalid config path")
		}
		return nil
	}

	// Load the config state if provided. If it fails, don't fail initialization;
	// still set the init-config override so the server can run and the user can fix the file.
	configPath = cleanWindowsPath(configPath)
	err := i.state.Load(configPath)
	if err != nil {
		i.handleInitConfigLoadFailure(configPath)
		//nolint:nilerr // intentionally do not fail initialization; LS should run and surface config errors later
		return nil
	}

	// Initialize ConfigManager with provided init-config path (override)
	numberOfAccounts := 0
	if i.enableFlowClient {
		if numberOfAccountsString, ok := optsMap["numberOfAccounts"].(string); ok && numberOfAccountsString != "" {
			if n, convErr := strconv.Atoi(numberOfAccountsString); convErr == nil {
				numberOfAccounts = n
			}
		}
	}
	// Reuse existing manager instance to avoid stale references
	if err := i.setupConfigManager(configPath, numberOfAccounts); err != nil {
		return err
	}

	// If client is enabled, initialize the client (only when state loaded successfully)
	if i.enableFlowClient {
		numberOfAccountsString, ok := optsMap["numberOfAccounts"].(string)
		if !ok || numberOfAccountsString == "" {
			return errors.New("initialization options: invalid account number value, should be passed as a string")
		}
		numberOfAccounts, err := strconv.Atoi(numberOfAccountsString)
		if err != nil {
			return errors.New("initialization options: invalid account number value")
		}

		err = i.client.Initialize(i.state, numberOfAccounts)
		if err != nil {
			return err
		}
		// Seed default client into ConfigManager to be available for commands without path
		if i.cfgManager != nil {
			i.cfgManager.SetDefaultClientForPath(configPath, i.client)
		}
	}

	return nil
}

// handleInitConfigLoadFailure seeds the config manager with the init-config path and
// records any load error for later surfacing, without failing initialization.
func (i *FlowIntegration) handleInitConfigLoadFailure(configPath string) {
	// Seed cfgManager with the init-config path so project scoping still works
	if i.cfgManager != nil {
		i.cfgManager.enableFlowClient = i.enableFlowClient
		i.cfgManager.numberOfAccounts = 0
		i.cfgManager.SetInitConfigPath(configPath)
		// Record the failure in cfgManager by attempting to resolve state there as well
		if _, loadErr := i.cfgManager.ResolveStateForProject(configPath); loadErr != nil {
			// cfgManager tracks last load error internally; retain for diagnostics to satisfy staticcheck
			_ = loadErr
		}
	}
	// Best-effort: read file and try to detect bad contract paths for user feedback later
	if i.client != nil {
		if data, readErr := i.loader.ReadFile(configPath); readErr == nil {
			var parsed struct {
				Contracts map[string]string `json:"contracts"`
			}
			if jsonErr := json.Unmarshal(data, &parsed); jsonErr == nil && len(parsed.Contracts) > 0 {
				for _, rel := range parsed.Contracts {
					candidate := filepath.Join(filepath.Dir(configPath), rel)
					if _, statErr := i.loader.Stat(candidate); statErr != nil {
						root := filepath.Dir(configPath)
						i.invalidWarnedMu.Lock()
						if _, seen := i.invalidWarnedRoots[root]; !seen {
							i.invalidWarnedRoots[root] = struct{}{}
							i.invalidWarnedMu.Unlock()
						} else {
							i.invalidWarnedMu.Unlock()
						}
						break
					}
				}
			}
		}
	}
}

// setupConfigManager configures cfgManager for the given configPath and numberOfAccounts
// and ensures state is loaded so LastLoadError reflects reality.
func (i *FlowIntegration) setupConfigManager(configPath string, numberOfAccounts int) error {
	if i.cfgManager == nil {
		return nil
	}
	i.cfgManager.enableFlowClient = i.enableFlowClient
	i.cfgManager.numberOfAccounts = numberOfAccounts
	i.cfgManager.SetInitConfigPath(configPath)
	if _, err := i.cfgManager.ResolveStateForProject(configPath); err != nil {
		return err
	}
	return nil
}

func (i *FlowIntegration) codeLenses(
	uri protocol.DocumentURI,
	version int32,
	checker *sema.Checker,
) (
	[]*protocol.CodeLens,
	error,
) {
	var actions []*protocol.CodeLens

	// todo refactor - define codelens provider interface and merge both into one

	// Ensure we watch the document directory for flow.json creation/removal
	if i.cfgManager != nil {
		// Only for file:// URIs
		u := string(uri)
		if strings.HasPrefix(u, "file://") {
			path := cleanWindowsPath(strings.TrimPrefix(u, "file://"))
			dir := filepath.Dir(path)
			i.cfgManager.EnsureDirWatcher(dir)
		}
	}

	// Prefer already-initialized integration client; fall back to per-document resolution only if nil
	clientToUse := i.client
	if clientToUse == nil && i.cfgManager != nil {
		if cl, err := i.cfgManager.ResolveClientForChecker(checker); err == nil && cl != nil {
			clientToUse = cl
		}
	}

	// Add code lenses for contracts and contract interfaces
	contract := i.contractInfo[uri]
	if contract == nil {
		contract = &contractInfo{} // create new
		i.contractInfo[uri] = contract
	}
	contract.update(uri, version, checker)
	actions = append(actions, contract.codelens(clientToUse)...)

	// Add code lenses for scripts and transactions
	entryPoint := i.entryPointInfo[uri]
	if entryPoint == nil {
		entryPoint = &entryPointInfo{}
		i.entryPointInfo[uri] = entryPoint
	}
	entryPoint.update(uri, version, checker)
	actions = append(actions, entryPoint.codelens(clientToUse)...)

	return actions, nil
}

// stableProjectID composes a stable project identifier using the canonical absolute config path.
// No modtime suffix is used to ensure identity is consistent across a session.
func stableProjectID(loader flowkit.ReaderWriter, cfgPath string) string {
	if abs, err := filepath.Abs(cfgPath); err == nil {
		cfgPath = abs
	}
	if real, err := filepath.EvalSymlinks(cfgPath); err == nil {
		cfgPath = real
	}
	return cfgPath
}
