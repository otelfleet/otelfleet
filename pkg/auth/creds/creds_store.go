package creds

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"sync"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/config"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
)

type CollectorCredential struct {
	ID             [16]byte
	SerialNumber   string
	CertificatePEM []byte
	PrivateKeyPEM  []byte

	NotBefore time.Time
	NotAfter  time.Time

	// TODO : status encode status like pending, active, revoked, expired.
}

func (c *CollectorCredential) TLSCertificate(caPEM []byte) *protobufs.TLSCertificate {
	return &protobufs.TLSCertificate{
		Cert:       bytes.Clone(c.CertificatePEM),
		PrivateKey: bytes.Clone(c.PrivateKeyPEM),
		CaCert:     bytes.Clone(caPEM),
	}
}

type Store interface {
	Get(ctx context.Context, deployID string) (CollectorCredential, error)
	Issue(ctx context.Context, deployID string) (CollectorCredential, error)
}

type credentialStoreInMemory struct {
	mu          sync.RWMutex
	credentials map[string]CollectorCredential
	ca          *x509.Certificate
	caKey       crypto.Signer
	validFor    time.Duration
}

func NewInMem(certConfig config.CollectorCertConfig) (Store, error) {
	if err := certConfig.Validate(); err != nil {
		return nil, fmt.Errorf("validate collector certificate config: %w", err)
	}
	ca, caKey, err := loadCollectorCA(certConfig)
	if err != nil {
		return nil, err
	}

	return &credentialStoreInMemory{
		credentials: make(map[string]CollectorCredential),
		ca:          ca,
		caKey:       caKey,
		validFor:    certConfig.ValidFor,
	}, nil
}

func (s *credentialStoreInMemory) Get(ctx context.Context, deployID string) (CollectorCredential, error) {
	if err := ctx.Err(); err != nil {
		return CollectorCredential{}, err
	}
	if deployID == "" {
		return CollectorCredential{}, grpcutil.ErrorInvalid(fmt.Errorf("deployment ID is required"))
	}

	s.mu.RLock()
	credential, ok := s.credentials[deployID]
	s.mu.RUnlock()
	if !ok {
		return CollectorCredential{}, grpcutil.ErrorNotFound(fmt.Errorf("credential for deployment %q not found", deployID))
	}

	return cloneCollectorCredential(credential), nil
}

func (s *credentialStoreInMemory) Issue(ctx context.Context, deployID string) (CollectorCredential, error) {
	if err := ctx.Err(); err != nil {
		return CollectorCredential{}, err
	}
	if deployID == "" {
		return CollectorCredential{}, grpcutil.ErrorInvalid(fmt.Errorf("deployment ID is required"))
	}
	// Keep issuance under the write lock so concurrent calls cannot issue two
	// independently valid credentials for the same deployment.
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.credentials[deployID]; ok {
		return CollectorCredential{}, grpcutil.Error(codes.AlreadyExists, fmt.Errorf("credential for deployment %q already exists", deployID))
	}
	if err := ctx.Err(); err != nil {
		return CollectorCredential{}, err
	}

	credential, err := issueCollectorCredential(deployID, s.ca, s.caKey, s.validFor)
	if err != nil {
		return CollectorCredential{}, fmt.Errorf("issue credential for deployment %q: %w", deployID, err)
	}

	s.credentials[deployID] = cloneCollectorCredential(credential)
	return cloneCollectorCredential(credential), nil
}

func loadCollectorCA(certConfig config.CollectorCertConfig) (*x509.Certificate, crypto.Signer, error) {
	keyPair, err := tls.LoadX509KeyPair(certConfig.CaCertFile, certConfig.CaKeyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("load collector CA key pair: %w", err)
	}
	if len(keyPair.Certificate) == 0 {
		return nil, nil, errors.New("collector CA certificate chain is empty")
	}

	ca, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("parse collector CA certificate: %w", err)
	}
	if !ca.IsCA || ca.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, nil, errors.New("collector CA certificate is not permitted to sign certificates")
	}
	caKey, ok := keyPair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, nil, errors.New("collector CA private key does not implement crypto.Signer")
	}

	return ca, caKey, nil
}

func issueCollectorCredential(
	deployID string,
	ca *x509.Certificate,
	caKey crypto.Signer,
	validFor time.Duration,
) (CollectorCredential, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return CollectorCredential{}, fmt.Errorf("generate collector private key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return CollectorCredential{}, fmt.Errorf("generate collector certificate serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		URIs:         []*url.URL{util.NewURIFromID(deployID)},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(validFor),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, ca, &privateKey.PublicKey, caKey)
	if err != nil {
		return CollectorCredential{}, fmt.Errorf("sign collector certificate: %w", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return CollectorCredential{}, fmt.Errorf("marshal collector private key: %w", err)
	}

	return CollectorCredential{
		SerialNumber:   serial.String(),
		CertificatePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		PrivateKeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}),
		NotBefore:      template.NotBefore,
		NotAfter:       template.NotAfter,
	}, nil
}

func cloneCollectorCredential(credential CollectorCredential) CollectorCredential {
	credential.CertificatePEM = bytes.Clone(credential.CertificatePEM)
	credential.PrivateKeyPEM = bytes.Clone(credential.PrivateKeyPEM)
	return credential
}
