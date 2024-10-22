/*
 * Cadence lint - The Cadence linter
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

package lint_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/cadence-tools/lint"
)

var testLocation = common.StringLocation("test")

func testAnalyzers(t *testing.T, code string, analyzers ...*analysis.Analyzer) []analysis.Diagnostic {
	return testAnalyzersAdvanced(t, code, nil, analyzers...)
}

func testAnalyzersWithCheckerError(
	t *testing.T,
	code string,
	analyzers ...*analysis.Analyzer,
) ([]analysis.Diagnostic, *sema.CheckerError) {
	var checkerErr *sema.CheckerError
	diagnostics := testAnalyzersAdvanced(
		t,
		code,
		func(config *analysis.Config) {
			config.HandleCheckerError = func(err analysis.ParsingCheckingError, checker *sema.Checker) error {
				require.NotNil(t, checker)
				require.Equal(t, err.ImportLocation(), testLocation)

				require.ErrorAs(t, err, &checkerErr)
				require.Len(t, checkerErr.Errors, 1)
				return nil
			}
		},
		analyzers...,
	)

	require.NotNil(t, checkerErr)
	return diagnostics, checkerErr
}

func testAnalyzersAdvanced(
	t *testing.T,
	code string,
	setCustomConfigOptions func(config *analysis.Config),
	analyzers ...*analysis.Analyzer,
) []analysis.Diagnostic {

	config := analysis.NewSimpleConfig(
		lint.LoadMode,
		map[common.Location][]byte{
			testLocation: []byte(code),
		},
		nil,
		nil,
	)

	if setCustomConfigOptions != nil {
		setCustomConfigOptions(config)
	}

	programs, err := analysis.Load(config, testLocation)
	require.NoError(t, err)

	var diagnostics []analysis.Diagnostic

	programs[testLocation].Run(
		analyzers,
		func(diagnostic analysis.Diagnostic) {
			diagnostics = append(diagnostics, diagnostic)
		},
	)

	return diagnostics
}
