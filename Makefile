BIN      := bin/hc
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/MorePeanuts/handcoding/internal/version.Version=$(VERSION)

# The tests run under uv run, which puts the uv project's environment first
# on PATH: .venv, on .python-version's Python (3.12, the protocol's minimum).
# make test PY=3.13 runs them on another Python, from .venv-3.13; make test
# PY=system on the python3 on PATH as it is. HC_TEST_REQUIRE_TOOLS makes a
# test that would skip for a missing or too old tool fail instead.
PY ?=
ifeq ($(PY),)
UV_RUN := uv run --locked
else
UV_RUN := UV_PROJECT_ENVIRONMENT=.venv-$(PY) uv run --locked --python $(PY)
endif
SHOW_PYTHON := echo "testing with $$(python3 -V) ($$(command -v python3))"

.PHONY: build install test lint fmt tidy clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/hc

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/hc

test:
ifeq ($(PY),system)
	@$(SHOW_PYTHON)
	HC_TEST_REQUIRE_TOOLS=1 go test -race ./...
else
	@command -v uv >/dev/null 2>&1 || { \
		echo "make test needs uv: https://docs.astral.sh/uv/getting-started/installation/" >&2; \
		echo "(make test PY=system runs the tests with the python3 on PATH instead)" >&2; \
		exit 1; \
	}
	@$(UV_RUN) sh -c '$(SHOW_PYTHON)'
	HC_TEST_REQUIRE_TOOLS=1 $(UV_RUN) go test -race ./...
endif

lint:
	go vet ./...
	golangci-lint run

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf bin dist
