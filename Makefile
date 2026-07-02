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
