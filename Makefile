.PHONY: build run test test-coverage lint fmt clean tidy docker-build install help hooks check

BINARY_NAME=cortex-ia
BINARY_DIR=bin
BINARY_PATH=$(BINARY_DIR)/$(BINARY_NAME)

GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

COVERAGE_DIR=coverage
COVERAGE_FILE=$(COVERAGE_DIR)/coverage.out
COVERAGE_HTML=$(COVERAGE_DIR)/coverage.html

DOCKER_IMAGE=cortex-ia
DOCKER_TAG=latest

all: build

help:
	@echo "cortex-ia — AI Agent Ecosystem Configurator"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  run            Run the TUI installer"
	@echo "  test           Run all tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  lint           Run golangci-lint"
	@echo "  fmt            Format code"
	@echo "  clean          Remove binaries and coverage"
	@echo "  tidy           Run go mod tidy"
	@echo "  docker-build   Build Docker image"
	@echo "  install        Install binary to GOPATH/bin"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_PATH) ./cmd/cortex-ia
	@echo "Binary built: $(BINARY_PATH)"

run:
	$(GOCMD) run ./cmd/cortex-ia

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report: $(COVERAGE_HTML)"
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE)

lint:
	@echo "Running golangci-lint..."
	$(GOLINT) run ./...

fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR) $(COVERAGE_DIR)
	$(GOCMD) clean

tidy:
	$(GOMOD) tidy

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

install:
	$(GOCMD) install ./cmd/cortex-ia

dev: build run

watch:
	@which air > /dev/null || (echo "Install air: go install github.com/cosmtrek/air@latest" && exit 1)
	air

security:
	@govulncheck ./... || echo "Install: go install golang.org/x/vuln/cmd/govulncheck@latest"

hooks:
	@echo "Installing git hooks..."
	git config core.hooksPath .hooks
	@echo "Git hooks installed from .hooks/"

check: fmt lint test security
	@echo "All checks passed."
