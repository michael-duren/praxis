.PHONY: all help build generate test vet lint fmt cover tidy run web setup seed seed-clean install clean release gopls ci

BINARY := bin/praxis

all: generate build test ## generate templ code, build, and test (default)

help: ## list available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-10s %s\n", $$1, $$2}'

build: generate ## compile the praxis binary into bin/
	go build -o $(BINARY) ./cmd/praxis

# templ compiles .templ files into _templ.go files (checked in).
generate: ## regenerate templ output (internal/web/templates)
	templ generate ./internal/web/templates

test: generate ## run all tests
	go test ./...

vet: ## run go vet
	go vet ./...

lint: ## run golangci-lint (same version config as CI)
	golangci-lint run ./...

fmt: ## gofmt all source in place
	gofmt -w cmd internal

cover: generate ## run tests with coverage and print the total
	@mkdir -p bin
	go test -coverprofile=bin/cover.out ./...
	go tool cover -func=bin/cover.out | tail -1
	@echo "open a browser report with: go tool cover -html=bin/cover.out"

tidy: ## go mod tidy
	go mod tidy

# run/web/setup/seed use --debug-db so local development never touches the
# real database (~/.local/share/praxis/praxis.db). Note: `sync` (and the
# wizard's "sync now" step) still writes context files to your real home
# directory — that part is not sandboxed.
run: build ## open the TUI against the debug database
	./$(BINARY) --debug-db

web: build ## serve the web UI against the debug database
	./$(BINARY) --debug-db web

setup: build ## run the first-run setup wizard against the debug database
	./$(BINARY) --debug-db setup

seed: build ## seed the debug db + demo agent skills with rich dev data
	./scripts/seed-dev.sh

seed-clean: ## remove seeded dev data (debug db + demo-* agent skills)
	./scripts/seed-dev.sh --clean

install: generate ## go install the praxis binary
	go install ./cmd/praxis

clean: ## remove build artifacts and the debug database
	rm -rf bin
	rm -f "$${TMPDIR:-/tmp}/praxis-debug.db"

# ci mirrors what .github/workflows runs: build, lint, test, vet, gopls,
# and a gofmt cleanliness check (CI's gofmt job auto-fixes; here we fail).
ci: generate build lint test vet gopls ## run every check CI runs
	@test -z "$$(gofmt -l cmd internal)" || { \
		echo "gofmt needed on:"; gofmt -l cmd internal; exit 1; \
	}
	@echo "all CI checks passed"

# Cut a release: validate, run every check CI runs, then create and push
# an annotated v* tag. The tag push triggers .github/workflows/release.yml,
# which cross-compiles all platforms and publishes the GitHub Release.
#
#   make release VERSION=v0.2.0
release: ## tag and push a release (make release VERSION=vX.Y.Z)
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=vX.Y.Z"; exit 1; }
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$$' \
		|| { echo "VERSION must be semver, e.g. v0.2.0 or v0.2.0-rc.1"; exit 1; }
	@git rev-parse --verify --quiet "refs/tags/$(VERSION)" >/dev/null \
		&& { echo "tag $(VERSION) already exists"; exit 1; } || true
	@test "$$(git branch --show-current)" = "main" \
		|| { echo "releases are cut from main (currently on $$(git branch --show-current))"; exit 1; }
	@test -z "$$(git status --porcelain)" \
		|| { echo "working tree is dirty — commit or stash first"; git status --short; exit 1; }
	@git fetch --quiet origin main
	@test "$$(git rev-parse HEAD)" = "$$(git rev-parse origin/main)" \
		|| { echo "main is not in sync with origin/main — push or pull first"; exit 1; }
	$(MAKE) ci
	git tag -a "$(VERSION)" -m "praxis $(VERSION)"
	git push origin "$(VERSION)"
	@echo "Pushed $(VERSION). The Release workflow is now building all platforms:"
	@echo "  gh run watch --repo $$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo '<owner>/<repo>')"

gopls: ## run gopls diagnostics (hints and up fail the build)
	@command -v gopls >/dev/null 2>&1 || { \
		echo "gopls not found — install with: go install golang.org/x/tools/gopls@latest"; \
		exit 1; \
	}
	@out=$$(find . -name '*.go' \
		-not -path './.claude/*' \
		-not -path './bin/*' \
		| xargs gopls check -severity=hint 2>&1); \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		echo; \
		echo "gopls reported findings — fix them before opening a PR."; \
		exit 1; \
	fi
