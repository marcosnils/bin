.PHONY: help build verify download coverage

NO_COLOR=\033[0m
GREEN=\033[32;01m
YELLOW=\033[33;01m
RED=\033[31;01m

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[33m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build .

clean: ## Clean artefacts
	rm -rf bin

lint: # Run lint
	go fmt ./...
	go vet ./...

test: ## Run all tests
	go test ./...

download: ## Download dependencies
	go mod download
	go mod tidy

verify: download ## Code verification
	gofmt -w -s ./.
	golangci-lint run
