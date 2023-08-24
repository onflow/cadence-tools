.PHONY: test
test: test-test test-lint test-languageserver test-docgen

.PHONY: test-test
test-test:
	(cd ./test && make test && cd -)

.PHONY: test-lint
test-lint:
	(cd ./lint && make test && cd -)

.PHONY: test-languageserver
test-languageserver:
	(cd ./languageserver && make test && cd -)

.PHONY: test-docgen
test-docgen:
	(cd ./docgen && make test && cd -)

.PHONY: generate
generate: generate-lint generate-test generate-languageserver generate-docgen

.PHONY: generate-lint
generate-lint:
	(cd ./lint && make generate && cd -)

.PHONY: generate-test
generate-test:
	(cd ./test && make generate && cd -)

.PHONY: generate-languageserver
generate-languageserver:
	(cd ./languageserver && make generate && cd -)

.PHONY: generate-docgen
generate-docgen:
	(cd ./docgen && make generate && cd -)

.PHONY: check-headers
check-headers:
	(cd ./lint && make check-headers && cd -)
	(cd ./test && make check-headers && cd -)
	(cd ./languageserver && make check-headers && cd -)
	(cd ./docgen && make check-headers && cd -)

.PHONY: check-tidy
check-tidy: check-tidy-lint check-tidy-test check-tidy-languageserver check-tidy-docgen

.PHONY: check-tidy-lint
check-tidy-lint: generate-lint
	(cd ./lint && make check-tidy && cd -)

.PHONY: check-tidy-test
check-tidy-test: generate-test
	(cd ./test && make check-tidy && cd -)

.PHONY: check-tidy-languageserver
check-tidy-languageserver: generate-languageserver
	(cd ./languageserver && make check-tidy && cd -)

.PHONY: check-tidy-docgen
check-tidy-docgen: generate-docgen
	(cd ./docgen && make check-tidy && cd -)
