BINARY := wolfcastle
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-X github.com/dorkusprime/wolfcastle/cmd.Version=$(VERSION) -X github.com/dorkusprime/wolfcastle/cmd.Commit=$(COMMIT) -X github.com/dorkusprime/wolfcastle/cmd.Date=$(DATE)"
GOFLAGS := -trimpath

.PHONY: build test install clean lint fmt vet

build:
	@echo "Building wolfcastle $(VERSION) ($(COMMIT))..."
	@go build $(GOFLAGS) $(LDFLAGS) -o $(BINARY) .
	@echo "Built ./$(BINARY)"

test:
	go test ./...

test-verbose:
	go test -v ./...

install:
	@echo "Installing wolfcastle $(VERSION) ($(COMMIT))..."
	@go install $(GOFLAGS) $(LDFLAGS) .
	@echo "Installed to $$(go env GOPATH)/bin/wolfcastle"

clean:
	rm -f $(BINARY)
	go clean

lint: vet fmt
	@echo "Lint passed"

vet:
	go vet ./...

fmt:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:"; gofmt -l .; exit 1)

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
