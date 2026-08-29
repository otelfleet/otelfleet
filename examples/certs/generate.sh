#!/usr/bin/env bash

set -euo pipefail

cert_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$cert_dir"

go tool cfssl gencert -initca server-ca-csr.json \
  | go tool cfssljson -bare server-ca

go tool cfssl gencert \
  -ca server-ca.pem \
  -ca-key server-ca-key.pem \
  -config ca-config.json \
  -profile server \
  server-csr.json \
  | go tool cfssljson -bare server

go tool cfssl gencert -initca collectors-ca-csr.json \
  | go tool cfssljson -bare collectors-ca

go tool cfssl gencert \
  -ca collectors-ca.pem \
  -ca-key collectors-ca-key.pem \
  -config ca-config.json \
  -profile collector \
  collector-csr.json \
  | go tool cfssljson -bare collector
