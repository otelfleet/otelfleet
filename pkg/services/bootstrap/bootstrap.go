package bootstrap

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1/v1alpha1connect"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/ecdh"
	otelfleetsvc "github.com/otelfleet/otelfleet/pkg/services"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type BootstrapServer struct {
	tokenStore storage.KeyValue[*v1alpha1.BootstrapToken]
	agentStore storage.KeyValue[*protobufs.AgentToServer]

	privateKey crypto.Signer
	logger     *slog.Logger
	services.Service
}

var _ otelfleetsvc.HTTPExtension = (*BootstrapServer)(nil)

var _ v1alpha1connect.TokenServiceHandler = (*BootstrapServer)(nil)

func NewBootstrapServer(
	logger *slog.Logger,
	privateKey crypto.Signer,
	tokenStore storage.KeyValue[*v1alpha1.BootstrapToken],
	agentStore storage.KeyValue[*protobufs.AgentToServer],
) *BootstrapServer {
	b := &BootstrapServer{
		tokenStore: tokenStore,
		agentStore: agentStore,
		privateKey: privateKey,
		logger:     logger,
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
	v1alpha1connect.RegisterTokenServiceHandler(mux, b)
}

func (b *BootstrapServer) CreateToken(ctx context.Context, connectReq *connect.Request[v1alpha1.CreateTokenRequest]) (*connect.Response[v1alpha1.BootstrapToken], error) {
	req := connectReq.Msg
	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	token := bootstrap.NewToken()

	bT := token.ToBootstrapToken()
	bT.TTL = req.TTL
	bT.Expiry = timestamppb.New(time.Now().Add(time.Minute * 5))
	if err := b.tokenStore.Put(ctx, bT.GetID(), bT); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return connect.NewResponse(bT), nil
}

func (b *BootstrapServer) ListTokens(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[v1alpha1.ListTokenReponse], error) {
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

	resp := &v1alpha1.ListTokenReponse{
		Tokens: tokens,
	}
	return connect.NewResponse(resp), nil
}

func (b *BootstrapServer) DeleteToken(ctx context.Context, connectReq *connect.Request[v1alpha1.DeleteTokenRequest]) (*connect.Response[emptypb.Empty], error) {
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

// func (b *BootstrapServer) routePrefix() string {
// 	return "api/bootstrap/v1alpha1/"
// }

// func (b *BootstrapServer) ConfigureHTTP(r *gin.Engine) {
// 	r.GET(path.Join(b.routePrefix(), "signatures"), b.Signatures)
// 	r.POST(path.Join(b.routePrefix(), "bootstrap"), b.Bootstrap)
// 	r.POST(path.Join(b.routePrefix(), "tokens/create"), b.CreateTokens2)
// 	r.GET(path.Join(b.routePrefix(), "tokens/list"), b.ListTokens2)
// 	r.DELETE(path.Join(b.routePrefix(), "tokens/delete"), b.RevokeToken)
// }

func (b *BootstrapServer) verifyToken(c *gin.Context) error {
	headers := c.Request.Header
	auth := strings.TrimSpace(headers.Get("Authorization"))
	if auth == "" {
		b.logger.Error("no request header set")
		return fmt.Errorf("no request header set")
	}
	bearerToken := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
	payload, err := jws.Verify([]byte(bearerToken), jwa.RS256, b.privateKey.Public())
	if err != nil {
		return err
	}
	token := &bootstrap.Token{}
	if err := json.Unmarshal(payload, token); err != nil {
		return err
	}

	// check token exists, maybe handle this a little different based on the error
	_, err = b.tokenStore.Get(c.Request.Context(), token.ToBootstrapToken().GetID())
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return err
	}
	return nil
}

func (b *BootstrapServer) deriveSharedSecret(c *gin.Context) ([]byte, ecdh.EphemeralKeyPair, error) {
	kp := ecdh.EphemeralKeyPair{}
	inData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, kp, err
	}

	bootstrapReq := &v1alpha1.BootstrapRequest{}
	if err := protojson.Unmarshal(inData, bootstrapReq); err != nil {
		return nil, kp, err
	}
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

func (b *BootstrapServer) Bootstrap(c *gin.Context) {
	if err := b.verifyToken(c); err != nil {
		c.JSON(http.StatusUnauthorized, err.Error())
		return
	}

	sharedSecret, ekp, err := b.deriveSharedSecret(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	slog.With("shared-secret", sharedSecret).Info("got shared secret")
	c.JSON(http.StatusOK, map[string]any{
		"serverPubKey": ekp.PublicKey.Bytes(),
	})

}

func (b *BootstrapServer) Signatures(c *gin.Context) {
	// headers := c.Request.Header
	// check headers?
	signatures := map[string][]byte{}
	tokenList, err := b.tokenStore.List(c.Request.Context())
	if err != nil {
		b.logger.With("err", err).Error("failed to list tokens")
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	for _, tok := range tokenList {
		rawToken, err := bootstrap.FromBootstrapToken(tok)
		if err != nil {
			b.logger.With("err", err).Error("failed to convert bootstrap token")
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		sig, err := rawToken.SignDetached(b.privateKey)
		if err != nil {
			b.logger.With("err", err).Error("failed to sign token")
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		signatures[rawToken.HexID()] = sig
	}
	if len(signatures) == 0 {
		c.JSON(http.StatusGone, "no signatures")
		return
	}

	resp := &v1alpha1.SignatureResponse{
		Signatures: signatures,
	}

	data, err := protojson.Marshal(resp)
	if err != nil {
		b.logger.With("err", err).Error("failed to marshal")
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write(data)
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
