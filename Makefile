# fsm-toolkit Makefile
# Copyright (c) 2025 haitch
# Licensed under Apache 2.0

BINARY_FSM = fsm
BINARY_FSMEDIT = fsmedit
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

GO = go
GOFLAGS = -trimpath

.PHONY: all build clean test lint fmt vet install uninstall help

all: build

build: $(BINARY_FSM) $(BINARY_FSMEDIT)

$(BINARY_FSM):
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_FSM) ./cmd/fsm/

$(BINARY_FSMEDIT):
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_FSMEDIT) ./cmd/fsmedit/

clean:
	rm -f $(BINARY_FSM) $(BINARY_FSMEDIT)
	rm -rf dist/

test:
	$(GO) test -v ./...

test-short:
	$(GO) test -short ./...

test-race:
	$(GO) test -race ./...

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: fmt vet test

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BINARY_FSM) $(DESTDIR)$(PREFIX)/bin/
	install -m 755 $(BINARY_FSMEDIT) $(DESTDIR)$(PREFIX)/bin/

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY_FSM)
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY_FSMEDIT)

# Cross-compilation targets
PLATFORMS = \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64 \
	freebsd/amd64 \
	freebsd/arm64 \
	openbsd/amd64 \
	openbsd/arm64 \
	netbsd/amd64 \
	netbsd/arm64

dist:
	mkdir -p dist

dist-all: dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) $(LDFLAGS) \
			-o dist/$(BINARY_FSM)-$$os-$$arch$$ext ./cmd/fsm/; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) $(LDFLAGS) \
			-o dist/$(BINARY_FSMEDIT)-$$os-$$arch$$ext ./cmd/fsmedit/; \
	done

release: dist-all
	@cd dist && for f in *; do \
		sha256sum "$$f" > "$$f.sha256"; \
	done

help:
	@echo "fsm-toolkit Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  all         Build all binaries (default)"
	@echo "  build       Build fsm and fsmedit"
	@echo "  clean       Remove built binaries and dist/"
	@echo "  test        Run all tests"
	@echo "  test-short  Run tests with -short flag"
	@echo "  test-race   Run tests with race detector"
	@echo "  lint        Run golangci-lint"
	@echo "  fmt         Format source code"
	@echo "  vet         Run go vet"
	@echo "  check       Run fmt, vet, and test"
	@echo "  install     Install binaries to PREFIX/bin"
	@echo "  uninstall   Remove installed binaries"
	@echo "  dist-all    Build for all platforms"
	@echo "  release     Build all platforms and generate checksums"
	@echo "  help        Show this help"
	@echo ""
	@echo "Variables:"
	@echo "  PREFIX      Installation prefix (default: /usr/local)"
	@echo "  DESTDIR     Destination directory for staged installs"
	@echo "  VERSION     Version string (default: git describe)"

PREFIX ?= /usr/local
