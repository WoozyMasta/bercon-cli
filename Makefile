GO                ?= go
BINARY            ?= bercon-cli
OUTPUT_DIR        ?= build
CGO_ENABLED       ?= 0
GOFLAGS           ?= -buildvcs=auto -trimpath
GO_FLAGS_SHELL    ?= bash
LDFLAGS           ?= -s -w
GOWORK            ?= off
LANG              ?= C

LINTER            ?= golangci-lint
ALIGNER           ?= betteralign
VULNCHECK         ?= govulncheck
WINRES            ?= go-winres
WINRES_OUT        ?= ./cmd/$(BINARY)/rsrc
BENCHSTAT         ?= benchstat

RUMDL             ?= rumdl
MKDOCS_IMAGE      ?= docker.io/squidfunk/mkdocs-material:9.7
MKDOCS_ADDR       ?= 127.0.0.1:8000

RELEASE_MATRIX    ?= \
	linux/amd64 linux/arm64 \
	darwin/amd64 darwin/arm64 \
	windows/amd64 windows/arm64

MODULE_PATH       := $(shell GOWORK=$(GOWORK) $(GO) list -m -f '{{.Path}}')
NATIVE_GOOS       := $(shell GOWORK=$(GOWORK) $(GO) env GOOS)
NATIVE_GOARCH     := $(shell GOWORK=$(GOWORK) $(GO) env GOARCH)
BUILD_GOOS        ?= $(NATIVE_GOOS)
BUILD_GOARCH      ?= $(NATIVE_GOARCH)
BUILD_EXTENSION   := $(if $(filter $(BUILD_GOOS),windows),.exe,)
VERSION           := $(shell git describe --tags --abbrev=0 2>/dev/null || printf v0.0.0)
COMMIT            := $(shell git rev-parse HEAD 2>/dev/null || printf unknown)
DATE              := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
URL               := https://$(MODULE_PATH)
LDFLAGS_X := \
	-X '$(MODULE_PATH)/internal/app.version=$(VERSION)' \
	-X '$(MODULE_PATH)/internal/app.commit=$(COMMIT)' \
	-X '$(MODULE_PATH)/internal/app.buildTimeOverride=$(DATE)' \
	-X '$(MODULE_PATH)/internal/app.repositoryURL=$(URL)'

RACE ?= 0
ifeq ($(RACE),1)
	EXTRA_BUILD_FLAGS := -race
endif

BENCH_COUNT       ?= 6
BENCH_REF         ?= bench_baseline.txt


export GOWORK
export LANG
export GO_FLAGS_SHELL

.PHONY: clean build compile release

clean:
	rm -rf $(OUTPUT_DIR)

build: clean $(if $(filter windows,$(BUILD_GOOS)),winres)
	@mkdir -p $(OUTPUT_DIR)
	@echo ">> build: $(BINARY)$(BUILD_EXTENSION) ($(BUILD_GOOS)/$(BUILD_GOARCH))"
	GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) \
	GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" $(EXTRA_BUILD_FLAGS) \
	-o $(OUTPUT_DIR)/$(BINARY)$(BUILD_EXTENSION) ./cmd/$(BINARY)

release: clean winres
	@mkdir -p $(OUTPUT_DIR)
	@set -e; \
	for target in $(RELEASE_MATRIX); do \
		goos=$${target%%/*}; \
		goarch=$${target##*/}; \
		ext=$$( [ $$goos = "windows" ] && echo ".exe" || echo "" ); \
		out="$(OUTPUT_DIR)/$(BINARY)-$${goos}-$${goarch}$$ext"; \
		echo ">> build $$out"; \
		GOOS=$$goos GOARCH=$$goarch GOWORK=$(GOWORK) CGO_ENABLED=$(CGO_ENABLED) \
			$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS) $(LDFLAGS_X)" -o $$out ./cmd/$(BINARY); \
	done

.PHONY: check ci

check: verify tidy fmt vet vulncheck lint-fix align-fix generate-docs markdown-lint test test-race
ci: download generate-check tools-ci verify tidy-check fmt-check vet vulncheck lint align test test-race

.PHONY: test test-race

test:
	$(GO) test ./...

test-race:
	CGO_ENABLED=1 $(GO) test -race ./...

.PHONY: bench bench-fast bench-reset

