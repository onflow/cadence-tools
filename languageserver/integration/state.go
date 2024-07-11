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
	"fmt"
	"path/filepath"

	"github.com/onflow/flowkit/v2"
)

//go:generate go run github.com/vektra/mockery/cmd/mockery --name flowState --filename mock_state_test.go --inpkg
type flowState interface {
	Load(configPath string) error
	Reload() error
	GetCodeByName(name string) (string, error)
	IsLoaded() bool
	getConfigPath() string
	getState() *flowkit.State
}

var _ flowState = &flowkitState{}

type flowkitState struct {
	loader     flowkit.ReaderWriter
	state      *flowkit.State
	configPath string
}

func newFlowkitState(loader flowkit.ReaderWriter) *flowkitState {
	return &flowkitState{
		loader: loader,
	}
}

func (s *flowkitState) Load(configPath string) error {
	state, err := flowkit.Load([]string{configPath}, s.loader)
	if err != nil {
		return err
	}

	s.configPath = configPath
	s.state = state
	return nil
}

func (s *flowkitState) Reload() error {
	if !s.IsLoaded() {
		return fmt.Errorf("state is not initialized")
	}
	return s.Load(s.configPath)
}

func (s *flowkitState) GetCodeByName(name string) (string, error) {
	// Check if the state is initialized
	if !s.IsLoaded() {
		return "", fmt.Errorf("state is not initialized")
	}

	// Try to find the contract by name
	contract, err := s.state.Contracts().ByName(name)
	if err != nil {
		return "", fmt.Errorf("couldn't find the contract by import identifier: %s", name)
	}

	// If no location is set, return an error
	if contract.Location == "" {
		return "", fmt.Errorf("source file could not be found for import identifier: %s", name)
	}

	// Resolve the contract source code from file location
	code, err := s.getCodeFromLocation(name, contract.Location)
	if err != nil {
		return "", err
	}
	return code, nil
}

func (s *flowkitState) IsLoaded() bool {
	return s.state != nil && s.configPath != ""
}

func (s *flowkitState) getConfigPath() string {
	return s.configPath
}

func (s *flowkitState) getState() *flowkit.State {
	return s.state
}

// Helpers
//

// Helper function to get code from a source file location
func (s *flowkitState) getCodeFromLocation(name, location string) (string, error) {
	dir := filepath.Dir(s.configPath)
	path := filepath.Join(dir, location)
	code, err := s.loader.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(code), nil
}
