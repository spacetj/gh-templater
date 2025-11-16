.RECIPEPREFIX := >
.PHONY: fmt lint test build all

fmt:
> gofmt -w ./

lint:
> if command -v golangci-lint >/dev/null 2>&1; then \
> golangci-lint run ./...; \
> else \
> echo "golangci-lint not found; running go vet instead"; \
> go vet ./...; \
> fi

test:
> go test ./...

build:
> go build ./...

all: fmt lint test build
