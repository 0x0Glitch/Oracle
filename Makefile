.PHONY: build run test clean install-deps help

# Binary name
BINARY_NAME=oracle_monitor

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-deps: ## Install Go dependencies
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the oracle monitor binary
	$(GOBUILD) -o $(BINARY_NAME) -v .

build-optimized: ## Build optimized binary for production
	CGO_ENABLED=0 $(GOBUILD) -ldflags="-s -w" -o $(BINARY_NAME) -v .

run: ## Run the oracle monitor (development)
	$(GORUN) .

run-base: ## Run monitoring for Base chain only
	ENABLED_CHAINS=base $(GORUN) .

run-multi: ## Run monitoring for all chains
	ENABLED_CHAINS=base,optimism,moonbeam,moonriver $(GORUN) .

test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

fmt: ## Format code
	$(GOCMD) fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

docker-build: ## Build Docker image
	docker build -t oracle-monitor:latest .

docker-run: ## Run in Docker container
	docker run --env-file .env oracle-monitor:latest

migrate-config: ## Create default config.json if it doesn't exist
	@if [ ! -f config.json ]; then \
		echo "Creating default config.json..."; \
		$(GORUN) -c 'package main; import "github.com/0x0Glitch/config"; import "encoding/json"; import "os"; func main() { cfg := config.DefaultConfig(); b, _ := json.MarshalIndent(cfg, "", "  "); os.WriteFile("config.json", b, 0644) }'; \
	else \
		echo "config.json already exists"; \
	fi
