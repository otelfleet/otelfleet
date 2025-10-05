package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/ecdh"
	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/otelfleet/otelfleet/pkg/keyring"
	_ "github.com/otelfleet/otelfleet/pkg/logger"
	"github.com/otelfleet/otelfleet/pkg/supervisor"
	"google.golang.org/protobuf/encoding/protojson"
)

type authRequest struct {
	Id ident.ID `json:"id"`
}

func main() {
	logger := slog.Default()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	bootstrapToken := os.Getenv("BOOTSTRAP_TOKEN")
	agentName := os.Getenv("AGENT_NAME")
	if _, err := bootstrapAgent(agentName, bootstrapToken); err != nil {
		panic(err)
	}

	supervisor := supervisor.NewSupervisor(
		slog.Default().With("component", "supervisor"),
		nil,
	)
	logger.Info("otelfleet agent starting...")
	if err := supervisor.Start(); err != nil {
		logger.With("err", err.Error()).Error("failed to start supervisor")
		os.Exit(1)
	}
	<-interrupt
	logger.Info("shutting down otelfleet agent...")
	if err := supervisor.Shutdown(); err != nil {
		logger.With("err", err.Error()).Error("failed to shutdown supervisor")
		os.Exit(1)
	}
}

func bootstrapAgent(agentName string, bootstrapToken string) (keyring.Keyring, error) {
	slog.Default().With("agent", agentName).Info("bootstraping agent")

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
	id, err := ident.IdFromMac(sha256.New(), agentName, map[string]string{})
	if err != nil {
		return nil, err
	}

	slog.Default().With("id", id.UniqueIdentifier().UUID).Info("generated agent identity")

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
	slog.Default().With("shared-secret", sharedSecret).Info("derived shared secret")

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
