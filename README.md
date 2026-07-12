# otelfleet

Otelfleet is a self-hostable, OpenTelemetry collector management service.

Otelfleet is built on open standards: 
- OpenTelemetry collector
- OpAMP 

## Testing

Lightweight Testing:
 
 ```
 docker run \
  -v ./examples/supervisor.yaml:/etc/otel/supervisor.yaml \
  -v ./examples/otel/config.yaml:/etc/otel/config.yaml \
  ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-opampsupervisor:latest
 ```