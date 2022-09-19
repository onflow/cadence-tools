.PHONY: test
test:
	(cd ./lint && make test && cd -)
	(cd ./test && make test && cd -)
	(cd ./languageserver && make test && cd -)

.PHONY: generate
generate:
	(cd ./lint && make generate && cd -)
	(cd ./test && make generate && cd -)
	(cd ./languageserver && make generate && cd -)

.PHONY: check-headers
check-headers:
	(cd ./lint && make check-headers && cd -)
	(cd ./test && make check-headers && cd -)
	(cd ./languageserver && make check-headers && cd -)

.PHONY: check-tidy
check-tidy: generate
	(cd ./lint && make check-tidy && cd -)
	(cd ./test && make check-tidy && cd -)
	(cd ./languageserver && make check-tidy && cd -)
