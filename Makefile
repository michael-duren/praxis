.PHONY: all build generate test vet run web install clean

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

run: build
	./$(BINARY)

web: build
	./$(BINARY) web

install: generate
	go install ./cmd/praxis

clean:
	rm -rf bin

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
