package creds_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/otelfleet/otelfleet/pkg/auth/creds"
	"github.com/otelfleet/otelfleet/pkg/config"
	"github.com/otelfleet/otelfleet/pkg/util"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func newCredentialStoreInMemoryForTest(t *testing.T) creds.Store {
	t.Helper()
	store, err := creds.NewInMem(testCollectorCertConfig(t))
	require.NoError(t, err)
	return store
}

func testCredentialStore(t *testing.T, newStore func(*testing.T) creds.Store) {
	t.Helper()

	t.Run("get", func(t *testing.T) {
		testCases := []struct {
			name     string
			deployID string
			prepare  func(*testing.T, creds.Store) creds.CollectorCredential
			wantCode codes.Code
		}{
			{
				name:     "existing credential",
				deployID: "collector-1",
				prepare: func(t *testing.T, store creds.Store) creds.CollectorCredential {
					credential, err := store.Issue(t.Context(), "collector-1")
					require.NoError(t, err)
					return credential
				},
			},
			{name: "missing credential", deployID: "missing", wantCode: codes.NotFound},
			{name: "empty deployment ID", wantCode: codes.InvalidArgument},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				store := newStore(t)
				var want creds.CollectorCredential
				if tc.prepare != nil {
					want = tc.prepare(t, store)
				}

				got, err := store.Get(t.Context(), tc.deployID)
				if tc.wantCode != codes.OK {
					require.Error(t, err)
					assert.True(t, grpcutil.IsError(tc.wantCode, err), "expected %s, got %v", tc.wantCode, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, want, got)
			})
		}
	})

	t.Run("issue", func(t *testing.T) {
		testCases := []struct {
			name     string
			deployID string
			prepare  func(*testing.T, creds.Store)
			wantCode codes.Code
		}{
			{name: "new credential", deployID: "collector-1"},
			{
				name:     "duplicate credential",
				deployID: "collector-1",
				prepare: func(t *testing.T, store creds.Store) {
					_, err := store.Issue(t.Context(), "collector-1")
					require.NoError(t, err)
				},
				wantCode: codes.AlreadyExists,
			},
			{name: "empty deployment ID", wantCode: codes.InvalidArgument},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				store := newStore(t)
				if tc.prepare != nil {
					tc.prepare(t, store)
				}

				credential, err := store.Issue(t.Context(), tc.deployID)
				if tc.wantCode != codes.OK {
					require.Error(t, err)
					assert.True(t, grpcutil.IsError(tc.wantCode, err), "expected %s, got %v", tc.wantCode, err)
					return
				}
				require.NoError(t, err)
				assertCollectorCredential(t, credential, tc.deployID)
			})
		}
	})

	t.Run("returned values do not alias storage", func(t *testing.T) {
		store := newStore(t)
		issued, err := store.Issue(t.Context(), "collector-1")
		require.NoError(t, err)
		wantCertificate := append([]byte(nil), issued.CertificatePEM...)
		wantPrivateKey := append([]byte(nil), issued.PrivateKeyPEM...)
		issued.CertificatePEM[0], issued.PrivateKeyPEM[0] = 'X', 'X'

		stored, err := store.Get(t.Context(), "collector-1")
		require.NoError(t, err)
		assert.Equal(t, wantCertificate, stored.CertificatePEM)
		assert.Equal(t, wantPrivateKey, stored.PrivateKeyPEM)
	})

	t.Run("concurrent issue creates one credential", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			store := newStore(t)
			const callers = 8
			var wg sync.WaitGroup
			errs := make(chan error, callers)
			for range callers {
				wg.Go(func() {
					_, err := store.Issue(t.Context(), "collector-1")
					errs <- err
				})
			}
			wg.Wait()
			close(errs)

			var issued, duplicates int
			for err := range errs {
				switch {
				case err == nil:
					issued++
				case grpcutil.IsError(codes.AlreadyExists, err):
					duplicates++
				default:
					t.Fatalf("unexpected error: %v", err)
				}
			}
			assert.Equal(t, 1, issued)
			assert.Equal(t, callers-1, duplicates)
		})
	})

	t.Run("canceled context prevents operations", func(t *testing.T) {
		testCases := []struct {
			name string
			call func(context.Context, creds.Store) error
		}{
			{name: "get", call: func(ctx context.Context, store creds.Store) error {
				_, err := store.Get(ctx, "collector-1")
				return err
			}},
			{name: "issue", call: func(ctx context.Context, store creds.Store) error {
				_, err := store.Issue(ctx, "collector-1")
				return err
			}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				store := newStore(t)
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				assert.ErrorIs(t, tc.call(ctx, store), context.Canceled)
			})
		}
	})
}

func TestCredentialStoreInMemory(t *testing.T) {
	testCredentialStore(t, newCredentialStoreInMemoryForTest)
}

func assertCollectorCredential(t *testing.T, credential creds.CollectorCredential, deployID string) {
	t.Helper()
	certificateBlock, _ := pem.Decode(credential.CertificatePEM)
	require.NotNil(t, certificateBlock)
	certificate, err := x509.ParseCertificate(certificateBlock.Bytes)
	require.NoError(t, err)
	require.Len(t, certificate.URIs, 1)
	assert.Equal(t, util.NewURIFromID(deployID).String(), certificate.URIs[0].String())
	assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, certificate.ExtKeyUsage)
	assert.Equal(t, certificate.SerialNumber.String(), credential.SerialNumber)

	keyBlock, _ := pem.Decode(credential.PrivateKeyPEM)
	require.NotNil(t, keyBlock)
	_, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	require.NoError(t, err)
}

func testCollectorCertConfig(t *testing.T) config.CollectorCertConfig {
	t.Helper()
	dir := t.TempDir()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test collector CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	certFile := filepath.Join(dir, "ca.pem")
	keyFile := filepath.Join(dir, "ca-key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}), 0o600))

	return config.CollectorCertConfig{
		CaCertFile:  certFile,
		CaKeyFile:   keyFile,
		ValidFor:    24 * time.Hour,
		RenewBefore: time.Hour,
	}
}
