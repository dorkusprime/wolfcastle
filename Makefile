BINARY := wolfcastle
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/dorkusprime/wolfcastle/cmd.Version=$(VERSION) -X github.com/dorkusprime/wolfcastle/cmd.Commit=$(COMMIT) -X github.com/dorkusprime/wolfcastle/cmd.Date=$(DATE)"
GOFLAGS := -trimpath

.PHONY: build test install clean lint fmt vet golangci-lint ci help

build:
	@echo "Building wolfcastle $(VERSION) ($(COMMIT))..."
	@go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY) .
	@echo "Built ./$(BINARY)"

test:
	go test -race ./...

test-verbose:
	go test -v ./...

install:
	@echo "Installing wolfcastle $(VERSION) ($(COMMIT))..."
	@go install $(GOFLAGS) $(LDFLAGS) .
	@echo "Installed to $$(go env GOPATH)/bin/wolfcastle"

clean:
	rm -f $(BINARY)
	go clean

lint: vet fmt golangci-lint
	@echo "Lint passed"

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

golangci-lint:
	golangci-lint run ./...

ci: lint test build

# Cross-compilation targets
.PHONY: build-all build-linux build-darwin build-windows

build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .

help: ## Print available targets
	@echo "wolfcastle build targets:"
	@echo ""
	@echo "  build          Build wolfcastle binary"
	@echo "  test           Run tests"
	@echo "  test-verbose   Run tests with verbose output"
	@echo "  install        Install wolfcastle to GOPATH/bin"
	@echo "  clean          Remove built binary and build cache"
	@echo "  lint           Run vet, fmt, and golangci-lint checks"
	@echo "  vet            Run go vet"
	@echo "  fmt            Check gofmt compliance"
	@echo "  golangci-lint  Run golangci-lint"
	@echo "  build-all      Cross-compile for all platforms"
	@echo "  build-linux    Cross-compile for Linux (amd64, arm64)"
	@echo "  build-darwin   Cross-compile for macOS (amd64, arm64)"
	@echo "  build-windows  Cross-compile for Windows (amd64)"
	@echo "  ci             Run lint, test, and build (full local CI)"
	@echo "  help           Print this help"
