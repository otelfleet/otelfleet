package bootstrap

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	cryptoecdh "crypto/ecdh"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/open-telemetry/opamp-go/protobufs"
	v1alpha1agents "github.com/otelfleet/otelfleet/pkg/api/agents/v1alpha1"
	v1alpha1bootstrap "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapconnect "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/ecdh"
	otelfleetsvc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// for secure vs insecure implementations
type Bootstrapper interface {
	VerifyToken(context.Context, http.Header) (token string, err error)
	DeriveSharedSecret(*v1alpha1bootstrap.BootstrapAuthRequest) (sharedSecret []byte, keypair ecdh.EphemeralKeyPair, err error)
}

type BootstrapServer struct {
	tokenStore      storage.KeyValue[*v1alpha1bootstrap.BootstrapToken]
	agentStore      storage.KeyValue[*v1alpha1agents.AgentDescription]
	opampAgentStore storage.KeyValue[*protobufs.AgentToServer]

	privateKey crypto.Signer
	logger     *slog.Logger
	services.Service

	bootstrapper         Bootstrapper
	configStore          storage.KeyValue[*configv1alpha1.Config]
	bootstrapConfigStore storage.KeyValue[*configv1alpha1.Config]
	assignedConfigStore  storage.KeyValue[*configv1alpha1.Config]
}

var _ otelfleetsvc.HTTPExtension = (*BootstrapServer)(nil)

var _ bootstrapconnect.TokenServiceHandler = (*BootstrapServer)(nil)
var _ bootstrapconnect.BootstrapServiceHandler = (*BootstrapServer)(nil)

func NewBootstrapServer(
	logger *slog.Logger,
	privateKey crypto.Signer,
	tokenStore storage.KeyValue[*v1alpha1bootstrap.BootstrapToken],
	opampAgentStore storage.KeyValue[*protobufs.AgentToServer],
	agentStore storage.KeyValue[*v1alpha1agents.AgentDescription],
	configStore storage.KeyValue[*configv1alpha1.Config],
	bootstrapConfigStore storage.KeyValue[*configv1alpha1.Config],
	assignedConfigStore storage.KeyValue[*configv1alpha1.Config],
) *BootstrapServer {
	b := &BootstrapServer{
		tokenStore:           tokenStore,
		opampAgentStore:      opampAgentStore,
		privateKey:           privateKey,
		logger:               logger,
		bootstrapper:         NewBootstrapper(logger, tokenStore, privateKey),
		agentStore:           agentStore,
		configStore:          configStore,
		bootstrapConfigStore: bootstrapConfigStore,
		assignedConfigStore:  assignedConfigStore,
	}

	b.Service = services.NewBasicService(nil, b.running, nil)
	return b
}

func (b *BootstrapServer) running(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (b *BootstrapServer) ConfigureHTTP(mux *mux.Router) {
	b.logger.Info("configuring routes")
	bootstrapconnect.RegisterTokenServiceHandler(mux, b)
	bootstrapconnect.RegisterBootstrapServiceHandler(mux, b)
}

func (b *BootstrapServer) CreateToken(ctx context.Context, connectReq *connect.Request[v1alpha1bootstrap.CreateTokenRequest]) (*connect.Response[v1alpha1bootstrap.BootstrapToken], error) {
	req := connectReq.Msg
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token := bootstrap.NewToken()

	bT := token.ToBootstrapToken()
	bT.TTL = req.TTL
	bT.Expiry = timestamppb.New(time.Now().Add(time.Minute * 5))
	bT.ConfigReference = req.ConfigReference
	bT.Labels = req.Labels

	if ref := req.GetConfigReference(); ref != "" {
		config, err := b.configStore.Get(ctx, ref)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get associated config for ref %s : %s", ref, err))
		}
		b.logger.With("token", bT.GetID()).Debug("persisting bootstrap config")
		if err := b.bootstrapConfigStore.Put(ctx, bT.GetID(), config); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to persist bootstrap config : %s", err))
		}
	}

	if err := b.tokenStore.Put(ctx, bT.GetID(), bT); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return connect.NewResponse(bT), nil
}

func (b *BootstrapServer) GetBootstrapConfig(ctx context.Context, connectReq *connect.Request[v1alpha1bootstrap.GetConfigRequest]) (*connect.Response[v1alpha1bootstrap.GetConfigResponse], error) {
	req := connectReq.Msg
	b.logger.With("token", req.TokenID).Debug("fetching bootstrap config")
	config, err := b.bootstrapConfigStore.Get(ctx, req.GetTokenID())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(
		&v1alpha1bootstrap.GetConfigResponse{
			Config: config,
		},
	), nil

}

