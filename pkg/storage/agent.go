package storage

import (
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
)

type Agent struct {
	InstanceID string
	Status     *protobufs.AgentToServer
	StartedAt  time.Time
}
