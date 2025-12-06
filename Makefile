.PHONY: build

build: build-ui build-go

build-ui:
# 	cd ui && npm run build
build-agent:
	go build -o ./bin/agent ./cmd/agent/main.go
build-go:
	go build -o ./bin/otelfleet ./cmd/server/main.go
