/*
 * Cadence - The resource-oriented smart contract programming language
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

package protocol

import (
	"encoding/json"
	"fmt"
)

// DocumentChanges is a union of a file edit and directory rename operations
// for package renaming feature. At most one field of this struct is non-nil.
type DocumentChanges struct {
	TextDocumentEdit *TextDocumentEdit
	RenameFile       *RenameFile
}

func (d *DocumentChanges) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}

	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if _, ok := m["textDocument"]; ok {
		d.TextDocumentEdit = new(TextDocumentEdit)
		return json.Unmarshal(data, d.TextDocumentEdit)
	}

	d.RenameFile = new(RenameFile)
	return json.Unmarshal(data, d.RenameFile)
}

func (d *DocumentChanges) MarshalJSON() ([]byte, error) {
	if d.TextDocumentEdit != nil {
		return json.Marshal(d.TextDocumentEdit)
	} else if d.RenameFile != nil {
		return json.Marshal(d.RenameFile)
	}
	return nil, fmt.Errorf("Empty DocumentChanges union value")
}
