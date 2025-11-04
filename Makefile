# BlackBox Daemon Makefile
# Provides build, test, clean, and Docker targets with proper dependencies

# Variables
APP_NAME := blackbox-daemon
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME) -w -s"

# Docker settings
DOCKER_IMAGE := $(APP_NAME)
DOCKER_TAG ?= $(VERSION)
DOCKER_FULL_IMAGE := $(DOCKER_IMAGE):$(DOCKER_TAG)

# Go settings
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Directories
BIN_DIR := bin
BUILD_DIR := build
COVERAGE_DIR := coverage

# Colors for output
RED := \033[31m
GREEN := \033[32m
YELLOW := \033[33m
BLUE := \033[34m
RESET := \033[0m

.PHONY: help clean build test dockerize run fmt lint vet deps check coverage race install uninstall

# Default target
all: clean test build

help: ## Show this help message
	@echo "$(BLUE)BlackBox Daemon Build System$(RESET)"
	@echo ""
	@echo "$(GREEN)Available targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  $(YELLOW)%-15s$(RESET) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(GREEN)Examples:$(RESET)"
	@echo "  make build                    # Build the application"
	@echo "  make test                     # Run all tests"
	@echo "  make dockerize                # Build Docker image (runs tests first)"
	@echo "  make dockerize tag=v1.2.3     # Build Docker image with specific tag"
	@echo "  make clean                    # Clean build artifacts"

clean: ## Clean build artifacts and caches
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BIN_DIR) $(BUILD_DIR) $(COVERAGE_DIR)
	@go clean -cache -testcache -modcache
	@docker rmi $(DOCKER_FULL_IMAGE) 2>/dev/null || true
	@echo "$(GREEN)✓ Clean complete$(RESET)"

deps: ## Download and verify dependencies
	@echo "$(YELLOW)Downloading dependencies...$(RESET)"
	@go mod download
	@go mod verify
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies updated$(RESET)"

fmt: ## Format Go code
	@echo "$(YELLOW)Formatting code...$(RESET)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(RESET)"

vet: ## Run Go vet
	@echo "$(YELLOW)Running go vet...$(RESET)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet complete$(RESET)"

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "$(YELLOW)Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)✓ Lint complete$(RESET)"; \
	else \
		echo "$(RED)golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(RESET)"; \
	fi

check: fmt vet ## Run code quality checks
	@echo "$(GREEN)✓ All checks passed$(RESET)"

test: ## Run all tests
	@echo "$(YELLOW)Running tests...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	@go test -v -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)✓ Tests completed. Coverage report: $(COVERAGE_DIR)/coverage.html$(RESET)"

race: ## Run tests with race detector
	@echo "$(YELLOW)Running tests with race detector...$(RESET)"
	@go test -race -v ./...
	@echo "$(GREEN)✓ Race tests completed$(RESET)"

coverage: test ## Generate and display test coverage report
	@echo "$(YELLOW)Test Coverage Summary:$(RESET)"
	@go tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo ""
	@echo "$(BLUE)Detailed coverage report: $(COVERAGE_DIR)/coverage.html$(RESET)"

build: ## Build the application
	@echo "$(YELLOW)Building $(APP_NAME)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) ./cmd/blackbox-daemon
	@echo "$(GREEN)✓ Build complete: $(BIN_DIR)/$(APP_NAME)$(RESET)"
	@echo "$(BLUE)  Version: $(VERSION)$(RESET)"
	@echo "$(BLUE)  Commit:  $(COMMIT)$(RESET)"
	@echo "$(BLUE)  Built:   $(BUILD_TIME)$(RESET)"

build-all: ## Build for multiple platforms
	@echo "$(YELLOW)Building for multiple platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ]; then \
				ext=".exe"; \
			else \
				ext=""; \
			fi; \
			echo "Building $$os/$$arch..."; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) \
				-o $(BUILD_DIR)/$(APP_NAME)-$$os-$$arch$$ext ./cmd/blackbox-daemon; \
		done; \
	done
	@echo "$(GREEN)✓ Multi-platform build complete$(RESET)"

install: build ## Install the application to GOPATH/bin
	@echo "$(YELLOW)Installing $(APP_NAME)...$(RESET)"
	@go install $(LDFLAGS) ./cmd/blackbox-daemon
	@echo "$(GREEN)✓ Installed to $(shell go env GOPATH)/bin/$(APP_NAME)$(RESET)"

uninstall: ## Uninstall the application from GOPATH/bin
	@echo "$(YELLOW)Uninstalling $(APP_NAME)...$(RESET)"
	@rm -f $(shell go env GOPATH)/bin/$(APP_NAME)
	@echo "$(GREEN)✓ Uninstalled$(RESET)"

run: build ## Build and run the application locally
	@echo "$(YELLOW)Running $(APP_NAME)...$(RESET)"
	@./$(BIN_DIR)/$(APP_NAME)

# Docker targets (dockerize depends on test to ensure quality)
dockerize: test ## Build Docker image (requires tests to pass)
	@echo "$(YELLOW)Building Docker image...$(RESET)"
ifdef tag
	$(eval DOCKER_TAG=$(tag))
	$(eval DOCKER_FULL_IMAGE=$(DOCKER_IMAGE):$(tag))
endif
	@docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(DOCKER_FULL_IMAGE) \
		-f Dockerfile .
	@echo "$(GREEN)✓ Docker image built: $(DOCKER_FULL_IMAGE)$(RESET)"

docker-run: dockerize ## Build and run Docker container
	@echo "$(YELLOW)Running Docker container...$(RESET)"
	@docker run --rm -p 8080:8080 $(DOCKER_FULL_IMAGE)

docker-push: dockerize ## Build and push Docker image to registry
	@echo "$(YELLOW)Pushing Docker image...$(RESET)"
	@docker push $(DOCKER_FULL_IMAGE)
	@echo "$(GREEN)✓ Docker image pushed: $(DOCKER_FULL_IMAGE)$(RESET)"

docker-clean: ## Clean Docker images and containers
	@echo "$(YELLOW)Cleaning Docker artifacts...$(RESET)"
	@docker system prune -f
	@echo "$(GREEN)✓ Docker cleanup complete$(RESET)"

# Development targets
dev: ## Start development mode (build and run with file watching)
	@echo "$(YELLOW)Starting development mode...$(RESET)"
	@echo "$(BLUE)Note: Install 'air' for hot reloading: go install github.com/cosmtrek/air@latest$(RESET)"
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "$(YELLOW)'air' not found, falling back to simple run...$(RESET)"; \
		make run; \
	fi

benchmark: ## Run benchmarks
	@echo "$(YELLOW)Running benchmarks...$(RESET)"
	@go test -bench=. -benchmem ./...
	@echo "$(GREEN)✓ Benchmarks complete$(RESET)"

profile: ## Run with CPU and memory profiling
	@echo "$(YELLOW)Building with profiling enabled...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@go build -tags profile $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-profile ./cmd/blackbox-daemon
	@echo "$(GREEN)✓ Profile build complete: $(BIN_DIR)/$(APP_NAME)-profile$(RESET)"
	@echo "$(BLUE)Run with: ./$(BIN_DIR)/$(APP_NAME)-profile$(RESET)"

# Validation and security
sec-scan: ## Run security scan with gosec (requires gosec to be installed)
	@echo "$(YELLOW)Running security scan...$(RESET)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
		echo "$(GREEN)✓ Security scan complete$(RESET)"; \
	else \
		echo "$(RED)gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest$(RESET)"; \
	fi

mod-check: ## Check for module updates
	@echo "$(YELLOW)Checking for module updates...$(RESET)"
	@go list -u -m all
	@echo "$(GREEN)✓ Module check complete$(RESET)"

# CI/CD targets
ci: clean deps check test build ## Run full CI pipeline
	@echo "$(GREEN)✓ CI pipeline completed successfully$(RESET)"

release: clean deps check test build build-all ## Prepare release artifacts
	@echo "$(YELLOW)Creating release package...$(RESET)"
	@mkdir -p $(BUILD_DIR)/release
	@cp README.md LICENSE $(BUILD_DIR)/release/
	@cd $(BUILD_DIR) && tar -czf release/$(APP_NAME)-$(VERSION).tar.gz $(APP_NAME)-*
	@echo "$(GREEN)✓ Release package created: $(BUILD_DIR)/release/$(APP_NAME)-$(VERSION).tar.gz$(RESET)"

# Info targets
version: ## Show version information
	@echo "$(BLUE)Version:    $(VERSION)$(RESET)"
	@echo "$(BLUE)Commit:     $(COMMIT)$(RESET)"
	@echo "$(BLUE)Go Version: $(GO_VERSION)$(RESET)"
	@echo "$(BLUE)OS/Arch:    $(GOOS)/$(GOARCH)$(RESET)"

env: ## Show build environment information
	@echo "$(BLUE)Build Environment:$(RESET)"
	@echo "  GOOS:        $(GOOS)"
	@echo "  GOARCH:      $(GOARCH)"
	@echo "  CGO_ENABLED: $(shell go env CGO_ENABLED)"
	@echo "  GOPATH:      $(shell go env GOPATH)"
	@echo "  GOROOT:      $(shell go env GOROOT)"
	@echo "  Go Version:  $(GO_VERSION)"
	@echo "  Docker:      $(shell docker --version 2>/dev/null || echo 'Not installed')"

# Quick development shortcuts
q: clean test build ## Quick: clean, test, build
	@echo "$(GREEN)✓ Quick build completed$(RESET)"

qd: clean test dockerize ## Quick Docker: clean, test, dockerize
	@echo "$(GREEN)✓ Quick Docker build completed$(RESET)"

# Make sure intermediate files are not deleted
.PRECIOUS: $(BIN_DIR)/$(APP_NAME) $(COVERAGE_DIR)/coverage.out