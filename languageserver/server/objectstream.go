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

type ObjectStream struct {
	writeObject func(obj any) error
	readObject  func(v any) error
	close       func() error
}

func NewObjectStream(
	writeObject func(obj any) error,
	readObject func(v any) error,
	close func() error,
) ObjectStream {
	return ObjectStream{
		writeObject: writeObject,
		readObject:  readObject,
		close:       close,
	}
}

func (o ObjectStream) WriteObject(obj any) error {
	return o.writeObject(obj)
}

func (o ObjectStream) ReadObject(v any) (err error) {
	return o.readObject(v)
}

func (o ObjectStream) Close() error {
	return o.close()
}
