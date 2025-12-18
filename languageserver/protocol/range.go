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

package protocol

func (r Range) Overlaps(other Range) bool {
	return r.Start.Compare(other.End) <= 0 &&
		r.End.Compare(other.Start) >= 0
}

func (p Position) Compare(other Position) int {
	if p.Line < other.Line {
		return -1
	}

	if p.Line > other.Line {
		return 1
	}

	if p.Character < other.Character {
		return -1
	} else if p.Character > other.Character {
		return 1
	}

	return 0
}
