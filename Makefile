.PHONY: test install install-goql build

VERSION ?= 0.5.0
LDFLAGS := -X github.com/yurimoinhos/ameego/internal/cliapp.Version=$(VERSION)

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/ameego ./cmd/ameego

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/ameego

install-goql:
	go install -ldflags "$(LDFLAGS)" ./cmd/goql
