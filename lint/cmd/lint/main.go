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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"plugin"
	"sort"

	"github.com/onflow/cadence/tools/analysis"
	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/cadence-tools/lint"
)

var csvPathFlag = flag.String("csv", "", "analyze all programs in the given CSV file")
var directoryPathFlag = flag.String("directory", "", "analyze all programs in the given directory")
var networkFlag = flag.String("network", "", "name of network")
var addressFlag = flag.String("address", "", "analyze contracts in the given account")
var transactionFlag = flag.String("transaction", "", "analyze transaction with given ID")
var loadOnlyFlag = flag.Bool("load-only", false, "only load (parse and check) programs")
var silentFlag = flag.Bool("silent", false, "only show parsing/checking success/failure")
var colorFlag = flag.Bool("color", true, "format using colors")
var cyclomaticThresholdFlag = flag.Int("cyclo-threshold", 10, "minimum cyclomatic complexity to report (0 = report all)")
var cyclomaticCountLogicalFlag = flag.Bool("cyclo-count-logical", true, "count && and || as complexity points")
var analyzersFlag stringSliceFlag
var pluginsFlag stringSliceFlag

func init() {
	flag.Var(&analyzersFlag, "analyze", "enable analyzer")
	flag.Var(&pluginsFlag, "plugin", "load plugin")
}

func main() {
	defaultUsage := flag.Usage
	flag.Usage = func() {
		loadPlugins()

		defaultUsage()
		_, _ = fmt.Fprintf(os.Stderr, "\nAvailable analyzers:\n")

		names := make([]string, 0, len(lint.Analyzers))
		for name := range lint.Analyzers {
			names = append(names, name)
		}

		sort.Strings(names)

		for _, name := range names {
			analyzer := lint.Analyzers[name]
			_, _ = fmt.Fprintf(
				os.Stderr,
				"  - %s:\n      %s\n",
				name,
				analyzer.Description,
			)
		}
	}

	flag.Parse()

	// Register cyclomatic complexity analyzer with command-line flag values
	lint.RegisterAnalyzer(
		"cyclomatic-complexity",
		lint.NewCyclomaticComplexityAnalyzer(
			*cyclomaticThresholdFlag,
			*cyclomaticCountLogicalFlag,
		),
	)

	loadPlugins()

	var enabledAnalyzers []*analysis.Analyzer

	loadOnly := *loadOnlyFlag
	if !loadOnly {
		if len(analyzersFlag) > 0 {
			for _, analyzerName := range analyzersFlag {
				analyzer, ok := lint.Analyzers[analyzerName]
				if !ok {
					log.Panic(fmt.Errorf("unknown analyzer: %s", analyzerName))
				}

				enabledAnalyzers = append(enabledAnalyzers, analyzer)
			}
		} else {
			// Use all analyzers
			for _, analyzer := range lint.Analyzers {
				enabledAnalyzers = append(enabledAnalyzers, analyzer)
			}
		}
	}

	linter := lint.NewLinter(lint.Config{
		Analyzers: enabledAnalyzers,
		Silent:    *silentFlag,
		UseColor:  *colorFlag,
	})

	cvsPath := *csvPathFlag
	directoryPath := *directoryPathFlag
	address := *addressFlag
	transaction := *transactionFlag

	switch {
	case cvsPath != "":
		linter.AnalyzeCSV(cvsPath)

	case directoryPath != "":
		linter.AnalyzeDirectory(directoryPath)

	case address != "":
		network := *networkFlag
		linter.AnalyzeAccount(address, network)

	case transaction != "":
		transactionID := flow.HexToID(transaction)
		network := *networkFlag
		linter.AnalyzeTransaction(transactionID, network)

	default:
		_, _ = fmt.Fprintln(
			os.Stdout,
			"Nothing to do. Please provide -address, -transaction, -directory, or -csv. See -help",
		)
	}
}

func loadPlugins() {
	for _, pluginPath := range pluginsFlag {
		_, err := plugin.Open(pluginPath)
		if err != nil {
			log.Panic(fmt.Errorf("failed to load plugin: %s: %w", pluginPath, err))
			return
		}
	}
}
