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

package query

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FormatMatches writes matches to the writer in the given format.
func FormatMatches(w io.Writer, matches []Match, format OutputFormat) error {
	switch format {
	case OutputJSON:
		return formatJSON(w, matches)
	case OutputText:
		return formatText(w, matches)
	default:
		return fmt.Errorf("unknown output format: %d", format)
	}
}

func formatJSON(w io.Writer, matches []Match) error {
	values := make([]any, len(matches))
	for i, m := range matches {
		values[i] = m.Value
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(values)
}

func formatText(w io.Writer, matches []Match) error {
	for i, m := range matches {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}

		// Try to extract location info from the matched value
		mMap, isMap := m.Value.(map[string]any)
		if !isMap {
			// Scalar or non-map result — just print as JSON
			b, _ := json.Marshal(m.Value)
			_, _ = fmt.Fprintf(w, "%s: %s\n", m.Location.Description(), string(b))
			continue
		}

		// Extract start position from Range
		line, col, hasPos := extractStartPosition(mMap)
		if hasPos {
			_, _ = fmt.Fprintf(w, "%s:%d:%d:\n", m.Location.Description(), line, col+1)
		} else {
			_, _ = fmt.Fprintf(w, "%s:\n", m.Location.Description())
		}

		// Print source snippet if possible
		if hasPos && m.Code != nil {
			snippet := extractLineSnippet(m.Code, line)
			if snippet != "" {
				_, _ = fmt.Fprintf(w, "  %s\n", snippet)
			}
		}

		// Print key fields
		if nodeType, ok := mMap["Type"].(string); ok {
			_, _ = fmt.Fprintf(w, "  node: %s\n", nodeType)
		}
		if ActualType, ok := mMap["ActualType"].(string); ok {
			_, _ = fmt.Fprintf(w, "  type: %s\n", ActualType)
		}
		if op, ok := mMap["Operation"].(string); ok {
			_, _ = fmt.Fprintf(w, "  operation: %s\n", op)
		}
		if sourceType, ok := mMap["SourceType"].(string); ok {
			_, _ = fmt.Fprintf(w, "  source type: %s\n", sourceType)
		}
		if targetType, ok := mMap["TargetType"].(string); ok {
			_, _ = fmt.Fprintf(w, "  target type: %s\n", targetType)
		}
	}
	return nil
}

// extractStartPosition extracts line and column from a map node's StartPos field.
func extractStartPosition(m map[string]any) (line, col int, ok bool) {
	startPos, isMap := m["StartPos"].(map[string]any)
	if !isMap {
		return 0, 0, false
	}

	lineF, ok := startPos["Line"].(float64)
	if !ok {
		return 0, 0, false
	}

	colF, ok := startPos["Column"].(float64)
	if !ok {
		return 0, 0, false
	}

	return int(lineF), int(colF), true
}

// extractLineSnippet returns a single source line, trimmed.
func extractLineSnippet(code []byte, line int) string {
	lines := strings.Split(string(code), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}
