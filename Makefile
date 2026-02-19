ROOT_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

.PHONY: update
update: ## update dependencies
	git pull
	go mod tidy

.PHONY: test
test: ## run go tests with coverage
	go test -covermode=count $(ROOT_DIR)...

.PHONY: lint
lint: ## run linters
	golangci-lint run --path-prefix $(ROOT_DIR) -E gocyclo