bench:
	@tmp=$$(mktemp); \
	$(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem -count=$(BENCH_COUNT) | tee "$$tmp"; \
	if [ -f "$(BENCH_REF)" ]; then \
		$(BENCHSTAT) "$(BENCH_REF)" "$$tmp"; \
	else \
		cp "$$tmp" "$(BENCH_REF)" && echo "Baseline saved to $(BENCH_REF)"; \
	fi; \
	rm -f "$$tmp"

bench-fast:
	$(GO) test ./... -run=^$$ -bench 'Benchmark' -benchmem

bench-reset:
	rm -f "$(BENCH_REF)"

.PHONY: download verify vet tidy tidy-check fmt fmt-check vulncheck lint lint-fix align align-fix

download:
	$(GO) mod download

verify:
	$(GO) mod verify

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

tidy-check:
	@$(GO) mod tidy
	@git diff --stat --exit-code -- go.mod go.sum || ( \
		echo "go mod tidy: repository is not tidy"; \
		exit 1; \
	)

fmt:
	$(GO) fmt ./...

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		echo "gofmt: files need formatting"; \
		exit 1; \
	fi

vulncheck:
	$(VULNCHECK) ./...

lint:
	$(LINTER) run ./...

lint-fix:
	$(LINTER) run --fix ./...

align:
	$(ALIGNER) ./...

align-fix:
	-$(ALIGNER) -apply ./...
	$(ALIGNER) ./...

.PHONY: generate-docs generate-check

generate-docs:
	@tmp=$$(mktemp); trap 'rm -f "$$tmp"' EXIT; \
	env -u BERCON_ADDRESS -u BERCON_PASSWORD -u BERCON_PORT \
		-u BERCON_TIMEOUT -u BERCON_ATTEMPTS -u BERCON_KEEPALIVE \
		-u BERCON_PROFILE -u BERCON_CONFIG \
		-u BERCON_GEO_DB -u BERCON_GEO_DB_URL -u BERCON_GEO_DB_URL_AUTH \
		-u BERCON_REPEAT -u BERCON_SLEEP_CMD -u BERCON_SLEEP_LOOP \
		-u BERCON_BUFFER_SIZE -u BERCON_FORMAT \
		LANG=en $(GO) run ./cmd/$(BINARY) --no-env --config "$$tmp" docs md \
		--program-name $(BINARY) --style posix --template table CLI.md

generate-check: generate-docs
	@git diff --stat --exit-code -- CLI.md || ( \
		echo "CLI docs are out of date; run 'make generate-docs' and commit changes"; \
		exit 1; \
	)

.PHONY: winres

winres:
	$(WINRES) make \
	--in winres/manifest.json \
	--arch amd64,arm64 \
	--out $(WINRES_OUT) \
	--product-version "$(VERSION)" \
	--file-version "$(VERSION)"

.PHONY: tools tools-ci tools-build \
	tool-golangci-lint tool-betteralign tool-govulncheck tool-winres tool-benchstat

tools: tool-golangci-lint tool-betteralign tool-govulncheck tool-winres tool-benchstat
tools-ci: tool-golangci-lint tool-betteralign tool-govulncheck

tool-golangci-lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

tool-betteralign:
	$(GO) install github.com/dkorunic/betteralign/cmd/betteralign@latest

tool-govulncheck:
	$(GO) install golang.org/x/vuln/cmd/govulncheck@latest

tool-winres:
	$(GO) install github.com/tc-hib/go-winres@latest

tool-benchstat:
	$(GO) install golang.org/x/perf/cmd/benchstat@latest

.PHONY: release-notes

release-notes:
	@awk '\
	/^<!--/,/^-->/ { next } \
	/^## \[[0-9]+\.[0-9]+\.[0-9]+\]/ { if (found) exit; found=1; next } \
	found { \
		if (/^## \[/) { exit } \
		if (/^$$/) { flush(); print; next } \
		if (/^\* / || /^- /) { flush(); buf=$$0; next } \
		if (/^###/ || /^\[/) { flush(); print; next } \
		sub(/^[ \t]+/, ""); sub(/[ \t]+$$/, ""); \
		if (buf != "") { buf = buf " " $$0 } else { buf = $$0 } \
		next \
	} \
	function flush() { if (buf != "") { print buf; buf = "" } } \
	END { flush() } \
	' CHANGELOG.md

.PHONY: mkdocs-run markdown-lint markdown-fix

mkdocs-run:
	docker run --rm -it -p $(MKDOCS_ADDR):8000 \
		-v "$(CURDIR):/docs/docs" \
		-v "$(CURDIR)/mkdocs.yml:/docs/mkdocs.yml:ro" \
		$(MKDOCS_IMAGE) serve --dev-addr=0.0.0.0:8000

define run-rumdl
	@if command -v $(RUMDL) &>/dev/null; then \
		$(RUMDL) $(1); \
	else \
		echo "WARN: $(RUMDL) not found; skipping markdown lint."; \
		echo 'WARN: Install it https://github.com/rvben/rumdl'; \
	fi
endef

markdown-lint:
	$(call run-rumdl,check)

markdown-fix:
	$(call run-rumdl,check --fix)
