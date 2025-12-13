package ident_test

import (
	"crypto/sha256"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/ident"
	"github.com/stretchr/testify/require"
)

func TestIdentFromMAC(t *testing.T) {
	provider, err := ident.IdFromMac(sha256.New(), "foo")
	require.NoError(t, err)

	id1 := provider.UniqueIdentifier().UUID
	require.NotEmpty(t, id1)
	id2 := provider.UniqueIdentifier().UUID
	require.NotEmpty(t, id2)
	require.Equal(t, id1, id2)
}
