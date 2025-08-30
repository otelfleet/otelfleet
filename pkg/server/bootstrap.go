package server

import (
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/otelfleet/otelfleet/pkg/storage"
)

type BootstrapServer struct {
	tokenStore storage.KeyValue[*v1alpha1.BootstrapToken]
	agentStore storage.KeyValue[*protobufs.AgentToServer]

	privateKey ed25519.PrivateKey
}

func NewBootstrapServer(
	tokenStore storage.KeyValue[*v1alpha1.BootstrapToken],
	agentStore storage.KeyValue[*protobufs.AgentToServer],
) *BootstrapServer {
	return &BootstrapServer{
		tokenStore: tokenStore,
		agentStore: agentStore,
		privateKey: []byte{},
	}
}

func (b *BootstrapServer) routePrefix() string {
	return "api/bootstrap/v1alpha1/"
}

func (b *BootstrapServer) ConfigureHttp(r *gin.Engine) {
	r.POST(path.Join(b.routePrefix(), "tokens/create"), b.CreateToken)
	r.GET(path.Join(b.routePrefix(), "tokens/list"), b.ListTokens)
	r.DELETE(path.Join(b.routePrefix(), "tokens/delete"), b.RevokeToken)

}

func (b *BootstrapServer) CreateToken(c *gin.Context) {
	token := bootstrap.NewToken()

	bT := token.ToBootstrapToken()
	if err := b.tokenStore.Put(c.Request.Context(), bT.GetID(), bT); err != nil {
		c.JSON(http.StatusInternalServerError, "")
		return
	}

	c.JSON(200, gin.H{
		"id":    token.HexID(),
		"token": token.EncodeToHex(),
	})
}

func (b *BootstrapServer) ListTokens(c *gin.Context) {
	tokens, err := b.tokenStore.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, "")
		return
	}
	c.JSON(http.StatusOK, tokens)
}

type deleteRequest struct {
	ID string `json:"id"`
}

func (b *BootstrapServer) RevokeToken(c *gin.Context) {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, "")
		return
	}
	var d deleteRequest
	if err := json.Unmarshal(data, &d); err != nil {
		c.JSON(http.StatusBadRequest, "")
		return
	}

	if d.ID == "" {
		c.JSON(http.StatusBadRequest, "invalid token ID")
		return
	}

	slog.Default().With("key", d.ID).Info("deleting key")
	if err := b.tokenStore.Delete(c.Request.Context(), d.ID); err != nil {
		c.JSON(http.StatusInternalServerError, "")
		return
	}

	c.JSON(http.StatusOK, "")
}
