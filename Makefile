.PHONY: help build test validate clean benchmark

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "fb2c - FB2 to MOBI Converter"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build fb2c binary
	@echo "Building fb2c..."
	go build -o fb2c ./cmd/fb2c
	@echo "✓ Build complete: ./fb2c"

test: ## Run all tests
	@echo "Running tests..."
	go test ./... -v
	@echo "✓ Tests complete"

validate: build ## Validate converter against Calibre
	@echo "Running validation..."
	@./scripts/validate.sh

benchmark: build ## Run performance benchmark (fb2c vs Calibre)
	@echo "Running benchmark..."
	@if command -v ebook-convert > /dev/null 2>&1; then \
		echo "Benchmarking fb2c vs Calibre..."; \
		FB2_FILE=$$(ls testdata/*.fb2 2>/dev/null | head -1); \
		if [ -z "$$FB2_FILE" ]; then \
			echo "No FB2 files found in testdata/"; \
			exit 1; \
		fi; \
		echo "Testing with: $$FB2_FILE"; \
		echo ""; \
		echo "fb2c (Go) - 10 iterations:"; \
		time for i in $$(seq 1 10); do ./fb2c convert "$$FB2_FILE" /dev/null > /dev/null 2>&1; done; \
		echo ""; \
		echo "Calibre (Python) - 10 iterations:"; \
		time for i in $$(seq 1 10); do ebook-convert "$$FB2_FILE" /dev/null > /dev/null 2>&1; done; \
	else \
		echo "Calibre not found. Install with: sudo pacman -S calibre"; \
		exit 1; \
	fi

clean: ## Clean build artifacts and validation output
	@echo "Cleaning..."
	rm -f fb2c
	rm -rf validation_output
	@echo "✓ Clean complete"

validate-one: ## Validate a single file (usage: make validate-one FILE=path/to/file.fb2)
	@if [ -z "$(FILE)" ]; then \
		echo "Usage: make validate-one FILE=path/to/file.fb2"; \
		exit 1; \
	fi
	@echo "Validating $(FILE)..."
	@if [ ! -f "./fb2c" ]; then \
		echo "Building fb2c first..."; \
		go build -o fb2c ./cmd/fb2c; \
	fi
	@./fb2c convert "$(FILE)" /tmp/test.mobi
	@echo "✓ Generated: /tmp/test.mobi"
	@if command -v mobitool > /dev/null 2>&1; then \
		echo "Extracting with mobitool..."; \
		mobitool -x /tmp/test.mobi -o /tmp/test_extracted; \
		echo "✓ Extracted to: /tmp/test_extracted"; \
		ls -lh /tmp/test.mobi; \
		echo ""; \
		echo "Contents:"; \
		ls -lh /tmp/test_extracted/ 2>/dev/null || true; \
	fi

check-tools: ## Check if required tools are installed
	@echo "Checking for required tools..."
	@echo ""
	@if command -v go > /dev/null 2>&1; then \
		echo "✓ Go (go version)"; \
	else \
		echo "✗ Go not found"; \
	fi
	@if command -v ebook-convert > /dev/null 2>&1; then \
		echo "✓ Calibre (ebook-convert)"; \
	else \
		echo "✗ Calibre not found - install with: sudo pacman -S calibre"; \
	fi
	@if command -v mobitool > /dev/null 2>&1; then \
		echo "✓ mobitool (libmobi)"; \
	else \
		echo "✗ mobitool not found - install with: yay -S libmobi"; \
	fi
