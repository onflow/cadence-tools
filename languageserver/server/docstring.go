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

package server

import (
	"fmt"
	"strings"
)

const (
	paramAnnotation  = "@param "
	returnAnnotation = "@return "
)

func formatDocString(docString string) string {
	docString = strings.TrimSpace(docString)
	if docString == "" {
		return ""
	}

	var content strings.Builder
	var params []string
	var returnDoc string
	var prevLineEmpty bool
	var contentLines int

	for line := range strings.SplitSeq(docString, "\n") {
		trimmed := strings.TrimSpace(line)

		if paramInfo, ok := strings.CutPrefix(trimmed, paramAnnotation); ok {
			paramName, paramDesc, hasColon := strings.Cut(paramInfo, ":")
			paramName = strings.TrimSpace(paramName)
			if hasColon && len(paramName) > 0 {
				paramDesc = strings.TrimSpace(paramDesc)
				if len(paramDesc) > 0 {
					params = append(params, fmt.Sprintf("- `%s`: %s", paramName, paramDesc))
				} else {
					params = append(params, fmt.Sprintf("- `%s`", paramName))
				}
				continue
			}
			// No valid colon/name — fall through to treat as normal content
		} else if returnInfo, ok := strings.CutPrefix(trimmed, returnAnnotation); ok {
			returnDoc = strings.TrimSpace(returnInfo)
			continue
		}

		isEmpty := len(trimmed) == 0
		if prevLineEmpty && isEmpty {
			continue
		}

		if contentLines > 0 {
			content.WriteByte('\n')
		}
		content.WriteString(trimmed)
		prevLineEmpty = isEmpty
		contentLines++
	}

	// Trim trailing whitespace/newlines from content before appending sections
	resultStr := strings.TrimRight(content.String(), "\n ")

	var finalBuilder strings.Builder
	finalBuilder.WriteString(resultStr)

	if len(params) > 0 {
		if finalBuilder.Len() > 0 {
			finalBuilder.WriteString("\n\n")
		}
		finalBuilder.WriteString("**Parameters**\n")
		for _, p := range params {
			finalBuilder.WriteByte('\n')
			finalBuilder.WriteString(p)
		}
	}

	if len(returnDoc) > 0 {
		if finalBuilder.Len() > 0 {
			finalBuilder.WriteString("\n\n")
		}
		finalBuilder.WriteString("**Returns** ")
		finalBuilder.WriteString(returnDoc)
	}

	return finalBuilder.String()
}
