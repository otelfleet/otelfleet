package authorization

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
	v1alpha1bootstrap "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	bootstrapconnect "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/ecdh"
	otelfleetsvc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/storage/types"
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
	tokenStore types.KeyValue[*v1alpha1bootstrap.BootstrapToken]

	privateKey crypto.Signer
	logger     *slog.Logger
	services.Service

	bootstrapper Bootstrapper
}

var _ otelfleetsvc.HTTPExtension = (*BootstrapServer)(nil)

var _ bootstrapconnect.TokenServiceHandler = (*BootstrapServer)(nil)
var _ bootstrapconnect.BootstrapServiceHandler = (*BootstrapServer)(nil)

func NewBootstrapServer(
	logger *slog.Logger,
	privateKey crypto.Signer,
	tokenStore types.KeyValue[*v1alpha1bootstrap.BootstrapToken],
) *BootstrapServer {
	b := &BootstrapServer{
		tokenStore:   tokenStore,
		privateKey:   privateKey,
		logger:       logger,
		bootstrapper: NewBootstrapper(logger, tokenStore, privateKey),
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
	bT.Labels = req.Labels
	// logger := b.logger.With("token", bT.GetID())

	if err := b.tokenStore.Put(ctx, bT.GetID(), bT); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return connect.NewResponse(bT), nil
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
	_, err := b.bootstrapper.VerifyToken(ctx, callInfo.RequestHeader())
	if err != nil {
		return nil, err
	}

	sharedSecret, ekp, err := b.bootstrapper.DeriveSharedSecret(req.Msg)
	if err != nil {
		return nil, grpcutil.ErrorInvalid(err)
	}

	b.logger.With("shared-secret", sharedSecret).Info("got shared secret")
	return connect.NewResponse(
		&v1alpha1bootstrap.BootstrapAuthResponse{
			ServerPubKey: ekp.PublicKey.Bytes(),
		},
	), nil
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
	tokenStore types.KeyValue[*v1alpha1bootstrap.BootstrapToken]
	privateKey crypto.Signer
}

var _ Bootstrapper = (*secureBootstrapper)(nil)

func NewSecureBootstrapper(
	logger *slog.Logger,
	tokenStore types.KeyValue[*v1alpha1bootstrap.BootstrapToken],
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
