package util

import (
	"crypto/sha256"
	"slices"

	"github.com/open-telemetry/opamp-go/protobufs"
	configv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/config/v1alpha1"
)

// ConfigToAgentConfigMap converts a Config proto to an AgentConfigMap.
// This ensures consistent structure when creating configs for agents,
// using "config.yaml" as the standard filename.
func ProtoConfigToAgentConfigMap(config *configv1alpha1.Config) *protobufs.AgentConfigMap {
	return &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigFile{
			"config.yaml": {
				ContentType: "text/yaml",
				Body:        config.GetConfig(),
			},
		},
	}
}

// HashAgentConfigMap computes a stable SHA256 hash of an AgentConfigMap.
// The hash is computed over sorted filenames and their body content only,
// ensuring the same configuration always produces the same hash regardless
// of map iteration order or content type metadata.
func HashAgentConfigMap(configMap *protobufs.AgentConfigMap) []byte {
	if configMap == nil || len(configMap.ConfigMap) == 0 {
		return []byte{}
	}

	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(configMap.ConfigMap))
	for k := range configMap.ConfigMap {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	h := sha256.New()
	for _, k := range keys {
		file := configMap.ConfigMap[k]
		if file == nil {
			continue
		}
		// Write filename and body to hash
		h.Write([]byte(k))
		h.Write(file.Body)
	}

	return h.Sum(nil)
}
