# Go toolchain
GO ?= go
MODULE := $(shell $(GO) list -m)

# Coverage
COVERAGE_FILE := coverage.out
COVERAGE_THRESHOLD := 90

.PHONY: all dev ci test coverage lint fmt fmt-check audit bench clean

# Default target
all: test

# Development workflow
dev: deps fmt lint test

# CI pipeline
ci: deps fmt-check audit lint coverage

# Install dependencies
deps:
	@$(GO) mod download
	@$(GO) mod verify

# Run tests
test:
	@$(GO) test -v -parallel 4 ./...

# Generate coverage report
coverage:
	@$(GO) test -coverprofile=$(COVERAGE_FILE) ./... > /dev/null 2>&1
	@COVERAGE=$$($(GO) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | sed 's/%//'); \
	INT_COVERAGE=$$(echo "$$COVERAGE" | cut -d. -f1); \
	echo ""; \
	echo "Code Coverage Report"; \
	echo "======================="; \
	echo "Coverage: $$COVERAGE%"; \
	echo "Target:   $(COVERAGE_THRESHOLD)%"; \
	if [ $$INT_COVERAGE -lt $(COVERAGE_THRESHOLD) ]; then \
		echo "Status:   FAILED"; \
		echo "Tip: Improve coverage by adding more test cases"; \
		echo "======================="; \
		exit 1; \
	else \
		echo "Status:   PASSED"; \
	    echo "======================="; \
	fi;

# Format code
fmt:
	@$(GO) fmt ./...

# Check code format without fixing
fmt-check:
	@echo "Checking code format..."
	@if [ -n "$$($(GO) fmt ./...)" ]; then \
		echo "FAILED: Code formatting issues found"; \
		echo "Run 'make fmt' to fix formatting"; \
		exit 1; \
	else \
		echo "PASSED: Code is properly formatted"; \
	fi

# Lint code
lint:
	@$(GO) vet ./...

# Security audit
audit:
	@$(GO) mod verify
	@$(GO) vet ./...

# Clean
clean:
	@rm -f $(COVERAGE_FILE)
	@$(GO) clean -cache -testcache