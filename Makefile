.PHONY: build

VERSION ?= $(shell git describe --tags --always --dirty)
GITCOMMIT := $(shell git rev-parse --short HEAD)

GOLDFLAGS = -X github.com/otelfleet/otelfleet/pkg/version.Version=${VERSION} \
	-X github.com/otelfleet/otelfleet/pkg/version.Commit=${GITCOMMIT}


build: build-ui build-go

build-ui:
# 	cd ui && npm run build
build-agent:
	CGO_ENABLED=0 go build --ldflags="$(GOLDFLAGS)" -o ./bin/agent ./cmd/agent/main.go
build-go:
	CGO_ENABLED=0 go build --ldflags="$(GOLDFLAGS)" -o ./bin/otelfleet ./cmd/server/main.go

build-dev:
	go build -tags insecure -o ./bin/otelfleet ./cmd/server/main.go
	go build -tags insecure -o ./bin/agent ./cmd/agent/

clean:
	rm -rf ./otelfleet.kv/