package storage_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/google/go-cmp/cmp"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProtoStorage(t *testing.T) {
	db, err := pebble.Open("", &pebble.Options{
		FS: vfs.NewMem(),
	})
	require.NoError(t, err)
	broker := otelpebble.NewKVBroker(db)
	kv := broker.KeyValue("test")
	protoKv := storage.NewProtoKV[*bootstrapv1alpha1.BootstrapToken](slog.Default(), kv)

	tok := &bootstrapv1alpha1.BootstrapToken{
		ID:     "b1",
		Secret: "TODO",
		TTL:    durationpb.New(time.Hour),
		Expiry: timestamppb.Now(),
	}

	require.NoError(t, protoKv.Put(t.Context(), "b1", tok))

	ret, err := protoKv.Get(t.Context(), "b1")
	require.NoError(t, err)
	assert.Empty(t, cmp.Diff(ret, tok, protocmp.Transform()))

	keys, err := protoKv.ListKeys(t.Context())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"b1"}, keys)

	vals, err := protoKv.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, len(vals))
}
