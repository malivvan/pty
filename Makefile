# Developer entry points for this package.
#
# "make" builds, vets and tests on the host system; "make help" lists every
# target. Building for other systems needs no extra tooling: "make cross"
# compiles the package for every supported GOOS/GOARCH pair.

GO      ?= go
PKG     ?= .
GOFLAGS ?=

# Every system the package supports, plus the ones it only has to keep
# compiling on. The ConPTY implementation is Windows only, the pseudo-terminal
# implementation is not.
CROSS ?= \
	linux/amd64 linux/386 linux/arm linux/arm64 \
	linux/ppc64le linux/riscv64 linux/loong64 linux/s390x linux/mips64le \
	darwin/amd64 darwin/arm64 \
	freebsd/amd64 freebsd/386 freebsd/arm freebsd/arm64 freebsd/riscv64 \
	netbsd/amd64 netbsd/arm \
	openbsd/amd64 openbsd/arm \
	dragonfly/amd64 \
	solaris/amd64 illumos/amd64 aix/ppc64 \
	windows/amd64 windows/386 windows/arm64 \
	android/arm64 ios/arm64 \
	js/wasm wasip1/wasm plan9/amd64

.PHONY: all build test vet fmt tidy generate cross clean help

all: build vet test ## Build, vet and test on the host system.

build: ## Build the package for the host system.
	$(GO) build $(GOFLAGS) $(PKG)

test: ## Run the tests for the host system.
	$(GO) test $(GOFLAGS) $(PKG)

vet: ## Report suspicious constructs on the host system.
	$(GO) vet $(GOFLAGS) $(PKG)

fmt: ## Format every Go source file.
	$(GO) fmt $(PKG)
	gofmt -s -l -w .

tidy: ## Keep go.mod and go.sum in sync with the sources.
	$(GO) mod tidy

generate: ## Regenerate the generated BSD ABI files; see mktypes.bash for how.
	./mktypes.bash

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

clean: ## Remove the build cache entries of this package.
	$(GO) clean $(PKG)

help: ## List the available targets.
	@grep -hE '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?##"}; {printf "  %-9s %s\n", $$1, $$2}'
