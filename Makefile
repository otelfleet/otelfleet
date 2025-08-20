build:
	go build -o ./bin/otelfleet ./cmd/server/main.go
	go build -o ./bin/agent ./cmd/agent/main.go
