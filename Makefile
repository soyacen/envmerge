BINARY   := envmerge
PKG      := ./cmd/envmerge
VERSION  := $(shell cat VERSION 2>/dev/null | tr -d '\n')
LDFLAGS  := -s -w -X github.com/soyacen/envmerge/internal/cli.Version=$(VERSION)

.PHONY: all build install test race cover vet fmt lint check clean docker examples

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

race:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint: vet
	@test -z "$(shell gofmt -l .)" || (echo "gofmt needed on the files above" && exit 1)

check: lint test race

clean:
	rm -rf bin coverage.out

docker:
	docker build -t $(BINARY):$(VERSION) .

# Compile the example MuonFlow service templates.
examples: build
	./bin/$(BINARY) generate --dir examples/deploy/env --pattern '*.env.example' --force

.PHONY: examples
