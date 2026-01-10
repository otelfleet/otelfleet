package util

import "github.com/google/uuid"

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
