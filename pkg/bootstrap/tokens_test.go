package bootstrap_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestSigningTokens(t *testing.T) {
	assert := assert.New(t)
	token := bootstrap.NewToken()

	pub, priv, err := ed25519.GenerateKey(nil)

	assert.NoError(err)
	sig, err := token.SignDetached(priv)
	assert.NoError(err)
	segments := bytes.Split(sig, []byte{'.'})
	assert.Len(segments, 3)
	assert.Empty(segments[1])
	header, err := base64.RawURLEncoding.DecodeString(string(segments[0]))
	assert.NoError(err)
	assert.Equal(`{"alg":"EdDSA"}`, string(header))
	complete, err := token.VerifyDetached(sig, pub)
	assert.NoError(err)
	segments = bytes.Split(complete, []byte{'.'})
	assert.Len(segments, 3)
	expectedData, _ := json.Marshal(token)
	encoded, err := base64.RawURLEncoding.DecodeString(string(segments[1]))
	assert.NoError(err)
	assert.Equal(expectedData, encoded)
}
