package schema_test

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/google/go-cmp/cmp"
	bootstrapv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	keyvaluev1 "github.com/otelfleet/otelfleet/pkg/api/keyvalue/v1alpha1"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// newSchema wires a StorageSchemaProto on top of a real (in-memory) KV, so the
// tests observe end-to-end behaviour rather than a hand-rolled fake.
func newSchema(t *testing.T) *schema.StorageSchemaProto {
	t.Helper()
	db, err := pebble.Open("", &pebble.Options{FS: vfs.NewMem()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	kv := otelpebble.NewKVBroker(db).KeyValue("test")
	return schema.NewStorageSchemaProto(kv)
}

func mustAny(t *testing.T, msg proto.Message) (string, *anypb.Any) {
	t.Helper()
	any, err := anypb.New(msg)
	require.NoError(t, err)
	return any.GetTypeUrl(), any
}

func mustAnyNoType(t *testing.T, msg proto.Message) *anypb.Any {
	t.Helper()
	any, err := anypb.New(msg)
	require.NoError(t, err)
	return any
}

func anyDiff(want, got *anypb.Any) string {
	return cmp.Diff(want, got, protocmp.Transform())
}

func putAny(t *testing.T, ctx context.Context, s *schema.StorageSchemaProto, typeURL, key string, obj *anypb.Any) {
	t.Helper()
	_, err := s.Put(ctx, typeURL, key, 0, obj)
	require.NoError(t, err)
}

func TestStorageSchemaProto_PutGet(t *testing.T) {
	type tc struct {
		name string
		key  string
		msg  proto.Message
	}
	cases := []tc{
		{
			name: "duration",
			key:  "d1",
			msg:  durationpb.New(90 * time.Minute),
		},
		{
			name: "timestamp",
			key:  "t1",
			msg:  timestamppb.New(time.Unix(1_700_000_000, 0)),
		},
		{
			name: "string wrapper",
			key:  "s1",
			msg:  wrapperspb.String("hello world"),
		},
		{
			name: "bootstrap token",
			key:  "b1",
			msg: &bootstrapv1alpha1.BootstrapToken{
				ID:     "b1",
				Secret: "shhh",
				TTL:    durationpb.New(time.Hour),
				Expiry: timestamppb.New(time.Unix(42, 0)),
			},
		},
		{
			name: "key with slashes",
			key:  "nested/key/name",
			msg:  wrapperspb.String("nested"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			s := newSchema(t)
			typeURL, want := mustAny(t, c.msg)

			putAny(t, ctx, s, typeURL, c.key, want)

			got, err := s.Get(ctx, typeURL, c.key)
			require.NoError(t, err)
			assert.Empty(t, anyDiff(want, got.GetObj()))
		})
	}
}

func TestStorageSchemaProto_Get_NotFound(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, ctx context.Context, s *schema.StorageSchemaProto, typeURL string)
		key   string
	}
	cases := []tc{
		{
			name:  "empty store",
			setup: func(*testing.T, context.Context, *schema.StorageSchemaProto, string) {},
			key:   "missing",
		},
		{
			name: "wrong key",
			setup: func(t *testing.T, ctx context.Context, s *schema.StorageSchemaProto, typeURL string) {
				_, any := mustAny(t, wrapperspb.String("present"))
				putAny(t, ctx, s, typeURL, "present", any)
			},
			key: "absent",
		},
		{
			name: "after delete",
			setup: func(t *testing.T, ctx context.Context, s *schema.StorageSchemaProto, typeURL string) {
				_, any := mustAny(t, wrapperspb.String("v"))
				putAny(t, ctx, s, typeURL, "gone", any)
				require.NoError(t, s.Delete(ctx, typeURL, "gone"))
			},
			key: "gone",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			s := newSchema(t)
			typeURL, _ := mustAny(t, wrapperspb.String(""))
			c.setup(t, ctx, s, typeURL)

			got, err := s.Get(ctx, typeURL, c.key)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.True(t, grpcutil.IsErrorNotFound(err), "expected NotFound, got %v", err)
		})
	}
}

