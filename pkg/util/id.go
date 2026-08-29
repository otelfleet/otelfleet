package util

import (
	"net/url"

	"github.com/google/uuid"
)

// NewUUID generates a new v7 uuid
func NewUUID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func NewInstanceUUID() [16]byte {
	bytes := Must(uuid.Must(uuid.NewV7()).MarshalBinary())
	var ret [16]byte
	copy(ret[:], bytes)
	return ret
}

func NewURIFromID(deployID string) *url.URL {
	return &url.URL{
		Scheme: "otelfleet",
		Host:   "collectors",
		Path:   deployID,
	}
}
