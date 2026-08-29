# Example certificates

These certificates and private keys are for local testing only. Generate them with the
CFSSL tools pinned in this repository's `go.mod`:

```console
./examples/certs/generate.sh
```

The server certificate is valid for `localhost`, `127.0.0.1`, and `::1`. The collector
certificate is a client-only certificate issued by a separate collector CA and contains
an example OpAMP instance identity as a URI SAN.

Configure OtelFleet with:

```yaml
certs:
  server:
    cert_file: examples/certs/server.pem
    key_file: examples/certs/server-key.pem
  collectors:
    ca_cert_file: examples/certs/collectors-ca.pem
    ca_key_file: examples/certs/collectors-ca-key.pem
    valid_for: 168h
    renew_before: 24h
```

The generated `collector.pem` and `collector-key.pem` provide a ready-made credential
for testing client-certificate verification. Production collector credentials should be
issued per OpAMP instance instead.
