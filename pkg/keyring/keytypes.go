package keyring

import (
	"crypto/x509"
	"slices"
)

// Key types are used indirectly via an interface, as most key values would
// benefit from extra accessor logic (e.g. copying raw byte arrays).

type SharedKeys struct {
	ClientKey []byte `json:"clientKey"`
	ServerKey []byte `json:"serverKey"`
}

type CACertsKey struct {
	// DER-encoded certificates
	CACerts [][]byte `json:"caCerts"`
}

func NewCACertsKey(certs []*x509.Certificate) *CACertsKey {
	key := &CACertsKey{
		CACerts: make([][]byte, len(certs)),
	}
	for i, cert := range certs {
		key.CACerts[i] = slices.Clone(cert.Raw)
	}
	return key
}

func NewSharedKeys(secret []byte) *SharedKeys {
	if len(secret) != 64 {
		panic("shared secret must be 64 bytes")
	}
	return &SharedKeys{
		ClientKey: secret[:32],
		ServerKey: secret[32:],
	}
}
