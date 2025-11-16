.RECIPEPREFIX := >
.PHONY: fmt lint test build all help

## Format Go source files with gofmt.
fmt:
> gofmt -w ./

## Run static analysis (golangci-lint if available, otherwise go vet).
lint:
> if command -v golangci-lint >/dev/null 2>&1; then \
> golangci-lint run ./...; \
> else \
> echo "golangci-lint not found; running go vet instead"; \
> go vet ./...; \
> fi

## Execute the Go test suite.
test:
> go test ./...

## Build the gh-templater binary.
build:
> go build ./...

## Run all local quality checks.
all: fmt lint test build

## Display available make targets with descriptions.
help:
> @awk '/^##/ {desc=$$0; sub(/^## /, "", desc); getline; if ($$0 ~ /^[a-zA-Z0-9_-]+:/) {split($$0, tgt, ":"); printf "%-10s %s\n", tgt[1], desc}}' $(MAKEFILE_LIST)
