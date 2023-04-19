package lint

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/onflow/cadence/tools/analysis"
)

func TestAnalyzeContractsFolder(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("unexpected panic")
		}
	}()
	nopAnalyzer := analysis.Analyzer{
		Description: "Noop analyzer",
		Run: func(pass *analysis.Pass) interface{} {
			return nil
		},
		Requires: nil,
	}
	linter := NewLinter(Config{
		Analyzers: []*analysis.Analyzer{&nopAnalyzer},
		Silent:    true,
		UseColor:  false,
	})

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to get the current filename")
	}
	dirname := filepath.Dir(filename)
	linter.AnalyzeDirectory(dirname + "/testcontracts/")
}
