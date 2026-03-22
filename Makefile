.PHONY: build test lint clean dev build-all sidecar

VERSION ?= $(shell node -p "require('./package.json').version" 2>/dev/null || echo "0.1.0")

# Build the migratex binary
build:
	go build -o migratex .

# Cross-compile for all platforms
build-all:
	GOOS=darwin  GOARCH=arm64 go build -o bin/migratex-darwin-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o bin/migratex-darwin-amd64 .
	GOOS=linux   GOARCH=amd64 go build -o bin/migratex-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/migratex-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/migratex-windows-amd64.exe .

# Run all tests
test:
	go test ./internal/... ./cmd/...

# Run tests with verbose output
test-v:
	go test -v ./internal/... ./cmd/...

# Run linter
lint:
	go vet ./internal/... ./cmd/...

# Clean build artifacts
clean:
	rm -f migratex
	rm -f bin/migratex-*
	rm -rf sidecar/dist sidecar/node_modules

# Build + install to $GOPATH/bin
install:
	go install .

# Build the TypeScript sidecar
sidecar:
	cd sidecar && npm install && npm run build