func (b *BootstrapServer) ListTokens(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1bootstrap.ListTokenReponse], error) {
	if b.tokenStore == nil {
		panic("token store is nil")
	}
	tokens, err := b.tokenStore.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	now := time.Now()
	for _, token := range tokens {
		b.logger.With("expire", token.Expiry.AsTime(), "now", now).Debug("token expiry check")
		if token.Expiry.AsTime().Before(now) {
			go b.gc(token.ID)
		}
	}

	resp := &v1alpha1bootstrap.ListTokenReponse{
		Tokens: tokens,
	}
	return connect.NewResponse(resp), nil
}

func (b *BootstrapServer) DeleteToken(ctx context.Context, connectReq *connect.Request[v1alpha1bootstrap.DeleteTokenRequest]) (*connect.Response[emptypb.Empty], error) {
	req := connectReq.Msg
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	b.logger.With("key", req.ID).Debug("deleting key")
	if err := b.tokenStore.Delete(ctx, req.ID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (b *BootstrapServer) Signatures(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1bootstrap.SignatureResponse], error) {
	signatures := map[string][]byte{}
	tokenList, err := b.tokenStore.List(ctx)
	if err != nil {
		b.logger.With("err", err).Error("failed to list tokens")
		return nil, grpcutil.ErrorInternal(err)
	}
	for _, tok := range tokenList {
		rawToken, err := bootstrap.FromBootstrapToken(tok)
		if err != nil {
			b.logger.With("err", err).Error("failed to convert bootstrap token")
			return nil, grpcutil.ErrorInternal(err)
		}
		sig, err := rawToken.SignDetached(b.privateKey)
		if err != nil {
			b.logger.With("err", err).Error("failed to sign token")
			return nil, grpcutil.ErrorInternal(err)
		}
		signatures[rawToken.HexID()] = sig
	}
	if len(signatures) == 0 {
		return nil, grpcutil.ErrorNotFound(err)
	}
	resp := &v1alpha1bootstrap.SignatureResponse{
		Signatures: signatures,
	}
	return connect.NewResponse(resp), err
}

func (b *BootstrapServer) Bootstrap(ctx context.Context, req *connect.Request[v1alpha1bootstrap.BootstrapAuthRequest]) (*connect.Response[v1alpha1bootstrap.BootstrapAuthResponse], error) {
	if req.Msg.GetClientId() == "" {
		return nil, grpcutil.ErrorInvalid(fmt.Errorf("empty agent id"))
	}

	if req.Msg.GetName() == "" {
		return nil, grpcutil.ErrorInvalid(fmt.Errorf("empty agent name"))
	}

	callInfo, ok := connect.CallInfoForHandlerContext(ctx)
	if !ok {
		return nil, grpcutil.ErrorInvalid(fmt.Errorf("can't access headers: no CallInfo for handler context"))
	}
	token, err := b.bootstrapper.VerifyToken(ctx, callInfo.RequestHeader())
	if err != nil {
		return nil, err
	}

	sharedSecret, ekp, err := b.bootstrapper.DeriveSharedSecret(req.Msg)
	if err != nil {
		return nil, grpcutil.ErrorInvalid(err)
	}

	if err := b.updateAgentDetails(ctx, req.Msg.GetClientId(), req.Msg.GetName(), token); err != nil {
		return nil, err
	}

	b.logger.With("shared-secret", sharedSecret).Info("got shared secret")
	return connect.NewResponse(
		&v1alpha1bootstrap.BootstrapAuthResponse{
			ServerPubKey: ekp.PublicKey.Bytes(),
		},
	), nil
}

func (b *BootstrapServer) updateAgentDetails(
	ctx context.Context,
	agentID string,
	name string,
	token string,
) error {
	l := b.logger.With("agentID", agentID).With("friendly-name", name)
	l.Info("bootstrap successful, persisting agent details")
	_, err := b.agentStore.Get(ctx, agentID)
	if grpcutil.IsErrorNotFound(err) {
		l.Info("persisting agent details")
		if err := b.agentStore.Put(ctx, agentID, &v1alpha1agents.AgentDescription{
			Id:           agentID,
			FriendlyName: name,
		}); err != nil {
			return err
		}
	} else if err != nil {
		return grpcutil.ErrorInternal(err)
	}
	incomingConfig, err := b.bootstrapConfigStore.Get(ctx, token)
	if err == nil {
		l.Info("agent has an assigned config")
		_, configErr := b.assignedConfigStore.Get(ctx, agentID)
		if grpcutil.IsErrorNotFound(configErr) {
			l.Info("no config has been assigned to an agent yet, associating bootstrap config with agent")
			if err := b.assignedConfigStore.Put(ctx, agentID, incomingConfig); err != nil {
				return err
			}
		} else if configErr != nil {
			return configErr
		}
	}
	// note: in the future there may be things we want to update here like capabilities / scope
	return nil
}

func (b *BootstrapServer) gc(key string) {
	b.logger.With("key", key).Debug("garbage collecting token")

	go func() {
		ctx, ca := context.WithTimeout(context.Background(), time.Minute)
		defer ca()
		if err := b.tokenStore.Delete(ctx, key); err != nil {
			b.logger.With("key", key, "err", err).Error("failed to delete token")
		}
	}()
}

type noopBootstrapper struct {
	logger *slog.Logger
}

func NewNoopBootstrapper(logger *slog.Logger) *noopBootstrapper {
	return &noopBootstrapper{
		logger: logger.With("bootstrapper", "no-op"),
	}
}

var _ Bootstrapper = (*noopBootstrapper)(nil)

func (n *noopBootstrapper) VerifyToken(_ context.Context, headers http.Header) (string, error) {
	token := strings.TrimSpace(headers.Get("Authorization"))
	n.logger.With("token", token).Debug("verified token")
	return token, nil
}

func (n *noopBootstrapper) DeriveSharedSecret(*v1alpha1bootstrap.BootstrapAuthRequest) (sharedSecret []byte, keyapir ecdh.EphemeralKeyPair, err error) {
	n.logger.Debug("derived shared secret")
	return []byte{}, ecdh.EphemeralKeyPair{
		PublicKey:  &cryptoecdh.PublicKey{},
		PrivateKey: &cryptoecdh.PrivateKey{},
	}, nil
}

type secureBootstrapper struct {
	logger     *slog.Logger
	tokenStore storage.KeyValue[*v1alpha1bootstrap.BootstrapToken]
	privateKey crypto.Signer
}

var _ Bootstrapper = (*secureBootstrapper)(nil)

func NewSecureBootstrapper(
	logger *slog.Logger,
	tokenStore storage.KeyValue[*v1alpha1bootstrap.BootstrapToken],
	privateKey crypto.Signer,
) *secureBootstrapper {
	return &secureBootstrapper{
		logger:     logger.With("bootstrapper", "secure"),
		tokenStore: tokenStore,
		privateKey: privateKey,
	}
}

func (s *secureBootstrapper) VerifyToken(ctx context.Context, headers http.Header) (string, error) {
	auth := strings.TrimSpace(headers.Get("Authorization"))
	if auth == "" {
		return "", fmt.Errorf("no request header set")
	}
	bearerToken := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
	payload, err := jws.Verify([]byte(bearerToken), jwa.RS256, s.privateKey.Public())
	if err != nil {
		return "", err
	}
	token := &bootstrap.Token{}
	if err := json.Unmarshal(payload, token); err != nil {
		return "", err
	}

	// check token exists, maybe handle this a little different based on the error
	id := token.ToBootstrapToken().GetID()
	_, err = s.tokenStore.Get(ctx, id)
	if grpcutil.IsErrorNotFound(err) {
		return "", err
	} else if err != nil {
		return "", grpcutil.ErrorInternal(err)
	}
	return id, nil
}

func (s *secureBootstrapper) DeriveSharedSecret(bootstrapReq *v1alpha1bootstrap.BootstrapAuthRequest) ([]byte, ecdh.EphemeralKeyPair, error) {
	kp := ecdh.EphemeralKeyPair{}
	ekp := ecdh.NewEphemeralKeyPair()
	clientPubKey, err := ecdh.ClientPubKey(bootstrapReq)
	if err != nil {
		return nil, kp, err
	}
	sharedSecret, err := ecdh.DeriveSharedSecret(ekp, clientPubKey)
	if err != nil {
		return nil, kp, err
	}
	return sharedSecret, ekp, nil
}
