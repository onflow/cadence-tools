.PHONY: test
test:
	(cd ./lint && make test && cd -)
	(cd ./test-framework && make test && cd -)

.PHONY: generate
generate:
	(cd ./lint && make generate && cd -)
	(cd ./test-framework && make generate && cd -)

.PHONY: check-headers
check-headers:
	(cd ./lint && make check-headers && cd -)
	(cd ./test-framework && make check-headers && cd -)

.PHONY: check-tidy
check-tidy: generate
	(cd ./lint && make check-tidy && cd -)
	(cd ./test-framework && make check-tidy && cd -)
