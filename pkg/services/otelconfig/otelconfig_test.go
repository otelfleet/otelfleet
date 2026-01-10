package otelconfig

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	var b bytes.Buffer
	require.NoError(t, yaml.Unmarshal([]byte(DefaultOtelConfig), b))
}
