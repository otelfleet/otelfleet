package util

import (
	"testing"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
)

func TestHashAgentConfigMap(t *testing.T) {
	tests := []struct {
		name      string
		configMap *protobufs.AgentConfigMap
		wantEmpty bool
	}{
		{
			name:      "nil config map returns nil",
			configMap: nil,
			wantEmpty: true,
		},
		{
			name: "empty config map returns nil",
			configMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{},
			},
			wantEmpty: true,
		},
		{
			name: "single file config returns hash",
			configMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"config.yaml": {
						Body:        []byte("receivers:\n  otlp:"),
						ContentType: "text/yaml",
					},
				},
			},
			wantEmpty: false,
		},
		{
			name: "multiple files config returns hash",
			configMap: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"config.yaml": {
						Body:        []byte("receivers:\n  otlp:"),
						ContentType: "text/yaml",
					},
					"processors.yaml": {
						Body:        []byte("processors:\n  batch:"),
						ContentType: "text/yaml",
					},
				},
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashAgentConfigMap(tt.configMap)
			if tt.wantEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				assert.Len(t, result, 32) // SHA256 produces 32 bytes
			}
		})
	}
}

func TestHashAgentConfigMap_Stability(t *testing.T) {
	configMap := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body:        []byte("receivers:\n  otlp:"),
				ContentType: "text/yaml",
			},
		},
	}

	// Same config should always produce the same hash
	hash1 := HashAgentConfigMap(configMap)
	hash2 := HashAgentConfigMap(configMap)
	assert.Equal(t, hash1, hash2, "same config should produce same hash")
}

func TestHashAgentConfigMap_DifferentContentTypeSameHash(t *testing.T) {
	configMap1 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body:        []byte("receivers:\n  otlp:"),
				ContentType: "text/yaml",
			},
		},
	}

	configMap2 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body:        []byte("receivers:\n  otlp:"),
				ContentType: "application/yaml", // Different content type
			},
		},
	}

	hash1 := HashAgentConfigMap(configMap1)
	hash2 := HashAgentConfigMap(configMap2)
	assert.Equal(t, hash1, hash2, "content type should not affect hash")
}

func TestHashAgentConfigMap_DifferentBodyDifferentHash(t *testing.T) {
	configMap1 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body:        []byte("receivers:\n  otlp:"),
				ContentType: "text/yaml",
			},
		},
	}

	configMap2 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body:        []byte("receivers:\n  jaeger:"),
				ContentType: "text/yaml",
			},
		},
	}

	hash1 := HashAgentConfigMap(configMap1)
	hash2 := HashAgentConfigMap(configMap2)
	assert.NotEqual(t, hash1, hash2, "different body should produce different hash")
}

func TestHashAgentConfigMap_DifferentFilenameDifferentHash(t *testing.T) {
	configMap1 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				Body: []byte("receivers:\n  otlp:"),
			},
		},
	}

	configMap2 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"other.yaml": {
				Body: []byte("receivers:\n  otlp:"),
			},
		},
	}

	hash1 := HashAgentConfigMap(configMap1)
	hash2 := HashAgentConfigMap(configMap2)
	assert.NotEqual(t, hash1, hash2, "different filename should produce different hash")
}

func TestHashAgentConfigMap_OrderIndependent(t *testing.T) {
	// Create two config maps with files added in different order
	// Go maps don't guarantee order, but we create them separately
	// to ensure they're independent
	configMap1 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"a.yaml": {Body: []byte("a content")},
			"b.yaml": {Body: []byte("b content")},
			"c.yaml": {Body: []byte("c content")},
		},
	}

	configMap2 := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"c.yaml": {Body: []byte("c content")},
			"a.yaml": {Body: []byte("a content")},
			"b.yaml": {Body: []byte("b content")},
		},
	}

	hash1 := HashAgentConfigMap(configMap1)
	hash2 := HashAgentConfigMap(configMap2)
	assert.Equal(t, hash1, hash2, "hash should be independent of map insertion order")
}

func TestHashAgentConfigMap_NilFileSkipped(t *testing.T) {
	configMap := &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {Body: []byte("content")},
			"nil.yaml":    nil,
		},
	}

	// Should not panic and should return a valid hash
	hash := HashAgentConfigMap(configMap)
	assert.NotNil(t, hash)
	assert.Len(t, hash, 32)
}
