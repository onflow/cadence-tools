.PHONY: build
build:
	go build .

.PHONY: test
test:
	# test all packages
	GO111MODULE=on go test -parallel 8 ./...

.PHONY: generate
generate:
	go generate -v ./...

.PHONY: check-headers
check-headers:
	@../check-headers.sh

.PHONY: check-tidy
check-tidy: generate
	go mod tidy
	git diff --exit-code
