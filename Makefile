.PHONY: all build generate test vet run web install clean release

BINARY := bin/praxis

all: generate build test

build: generate
	go build -o $(BINARY) ./cmd/praxis

# templ compiles .templ files into _templ.go files (checked in).
generate:
	templ generate ./internal/web/templates

test: generate
	go test ./...

vet:
	go vet ./...

# run/web use --debug-db so local development never touches the real
# database (~/.local/share/praxis/praxis.db). Seed it the same way, e.g.:
#   ./bin/praxis --debug-db skill add go beginner language
run: build
	./$(BINARY) --debug-db

web: build
	./$(BINARY) --debug-db web

install: generate
	go install ./cmd/praxis

clean:
	rm -rf bin
	rm -f "$${TMPDIR:-/tmp}/praxis-debug.db"

# Cut a release: validate, run every check CI runs, then create and push
# an annotated v* tag. The tag push triggers .github/workflows/release.yml,
# which cross-compiles all platforms and publishes the GitHub Release.
#
#   make release VERSION=v0.2.0
release:
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
	$(MAKE) all vet gopls
	git tag -a "$(VERSION)" -m "praxis $(VERSION)"
	git push origin "$(VERSION)"
	@echo "Pushed $(VERSION). The Release workflow is now building all platforms:"
	@echo "  gh run watch --repo $$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo '<owner>/<repo>')"

gopls:
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
