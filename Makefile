# Project metadata
BINARY_NAME  := bazinga
AUTHOR       := Ahmed ElSebaei
EMAIL        := tildaslashalef@gmail.com

# Directory structure
BUILD_DIR    := bin
COVERAGE_DIR := coverage

# Go toolchain
GO           := go
GOFMT        := gofmt
GOBUILD      := $(GO) build
GOTEST       := $(GO) test
GOVET        := $(GO) vet
GOGET        := $(GO) get
GOMOD        := $(GO) mod

# Version control
VERSION      ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME   := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH  := $(shell git rev-parse HEAD)

# Version calculation for releases
LATEST_TAG   := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
MAJOR        := $(shell echo $(LATEST_TAG) | sed 's/v\([0-9]*\)\..*/\1/')
MINOR        := $(shell echo $(LATEST_TAG) | sed 's/v[0-9]*\.\([0-9]*\)\..*/\1/')
PATCH        := $(shell echo $(LATEST_TAG) | sed 's/v[0-9]*\.[0-9]*\.\([0-9]*\).*/\1/')
NEXT_PATCH   := $(shell expr $(PATCH) + 1)
NEXT_VERSION := v$(MAJOR).$(MINOR).$(NEXT_PATCH)

# Build configuration
LDFLAGS      := -ldflags="-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH) -X 'main.Author=$(AUTHOR)' -X 'main.Email=$(EMAIL)'"
PLATFORMS    := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build test coverage clean deps lint format help release cross-build run install

#------------------------------------------------------------------------------
# Main targets
#------------------------------------------------------------------------------

all: test build ## Run tests and build

build: ## Build the binary for current platform
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/bazinga
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

run: build ## Build and run the application
	./$(BUILD_DIR)/$(BINARY_NAME)

install: build ## Install the application to GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/

#------------------------------------------------------------------------------
# Development targets
#------------------------------------------------------------------------------

dev: format lint test build ## Development build with all checks
quick: build ## Quick build without tests

#------------------------------------------------------------------------------
# Testing targets
#------------------------------------------------------------------------------

test: ## Run tests
	$(GOTEST) -race -v ./...

test-short: ## Run tests in short mode
	$(GOTEST) -race -v -short ./...

coverage: ## Generate test coverage report
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report available at $(COVERAGE_DIR)/coverage.html"


#------------------------------------------------------------------------------
# Code quality targets
#------------------------------------------------------------------------------

lint-deps: ## Ensure linting tools are installed
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@which revive > /dev/null || (echo "Installing revive..." && go install github.com/mgechev/revive@latest)
	@echo "Linting dependencies are installed"

lint: lint-deps ## Run linters
	golangci-lint run --config=.golangci.yml --fix --verbose

lint-check: lint-deps ## Check linting without fixing (useful for CI)
	golangci-lint run --config=.golangci.yml --verbose

revive: lint-deps ## Run just the revive linter
	revive -config .golangci.yml -formatter friendly ./...

format: ## Format code
	$(GOFMT) -w .

#------------------------------------------------------------------------------
# Release targets
#------------------------------------------------------------------------------

cross-build: ## Build for multiple platforms
	@mkdir -p $(BUILD_DIR)
	@echo "Building version: $(VERSION)"
	$(foreach platform,$(PLATFORMS),\
		$(eval GOOS = $(word 1,$(subst /, ,$(platform))))\
		$(eval GOARCH = $(word 2,$(subst /, ,$(platform))))\
		$(eval EXTENSION = $(if $(filter windows,$(GOOS)),.exe,))\
		$(eval OUTFILE = $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)$(EXTENSION))\
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(LDFLAGS) -o $(OUTFILE) ./cmd/bazinga && \
		echo "Built $(OUTFILE)" ; \
	)

release: clean deps test cross-build ## Create a new release
	@echo "Creating release $(NEXT_VERSION)"
	@git tag -a $(NEXT_VERSION) -m "Release $(NEXT_VERSION)"
	@echo "Tag created. Push with: git push origin $(NEXT_VERSION)"

#------------------------------------------------------------------------------
# CI/CD targets
#------------------------------------------------------------------------------

ci-build: deps test build ## CI build task
	@echo "CI build completed"

ci-release: ## CI release task for GitHub Actions
	@echo "Using version from GitHub Actions: $(VERSION)"
	@mkdir -p $(BUILD_DIR)
	$(foreach platform,$(PLATFORMS),\
		$(eval GOOS = $(word 1,$(subst /, ,$(platform))))\
		$(eval GOARCH = $(word 2,$(subst /, ,$(platform))))\
		$(eval EXTENSION = $(if $(filter windows,$(GOOS)),.exe,))\
		$(eval OUTFILE = $(BUILD_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)$(EXTENSION))\
		GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) $(LDFLAGS) -o $(OUTFILE) ./cmd/bazinga && \
		echo "Built $(OUTFILE)" ; \
	)
	@echo "CI release artifacts prepared"

#------------------------------------------------------------------------------
# Utility targets
#------------------------------------------------------------------------------

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -rf $(BENCH_DIR)/*.txt
	$(GO) clean -testcache

deps: ## Download and verify dependencies
	$(GOMOD) download
	$(GOMOD) verify
	$(GOMOD) tidy

help: ## Display this help
	@echo "Bazinga Makefile"
	@echo "================="
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
