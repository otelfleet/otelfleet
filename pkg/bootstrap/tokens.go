package bootstrap

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
)

var ErrMalformedToken = errors.New("malformed token")

type Token struct {
	ID     []byte `json:"id"`
	Secret []byte `json:"secret"`
}

// TODO : attaching a uuidV7 as an ID might be a good idea here
func NewToken(source ...io.Reader) *Token {
	entropy := rand.Reader
	if len(source) > 0 {
		entropy = source[0]
	}
	buf := make([]byte, 32)
	if _, err := io.ReadFull(entropy, buf); err != nil {
		panic(err)
	}
	return &Token{
		ID:     buf[:6],
		Secret: buf[6:],
	}
}

func (t *Token) SignDetached(key any) ([]byte, error) {
	_, ed25519 := key.(ed25519.PrivateKey)
	_, rsa := key.(*rsa.PrivateKey)
	if !rsa && !ed25519 {
		return nil, errors.New("invalid key type, expected ed25519.PrivateKey or rsa.PrivateKey")
	}
	jsonData, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	sig, err := jws.Sign(jsonData, jwa.RS256, key)
	if err != nil {
		return nil, err
	}
	firstIndex := bytes.IndexByte(sig, '.')
	lastIndex := bytes.LastIndexByte(sig, '.')
	buf := new(bytes.Buffer)
	buf.Write(sig[:firstIndex+1])
	buf.Write(sig[lastIndex:])
	return buf.Bytes(), nil
}

func (t *Token) VerifyDetached(sig []byte, key any) ([]byte, error) {
	jsonData, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	firstIndex := bytes.IndexByte(sig, '.')
	lastIndex := bytes.LastIndexByte(sig, '.')
	if firstIndex == -1 || lastIndex == -1 {
		return nil, ErrMalformedToken
	}
	payload := base64.RawURLEncoding.EncodeToString(jsonData)
	buf := new(bytes.Buffer)
	buf.Write(sig[:firstIndex+1])
	buf.WriteString(payload)
	buf.Write(sig[lastIndex:])
	fullToken := buf.Bytes()
	cloned := make([]byte, len(fullToken))
	copy(cloned, fullToken)
	// _, err = jws.Verify(cloned, jwa.EdDSA, key)
	_, err = jws.Verify(buf.Bytes(), jwa.RS256, key)
	if err != nil {
		return nil, err
	}
	return fullToken, nil
}

func (t *Token) HexID() string {
	return hex.EncodeToString(t.ID)
}

func (t *Token) HexSecret() string {
	return hex.EncodeToString(t.Secret)
}

func (t *Token) EncodeToHex() string {
	return t.HexID() + "." + t.HexSecret()
}

func ParseHex(str string) (*Token, error) {
	parts := bytes.Split([]byte(str), []byte("."))
	if len(parts) != 2 ||
		len(parts[0]) != hex.EncodedLen(6) ||
		len(parts[1]) != hex.EncodedLen(26) {
		return nil, ErrMalformedToken
	}
	t := &Token{
		ID:     make([]byte, 6),
		Secret: make([]byte, 26),
	}
	if n, err := hex.Decode(t.ID, parts[0]); err != nil || n != 6 {
		return nil, ErrMalformedToken
	}
	if n, err := hex.Decode(t.Secret, parts[1]); err != nil || n != 26 {
		return nil, ErrMalformedToken
	}
	return t, nil
}

func FromBootstrapToken(t *v1alpha1.BootstrapToken) (*Token, error) {
	tokenID := t.GetID()
	tokenSecret := t.GetSecret()
	token := &Token{
		ID:     make([]byte, hex.DecodedLen(len(tokenID))),
		Secret: make([]byte, hex.DecodedLen(len(tokenSecret))),
	}
	decodedID, err := hex.DecodeString(tokenID)
	if err != nil {
		return nil, err
	}
	decodedSecret, err := hex.DecodeString(tokenSecret)
	if err != nil {
		return nil, err
	}
	copy(token.ID, decodedID)
	copy(token.Secret, decodedSecret)
	return token, nil
}

func (t *Token) ToBootstrapToken() *v1alpha1.BootstrapToken {
	return &v1alpha1.BootstrapToken{
		ID:     t.HexID(),
		Secret: t.HexSecret(),
	}
}
