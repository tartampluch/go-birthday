# ==============================================================================
# Go Birthday - Makefile (Unix / CI)
# ==============================================================================
# This Makefile automates the build process for Linux and macOS.
# It handles version injection, resource embedding (optional), and optimization.

# Output binary name
BINARY_NAME=go-birthday

# ------------------------------------------------------------------------------
# 1. Metadata Retrieval
# ------------------------------------------------------------------------------

# Retrieve the semantic version from the VERSION file.
# Defaults to "dev" if the file is missing.
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")

# Retrieve the short Git commit hash to track the exact build source.
# Defaults to "none" if git is not available.
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")

# Get the current build time in UTC (ISO 8601 format).
# This is useful for debugging when a binary was built.
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# The Go package path where build variables (Version, Commit, Date) are defined.
# We inject values into this package at compile time.
CONFIG_PKG := github.com/tartampluch/go-birthday/internal/config

# ------------------------------------------------------------------------------
# 2. Build Flags (LDFLAGS)
# ------------------------------------------------------------------------------

# Linker Flags configuration:
# -s : Omit the symbol table and debug information (reduces binary size).
# -w : Omit the DWARF symbol table (further reduces binary size).
# -X : Inject string values into the specified Go variables.
LDFLAGS := -s -w \
	-X '$(CONFIG_PKG).Version=$(VERSION)' \
	-X '$(CONFIG_PKG).Commit=$(COMMIT)' \
	-X '$(CONFIG_PKG).Date=$(DATE)'

# Windows GUI Flag Logic:
# If we are cross-compiling for Windows (GOOS=windows), add -H=windowsgui.
# This prevents the command prompt window from appearing when launching the app.
ifeq ($(GOOS),windows)
	LDFLAGS += -H=windowsgui
endif

# ------------------------------------------------------------------------------
# 3. Targets
# ------------------------------------------------------------------------------

# Default target executed when running 'make' without arguments.
.PHONY: all
all: clean build

# Build the application binary.
.PHONY: build
build:
	@echo ">> Building $(BINARY_NAME) v$(VERSION) [Commit: $(COMMIT)]..."
	
	@# Optional: Windows Resource Generation (Icon/Manifest).
	@# Checks if 'go-winres' is installed. If so, it generates the .syso files.
	@# This allows cross-compiling a Windows binary with an icon from Linux/macOS.
	@if command -v go-winres > /dev/null; then \
		echo "   [Info] Generating Windows resources (Icon/Manifest)..."; \
		go-winres make --product-version $(VERSION) --file-version $(VERSION); \
		mv rsrc_windows_*.syso cmd/go-birthday/ 2>/dev/null || true; \
	fi

	@# Run 'go generate' to execute any pre-build directives in the code.
	go generate ./...

	@# Compile the binary with the configured flags.
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/go-birthday
	@echo ">> Build successful."

# Run the full test suite.
.PHONY: test
test:
	@echo ">> Running tests..."
	go test ./... -v

# Remove build artifacts.
.PHONY: clean
clean:
	@echo ">> Cleaning up..."
	rm -f $(BINARY_NAME)
	# Remove any Windows resource files generated during build.
	rm -f cmd/go-birthday/*.syso
