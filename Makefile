.PHONY: build

build: build-ui build-go

build-ui:
# 	cd ui && npm run build
build-agent:
	go build -o ./bin/agent ./cmd/agent/main.go
build-go:
	go build -o ./bin/otelfleet ./cmd/server/main.go

build-dev:
	go build -tags insecure -o ./bin/otelfleet ./cmd/server/main.go
	go build -tags insecure -o ./bin/agent ./cmd/agent/

