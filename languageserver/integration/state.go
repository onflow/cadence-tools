package integration

import (
	"fmt"
	"path/filepath"

	"github.com/onflow/flow-cli/flowkit"
)

//go:generate go run github.com/vektra/mockery/cmd/mockery --name flowState --filename mock_state_test.go --inpkg
type flowState interface {
	Load(configPath string) (error)
	Reload() (error)
	GetCodeByName(name string) (string, error)
	IsLoaded() (bool)
	getConfigPath() (string)
	getState() (*flowkit.State)
}

var _ flowState = &flowkitState{}

type flowkitState struct {
	loader        flowkit.ReaderWriter
	state  				*flowkit.State
	configPath    string
}

func newFlowkitState(loader flowkit.ReaderWriter) (*flowkitState) {
	return &flowkitState{
		loader: loader,
	}
}

func (f *flowkitState) Load(configPath string) (error) {
	state, err := flowkit.Load([]string{configPath}, f.loader)
	if err != nil {
		return err
	}

	f.state = state
	f.configPath = configPath
	return nil
}

func (f *flowkitState) Reload() (error) {
	return f.Load(f.configPath)
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

func (s *flowkitState) IsLoaded() (bool) {
	return s.state != nil
}

func (s *flowkitState) getConfigPath() (string) {
	return s.configPath
}

func (s *flowkitState) getState() (*flowkit.State) {
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