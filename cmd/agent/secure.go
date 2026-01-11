//go:build !insecure

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/ecdh"
	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/otelfleet/otelfleet/pkg/keyring"
	_ "github.com/otelfleet/otelfleet/pkg/logutil"
	"google.golang.org/protobuf/encoding/protojson"
)

func NewBootstrapper(
	logger *slog.Logger,
	bClient bootstrapv1alpha1.BootstrapServiceClient,
	tClient bootstrapv1alpha1.TokenServiceClient,
) Bootstrapper {
	return &secureBootstrapper{
		logger:  logger,
		bClient: bClient,
		tClient: tClient,
	}
}

type secureBootstrapper struct {
	logger  *slog.Logger
	bClient bootstrapv1alpha1.BootstrapServiceClient
	tClient bootstrapv1alpha1.TokenServiceClient
}

func (sb *secureBootstrapper) VerifyToken(token string) error {
	// TODO: implement token verification
	return nil
}

func (sb *secureBootstrapper) Bootstrap(ctx context.Context, req *v1alpha1.BootstrapAuthRequest) (*tls.Config, error) {
	_, err := bootstrapAgent(sb.logger, req.Name, "")
	if err != nil {
		return nil, err
	}
	// TODO : use returned keyring to store and get *tls.Config depending on environment
	return nil, fmt.Errorf("not implemented yet")
}

func bootstrapAgent(logger *slog.Logger, agentName string, bootstrapToken string) (keyring.Keyring, error) {
	logger.With("agent", agentName).Info("bootstraping agent")

	tok, err := bootstrap.ParseHex(bootstrapToken)
	if err != nil {
		return nil, err
	}

	client := http.DefaultClient
	server := "https://localhost:8080"
	req, err := http.NewRequest(http.MethodGet, server+"/api/bootstrap/v1alpha1/signatures", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response code signatures : %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	signatures := &v1alpha1.SignatureResponse{}
	if err := protojson.Unmarshal(data, signatures); err != nil {
		panic(err)
	}
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no TLS")
	}
	serverLeafCert := resp.TLS.PeerCertificates[0]

	sig, ok := signatures.Signatures[tok.HexID()]
	if !ok {
		return nil, fmt.Errorf("signature not found")
	}

	completeJWS, err := tok.VerifyDetached(sig, serverLeafCert.PublicKey)
	if err != nil {
		panic(err)
	}
	// get keyring
	id, err := ident.IdFromMac(sha256.New(), agentName)
	if err != nil {
		return nil, err
	}

	logger.With("id", id.UniqueIdentifier().UUID).Info("generated agent identity")

	ekp := ecdh.NewEphemeralKeyPair()
	bootstrapReqBodyReq := v1alpha1.BootstrapRequest{
		ID:           id.UniqueIdentifier().UUID,
		Name:         agentName,
		ClientPubKey: ekp.PublicKey.Bytes(),
	}

	bodyReq, err := protojson.Marshal(&bootstrapReqBodyReq)
	if err != nil {
		return nil, err
	}

	bootstrapReq, err := http.NewRequest(http.MethodPost, server+"/api/bootstrap/v1alpha1/bootstrap", bytes.NewReader(bodyReq))
	if err != nil {
		return nil, err
	}
	bootstrapReq.Header.Add(
		"Authorization",
		"Bearer "+string(completeJWS),
	)

	bootstrapResp, err := client.Do(bootstrapReq)
	if err != nil {
		return nil, err
	}
	if bootstrapResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected bootstrap response : %d", resp.StatusCode)
	}

	bootstrapRespData, err := io.ReadAll(bootstrapResp.Body)
	if err != nil {
		return nil, err
	}

	br := &a{}
	if err := json.Unmarshal(bootstrapRespData, br); err != nil {
		return nil, err
	}

	serverPubKey, err := ecdh.ServerPubKey(br)
	if err != nil {
		return nil, err
	}
	sharedSecret, err := ecdh.DeriveSharedSecret(ekp, serverPubKey)
	if err != nil {
		return nil, err
	}
	logger.With("shared-secret", sharedSecret).Info("derived shared secret")

	keys := []any{
		keyring.NewSharedKeys(sharedSecret),
	}

	return keyring.New(keys...), nil
}

type a struct {
	ServerKeyPub []byte `json:"serverPubKey"`
}

func (aa *a) GetServerPubKey() []byte {
	return aa.ServerKeyPub
}
