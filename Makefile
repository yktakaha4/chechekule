.PHONY: install fix test build clean

# Default target
.DEFAULT_GOAL := install

# Variables
BINARY_NAME=chechekule
GO=go
GOFMT=$(GO) fmt
GOVET=$(GO) vet
GOTEST=$(GO) test
GOBUILD=$(GO) build

# Install dependencies
install:
	$(GO) mod download
	$(GO) mod tidy

# Format code and run go vet
fix:
	$(GOFMT) ./...
	$(GOVET) ./...

# Run tests
test:
	$(GOTEST) -v -race ./...

# Build binary
build:
	$(GOBUILD) -o $(BINARY_NAME)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	$(GO) clean 