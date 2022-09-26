/*
 * Cadence-lint - The Cadence linter
 *
 * Copyright 2019-2022 Dapper Labs, Inc.
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

package analyzers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/tools/analysis"
)

var testLocation = common.StringLocation("test")

func testAnalyzers(t *testing.T, code string, analyzers ...*analysis.Analyzer) []analysis.Diagnostic {

	config := analysis.NewSimpleConfig(
		analysis.NeedTypes|analysis.NeedExtendedElaboration,
		map[common.Location][]byte{
			testLocation: []byte(code),
		},
		nil,
		nil,
	)

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
