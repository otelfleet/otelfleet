package util

import (
	"fmt"

	"github.com/open-telemetry/opamp-go/protobufs"
	"google.golang.org/protobuf/proto"
)

var (
	deterministicMarshalOpts = proto.MarshalOptions{
		AllowPartial:  true,
		Deterministic: true,
	}
)

func KeyVal(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: val},
		},
	}
}

// ProtoHash a non-cryptographically secure fast hash of the protobuf contents
func ProtoHash[T proto.Message](in T) ([]byte, error) {
	data, err := deterministicMarshalOpts.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proto for hash : %w", err)
	}
	return XXHash(data), nil
}
