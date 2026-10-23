BIN      := bin/hc
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/MorePeanuts/handcoding/internal/cli.Version=$(VERSION)

.PHONY: build install test lint fmt tidy clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/hc

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/hc

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin dist
