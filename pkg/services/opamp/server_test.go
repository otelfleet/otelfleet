package opamp_test

import (
	"testing"

	"github.com/otelfleet/otelfleet/pkg/util/testutil"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	env := testutil.NewTestEnv(t)
	require.NotNil(t, env.OpampServer)
	require.NotNil(t, env.OpampWSServer)
}
