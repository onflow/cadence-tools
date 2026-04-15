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

package lint

import (
	"bytes"
	"strings"
)

const lintDisableNextLinePrefix = "// lint-disable-next"

type disableDirective struct {
	analyzerNames []string // nil/empty = disable all
}

type disableDirectives struct {
	// keyed by target line number (1-based, the line AFTER the comment)
	directives map[int]disableDirective
}

// parseDisableDirectives scans source code for // lint-disable-next comments.
// Returns directives keyed by the line number they apply to (the line after the comment).
func parseDisableDirectives(code []byte) disableDirectives {
	if len(code) == 0 {
		return disableDirectives{}
	}

	var result map[int]disableDirective

	lines := bytes.Split(code, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if !bytes.HasPrefix(trimmed, []byte(lintDisableNextLinePrefix)) {
			continue
		}

		rest := string(trimmed[len(lintDisableNextLinePrefix):])
		rest = strings.TrimSpace(rest)

		var names []string
		if rest != "" {
			for _, name := range strings.Split(rest, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					names = append(names, name)
				}
			}
		}

		// i is 0-based, Cadence lines are 1-based.
		// The directive applies to the next line: (i+1) + 1 = i+2
		targetLine := i + 2

		if result == nil {
			result = map[int]disableDirective{}
		}
		result[targetLine] = disableDirective{analyzerNames: names}
	}

	return disableDirectives{directives: result}
}

func (d disableDirectives) isDisabled(line int, analyzerName string) bool {
	if d.directives == nil {
		return false
	}

	directive, ok := d.directives[line]
	if !ok {
		return false
	}

	// No specific analyzers listed = disable all
	if len(directive.analyzerNames) == 0 {
		return true
	}

	for _, name := range directive.analyzerNames {
		if name == analyzerName {
			return true
		}
	}

	return false
}
