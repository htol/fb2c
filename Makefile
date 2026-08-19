.PHONY: help build test lint validate clean benchmark test-validate-by-mobitool preview kindle kindle-reset kindle-udev kindle-probe

# Default target
.DEFAULT_GOAL := build

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
	go test ./...
	@echo "✓ Tests complete"

lint: ## Lint the codebase with golangci-lint (config: .golangci.yml)
	@echo "Running golangci-lint..."
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "✗ golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8"; \
		exit 1; \
	fi
	@echo "✓ Lint complete"

validate: build ## Validate converter against Calibre
	@echo "Running validation..."
	@./scripts/validate.sh

benchmark: build ## Run performance benchmark (fb2c vs Calibre)
	@echo "Running benchmark..."
	@if command -v ebook-convert > /dev/null 2>&1; then \
		echo "Benchmarking fb2c vs Calibre..."; \
		FB2_FILE=$$(ls testdata/fb2/*.fb2 2>/dev/null | head -1); \
		if [ -z "$$FB2_FILE" ]; then \
			echo "No FB2 files found in testdata/fb2/"; \
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

test-validate-by-mobitool: build ## Validate fb2c MOBI output with mobitool (independent strict parser)
	@./scripts/validate_mobitool.sh

preview: build ## Render corpus via Kindle Previewer CLI (closest to a real device; needs kindlepreviewer)
	@./scripts/preview.sh

kindle: build ## Convert src_ref.fb2 and send it to a USB-connected Kindle (ejects the device)
	@./scripts/kindle.sh

kindle-reset: ## Re-attach Kindle storage after an eject (USB reset, no conversion)
	@./scripts/kindle.sh --reset

kindle-udev: ## Install udev rules for cable-free Kindle attach/eject (sudo, one time)
	@./scripts/kindle-udev.sh

kindle-probe: build ## Deploy a hygiene-clean probe book with a unique title/ASIN (usage: make kindle-probe SUFFIX=b [INPUT=fixture.fb2])
	@if [ -z "$(SUFFIX)" ]; then echo "Usage: make kindle-probe SUFFIX=b [INPUT=path/to/file.fb2]"; exit 1; fi
	@./scripts/kindle.sh --probe $(SUFFIX) $(if $(INPUT),$(INPUT),)

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
