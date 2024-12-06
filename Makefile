.PHONY: help update vet
.SILENT:

SHELL := bash -eou pipefail

help:
	awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

update: ## Upgrade dependencies
	go get -u -t -d -v ./...
	go mod tidy

vet: ## Run vet
	go vet ./...
