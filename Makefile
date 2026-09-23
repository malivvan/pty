# Developer entry points for this package.
#
# "make" builds, vets and tests on the host system, "make check" runs everything
# the CI workflow runs, and "make help" lists every target.

GO      ?= go
PKG     ?= .
GOFLAGS ?=

# The system the build runs on, which is what "make generate" has to run on as
# well: cgo -godefs reads the C headers of the host, not of a target. GOHOSTOS
# and GOHOSTARCH are what the toolchain reports for itself, whatever GOOS and
# GOARCH are set to.
HOST_GOOS   := $(shell $(GO) env GOHOSTOS)
HOST_GOARCH := $(shell $(GO) env GOHOSTARCH)

GOOS   ?= $(HOST_GOOS)
GOARCH ?= $(HOST_GOARCH)

# The Go toolchain keeps the runners of its WebAssembly targets here; they have
# to be on the path for "go test" to be able to run a wasm binary.
WASM_EXEC ?= $(shell $(GO) env GOROOT)/lib/wasm

# Every system the package supports, plus the ones it only has to keep
# compiling on. The ConPTY implementation is Windows only and the virtual
# terminal works everywhere, the rest is not.
CROSS ?= \
	linux/amd64 linux/386 linux/arm linux/arm64 \
	linux/ppc64le linux/riscv64 linux/loong64 linux/s390x linux/mips64le \
	darwin/amd64 darwin/arm64 \
	freebsd/amd64 freebsd/386 freebsd/arm freebsd/arm64 freebsd/riscv64 \
	netbsd/amd64 netbsd/arm \
	openbsd/amd64 openbsd/arm openbsd/ppc64 \
	dragonfly/amd64 \
	solaris/amd64 illumos/amd64 aix/ppc64 \
	windows/amd64 windows/386 windows/arm64 \
	android/arm64 ios/arm64 \
	js/wasm wasip1/wasm plan9/amd64

# The same systems, minus the ones whose tests need an external linker, which
# is what vetting them would need (ios, for instance).
CROSS_VET ?= \
	linux/amd64 linux/386 linux/arm linux/arm64 \
	linux/ppc64le linux/riscv64 linux/loong64 linux/s390x linux/mips64le \
	darwin/amd64 darwin/arm64 \
	freebsd/amd64 freebsd/386 freebsd/arm freebsd/arm64 freebsd/riscv64 \
	netbsd/amd64 netbsd/arm \
	openbsd/amd64 openbsd/arm openbsd/ppc64 \
	dragonfly/amd64 \
	solaris/amd64 illumos/amd64 aix/ppc64 \
	windows/amd64 windows/386 windows/arm64 android/arm64 \
	js/wasm wasip1/wasm plan9/amd64

.PHONY: all build test test-race test-shuffle test-wasm vet fmt fmt-check tidy generate cross cross-vet check clean help

all: build vet test ## Build, vet and test on the host system.

build: ## Build the package for the host system.
	$(GO) build $(GOFLAGS) $(PKG)

test: ## Run the tests for the host system.
	$(GO) test $(GOFLAGS) -count=1 $(PKG)

test-race: ## Run the tests for the host system with the race detector.
	$(GO) test $(GOFLAGS) -count=1 -race $(PKG)

test-shuffle: ## Run the tests for the host system in a random order.
	$(GO) test $(GOFLAGS) -count=1 -shuffle=on $(PKG)

test-wasm: ## Run the tests on the WebAssembly targets; needs node and wasmtime.
	PATH="$$PATH:$(WASM_EXEC)" GOOS=js GOARCH=wasm $(GO) test $(GOFLAGS) -count=1 $(PKG)
	PATH="$$PATH:$(WASM_EXEC)" GOOS=wasip1 GOARCH=wasm $(GO) test $(GOFLAGS) -count=1 $(PKG)

vet: ## Report suspicious constructs on the host system.
	$(GO) vet $(GOFLAGS) $(PKG)

fmt: ## Format every Go source file.
	$(GO) fmt $(PKG)
	gofmt -s -l -w .

fmt-check: ## Fail when a Go source file is not formatted.
	@carriage_returns="$$(grep -l "$$(printf '\r')" *.go *.s 2>/dev/null || true)"; \
	if [ -n "$$carriage_returns" ]; then \
		echo "These files have carriage returns, but the sources use line feeds only; see .gitattributes:"; \
		echo "$$carriage_returns"; \
		exit 1; \
	fi; \
	unformatted="$$(gofmt -s -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

tidy: ## Keep go.mod and go.sum in sync with the sources.
	$(GO) mod tidy

generate: ## Regenerate the generated ABI files of the BSDs; run it on the system it describes.
	@set -e; \
	if [ "$(GOOS)/$(GOARCH)" != "$(HOST_GOOS)/$(HOST_GOARCH)" ]; then \
		echo "generate: run this on $(GOOS)/$(GOARCH) itself: cgo -godefs reads the C headers of the host, not those of $(GOOS)/$(GOARCH)" >&2; \
		exit 1; \
	fi; \
	case "$(GOOS)" in \
	freebsd | dragonfly) \
		# The layouts and the maximum device name length differ between the \
		# architectures of these systems, hence one file per architecture. \
		out="ztypes_$(GOOS)_$(GOARCH).go"; tags="$(GOARCH) && $(GOOS)" ;; \
	netbsd | openbsd) \
		# One file per system: the layouts are the same on every architecture, \
		# and the file name already implies the build constraint. \
		out="ztypes_$(GOOS).go"; tags="$(GOOS)" ;; \
	*) \
		echo "generate: no generated types are defined for $(GOOS)" >&2; \
		exit 1 ;; \
	esac; \
	echo "generate: writing $$out" >&2; \
	body="$$($(GO) tool cgo -godefs "types_$(GOOS).go")"; \
	{ printf '//go:build %s\n\n' "$$tags"; printf '%s\n' "$$body"; } | gofmt > "$$out.new" || { rm -f "$$out.new"; exit 1; }; \
	mv "$$out.new" "$$out"

cross: ## Compile the package for every system listed in CROSS.
	@failed=0; \
	for target in $(CROSS); do \
		os=$${target%%/*}; arch=$${target##*/}; \
		output=$$(GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build $(PKG) 2>&1) || failed=1; \
		if [ -n "$$output" ]; then \
			printf 'FAIL %s\n%s\n' "$$target" "$$output"; \
		else \
			printf '  ok %s\n' "$$target"; \
		fi; \
	done; \
	if [ $$failed -ne 0 ]; then echo 'cross: at least one target failed'; exit 1; fi

cross-vet: ## Type-check the package and its tests for every system in CROSS_VET.
	@failed=0; \
	for target in $(CROSS_VET); do \
		os=$${target%%/*}; arch=$${target##*/}; \
		output=$$(GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) vet $(PKG) 2>&1) || failed=1; \
		if [ -n "$$output" ]; then \
			printf 'FAIL %s\n%s\n' "$$target" "$$output"; \
		else \
			printf '  ok %s\n' "$$target"; \
		fi; \
	done; \
	if [ $$failed -ne 0 ]; then echo 'cross-vet: at least one target failed'; exit 1; fi

check: fmt-check vet test test-race test-shuffle cross cross-vet ## Run what the CI workflow runs on this system; test-wasm needs node and wasmtime.

clean: ## Remove the build cache entries of this package.
	$(GO) clean $(PKG)

help: ## List the available targets.
	@grep -hE '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?##"}; {printf "  %-10s %s\n", $$1, $$2}'
