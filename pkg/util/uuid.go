package util

import "github.com/google/uuid"

// NewUUID generates a new v7 uuid
func NewUUID() string {
	return uuid.Must(uuid.NewV7()).String()
}