func TestStorageSchemaProto_Delete(t *testing.T) {
	type tc struct {
		name       string
		key        string
		deleteKey  string
		wantGetErr bool // original key should be gone afterwards
	}
	cases := []tc{
		{
			name:       "delete existing key",
			key:        "k1",
			deleteKey:  "k1",
			wantGetErr: true,
		},
		{
			name:       "delete unrelated key leaves original",
			key:        "k1",
			deleteKey:  "other",
			wantGetErr: false,
		},
		{
			name:       "delete missing key is a no-op",
			key:        "k1",
			deleteKey:  "does-not-exist",
			wantGetErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			s := newSchema(t)
			typeURL, any := mustAny(t, wrapperspb.String("payload"))
			putAny(t, ctx, s, typeURL, c.key, any)

			require.NoError(t, s.Delete(ctx, typeURL, c.deleteKey))

			got, err := s.Get(ctx, typeURL, c.key)
			if c.wantGetErr {
				require.Error(t, err)
				assert.True(t, grpcutil.IsErrorNotFound(err), "expected NotFound, got %v", err)
			} else {
				require.NoError(t, err)
				assert.Empty(t, anyDiff(any, got.GetObj()))
			}
		})
	}
}

func TestStorageSchemaProto_ListKeys(t *testing.T) {
	// two distinct proto types => two distinct typeURLs, used to assert that
	// listing one type never leaks the keys of another.
	typeStr, _ := mustAny(t, wrapperspb.String(""))
	typeDur, _ := mustAny(t, durationpb.New(0))

	type entry struct {
		typeURL string
		key     string
	}
	type tc struct {
		name     string
		queryURL string
		wantKeys []string
	}

	entries := []entry{
		{typeStr, "a"},
		{typeStr, "b"},
		{typeStr, "c"},
		{typeDur, "d"},
	}
	cases := []tc{
		{
			name:     "string type keys",
			queryURL: typeStr,
			wantKeys: []string{"a", "b", "c"},
		},
		{
			name:     "duration type keys",
			queryURL: typeDur,
			wantKeys: []string{"d"},
		},
		{
			name:     "unknown type is empty",
			queryURL: "type.googleapis.com/google.protobuf.BoolValue",
			wantKeys: []string{},
		},
	}

	ctx := context.Background()
	s := newSchema(t)
	for _, e := range entries {
		putAny(t, ctx, s, e.typeURL, e.key, anyForType(t, e.typeURL))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			keys, err := s.ListKeys(ctx, c.queryURL)
			require.NoError(t, err)
			assert.ElementsMatch(t, c.wantKeys, keys)
		})
	}
}

func TestStorageSchemaProto_List(t *testing.T) {
	type tc struct {
		name     string
		queryURL string
		wantVals []*anypb.Any
	}

	typeStr, av1 := mustAny(t, wrapperspb.String("one"))
	_, av2 := mustAny(t, wrapperspb.String("two"))
	typeDur, adur := mustAny(t, durationpb.New(time.Second))

	ctx := context.Background()
	s := newSchema(t)
	putAny(t, ctx, s, typeStr, "k1", av1)
	putAny(t, ctx, s, typeStr, "k2", av2)
	putAny(t, ctx, s, typeDur, "k3", adur)

	cases := []tc{
		{
			name:     "lists all values for a type",
			queryURL: typeStr,
			wantVals: []*anypb.Any{av1, av2},
		},
		{
			name:     "isolated per type",
			queryURL: typeDur,
			wantVals: []*anypb.Any{adur},
		},
		{
			name:     "unknown type is empty",
			queryURL: "type.googleapis.com/google.protobuf.BoolValue",
			wantVals: []*anypb.Any{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.List(ctx, c.queryURL)
			require.NoError(t, err)
			require.Len(t, got, len(c.wantVals))
			// List makes no ordering guarantee, so compare as an unordered set.
			assert.ElementsMatch(t, anySet(c.wantVals), kvoSet(got))
		})
	}
}

// anySet renders Any values into a comparable form for order-independent
// set comparison via ElementsMatch.
func anySet(anys []*anypb.Any) []string {
	out := make([]string, len(anys))
	for i, a := range anys {
		out[i] = a.GetTypeUrl() + "|" + string(a.GetValue())
	}
	return out
}

func kvoSet(objs []*keyvaluev1.KeyValueObject) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.GetObj().GetTypeUrl() + "|" + string(o.GetObj().GetValue())
	}
	return out
}

// anyForType constructs a valid Any for one of the type URLs used above.
func anyForType(t *testing.T, typeURL string) *anypb.Any {
	t.Helper()
	durURL, _ := mustAny(t, durationpb.New(0))
	if typeURL == durURL {
		_, a := mustAny(t, durationpb.New(time.Second))
		return a
	}
	_, a := mustAny(t, wrapperspb.String("v"))
	return a
}
