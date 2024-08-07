#
# Cadence - The resource-oriented smart contract programming language
#
# Copyright Flow Foundation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

GOPATH ?= $(HOME)/go

# Ensure go bin path is in path (Especially for CI)
PATH := $(PATH):$(GOPATH)/bin


.PHONY: build
build:
	go build -o ./cmd/languageserver/languageserver ./cmd/languageserver
	GOARCH=wasm GOOS=js go build -o ./cmd/languageserver/languageserver.wasm -tags no_cgo ./cmd/languageserver

.PHONY: generate
generate:
	go get -d github.com/vektra/mockery/cmd/mockery
	go generate ./...
	go mod tidy

.PHONY: test
test: e2e-test generate unit-test # TODO: wasm-test

.PHONY: unit-test
unit-test:
	go test ./...

.PHONY: e2e-test
e2e-test:
	cd test && npm install && npm t
	GO111MODULE=on go test -parallel 8 ./...

.PHONY: wasm-test
wasm-test:
	cd ../npm-packages/cadence-language-server && npm install && npm run build && npm test

.PHONY: check-headers
check-headers:
	@../check-headers.sh

.PHONY: check-tidy
check-tidy: generate
	go mod tidy
	git diff --exit-code
