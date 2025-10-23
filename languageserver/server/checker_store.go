package server

import (
	"sync"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"
)

// CheckerKey identifies a cached checker instance within a project scope.
type CheckerKey struct {
	ProjectID string
	Location  common.Location
}

// CheckerStore caches checkers, tracks a parent→child graph, and only supports
// boolean root pins. Import liveness is implicit via number of incoming edges.
type CheckerStore struct {
	mu sync.RWMutex

	checkers map[CheckerKey]*sema.Checker

	// Dependency graph:
	//   children[parent] -> set of children
	//   parents[child]   -> set of parents
	children map[CheckerKey]map[CheckerKey]struct{}
	parents  map[CheckerKey]map[CheckerKey]struct{}

	// Root pins: true if this key is explicitly kept as a root.
	rootPinned map[CheckerKey]bool
}

func NewCheckerStore() *CheckerStore {
	return &CheckerStore{
		checkers:   make(map[CheckerKey]*sema.Checker),
		children:   make(map[CheckerKey]map[CheckerKey]struct{}),
		parents:    make(map[CheckerKey]map[CheckerKey]struct{}),
		rootPinned: make(map[CheckerKey]bool),
	}
}

// Get retrieves a cached checker by key.
func (s *CheckerStore) Get(key CheckerKey) (*sema.Checker, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	chk, ok := s.checkers[key]
	return chk, ok
}

// Put stores/updates a checker under its key.
func (s *CheckerStore) Put(key CheckerKey, chk *sema.Checker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers[key] = chk
}

// Delete removes a checker (manual override).
// Does not touch edges; prefer RemoveParent* / Invalidate for usual flows.
func (s *CheckerStore) Delete(key CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkers, key)
	delete(s.rootPinned, key)
	// Best-effort: remove outgoing edges and clean up children's parent sets.
	if kids, ok := s.children[key]; ok {
		for child := range kids {
			if ps, ok := s.parents[child]; ok {
				delete(ps, key)
				if len(ps) == 0 {
					delete(s.parents, child)
				}
			}
		}
		delete(s.children, key)
	}
	// If this makes any children cold, evict them too.
	for child := range s.parents[key] {
		s.evictIfColdUnsafe(child)
	}
	delete(s.parents, key)
}

// RemoveCheckerOnly removes a checker from the cache and unpins it, but preserves
// all dependency edges. This is useful when a file is closed but we want to maintain
// the dependency graph for transitive change propagation.
func (s *CheckerStore) RemoveCheckerOnly(key CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkers, key)
	delete(s.rootPinned, key)
}

// Invalidate removes the checker for (projectID, location), clears its root pin,
// and prunes all outgoing edges. Cold children (and possibly the root) are evicted.
func (s *CheckerStore) Invalidate(projectID string, location common.Location) {
	key := CheckerKey{ProjectID: projectID, Location: location}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.checkers, key)
	delete(s.rootPinned, key)

	// Remove edges parent(key) -> children and cascade-candidate children list.
	children := s.removeParentAndCollectChildrenUnsafe(key)

	// Try to evict children that became cold (no parents, not root-pinned).
	for _, child := range children {
		s.evictIfColdUnsafe(child)
	}
	// The invalidated root itself may be cold if it had no parents.
	s.evictIfColdUnsafe(key)
}

// PinRoot marks a checker as an actively opened root.
func (s *CheckerStore) PinRoot(key CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rootPinned[key] = true
}

// UnpinRoot marks a checker as no longer opened as a root.
// If it now has no parents (i.e., nobody imports it), evict it.
func (s *CheckerStore) UnpinRoot(key CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rootPinned, key)
	s.evictIfColdUnsafe(key)
}

// AddEdge adds a dependency edge parent -> child.
func (s *CheckerStore) AddEdge(parent, child CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.children[parent]; !ok {
		s.children[parent] = make(map[CheckerKey]struct{})
	}
	if _, ok := s.parents[child]; !ok {
		s.parents[child] = make(map[CheckerKey]struct{})
	}
	// idempotent
	if _, exists := s.children[parent][child]; exists {
		return
	}
	s.children[parent][child] = struct{}{}
	s.parents[child][parent] = struct{}{}
}

// RemoveParent removes all outgoing edges from a parent and evicts any children
// that become cold as a result. The parent itself may also become cold.
func (s *CheckerStore) RemoveParent(parent CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	children := s.removeParentAndCollectChildrenUnsafe(parent)
	for _, child := range children {
		s.evictIfColdUnsafe(child)
	}
	s.evictIfColdUnsafe(parent)
}

// ClearChildren removes all outgoing edges from parent without evicting nodes.
// Intended for change events where dependencies will be rebuilt immediately.
func (s *CheckerStore) ClearChildren(parent CheckerKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeParentUnsafe(parent)
}

// RemoveParentAndCollectChildren removes all outgoing edges from parent and returns previous children.
func (s *CheckerStore) RemoveParentAndCollectChildren(parent CheckerKey) []CheckerKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeParentAndCollectChildrenUnsafe(parent)
}

// AffectedParents returns all transitive parents that depend on key.
func (s *CheckerStore) AffectedParents(key CheckerKey) []CheckerKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	visited := make(map[CheckerKey]struct{})
	queue := make([]CheckerKey, 0)

	for p := range s.parents[key] {
		queue = append(queue, p)
	}

	out := make([]CheckerKey, 0)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, seen := visited[cur]; seen {
			continue
		}
		visited[cur] = struct{}{}
		out = append(out, cur)
		for pp := range s.parents[cur] {
			if _, seen := visited[pp]; !seen {
				queue = append(queue, pp)
			}
		}
	}
	return out
}

// ---- internal (caller must hold lock) ----

func (s *CheckerStore) removeParentUnsafe(parent CheckerKey) {
	children, ok := s.children[parent]
	if !ok {
		return
	}
	for child := range children {
		if ps, ok := s.parents[child]; ok {
			delete(ps, parent)
			if len(ps) == 0 {
				delete(s.parents, child)
			}
		}
	}
	delete(s.children, parent)
}

func (s *CheckerStore) removeParentAndCollectChildrenUnsafe(parent CheckerKey) []CheckerKey {
	children, ok := s.children[parent]
	if !ok {
		return nil
	}
	out := make([]CheckerKey, 0, len(children))
	for child := range children {
		out = append(out, child)
		if ps, ok := s.parents[child]; ok {
			delete(ps, parent)
			if len(ps) == 0 {
				delete(s.parents, child)
			}
		}
	}
	delete(s.children, parent)
	return out
}

// isCold: true if not root-pinned and has no incoming edges (imports).
func (s *CheckerStore) isColdUnsafe(key CheckerKey) bool {
	if s.rootPinned[key] {
		return false
	}
	if ps, ok := s.parents[key]; ok && len(ps) > 0 {
		return false
	}
	return true
}

// evictIfColdUnsafe deletes key if cold and cascades to any children that
// become cold once this node is removed.
func (s *CheckerStore) evictIfColdUnsafe(key CheckerKey) {
	if !s.isColdUnsafe(key) {
		return
	}
	// Remove and detach from children
	kids := s.children[key]
	delete(s.checkers, key)
	delete(s.children, key)
	delete(s.parents, key)
	delete(s.rootPinned, key)

	// Removing this node as a parent might make children cold now.
	for child := range kids {
		if ps, ok := s.parents[child]; ok {
			delete(ps, key)
			if len(ps) == 0 {
				delete(s.parents, child)
			}
		}
		s.evictIfColdUnsafe(child)
	}
}
