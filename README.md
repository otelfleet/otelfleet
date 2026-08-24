# otelfleet

Otelfleet is a self-hostable, OpenTelemetry collector management service.

Otelfleet is built on open standards: 
- OpenTelemetry collector
- OpAMP 

## Testing

With local TLS:

```
mkcert -cert-file localhost.otelfleet.io.pem -key-file localhost.otelfleet.io-key.pem   localhost.otelfleet.io host.docker.internal localhost 127.0.0.1 ::1
```

Need to explicitly set:
```yaml
services: 
  - all
certs:
  cert_file: ./localhost.otelfleet.io.pem
  key_file:  ./localhost.otelfleet.io-key.pem
otlp:
  advertise_addr : localhost.otelfleet.io:16587
```