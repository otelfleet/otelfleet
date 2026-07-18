package schema_test

import (
	"context"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestRevision_UnconditionalPutIncrements(t *testing.T) {
	ctx := context.Background()
	s := newSchema(t)
	typeURL, a1 := mustAny(t, wrapperspb.String("one"))
	_, a2 := mustAny(t, wrapperspb.String("two"))

	first, err := s.Put(ctx, typeURL, "k", 0, a1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), first.GetRevision())

	second, err := s.Put(ctx, typeURL, "k", 0, a2)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), second.GetRevision())
}

func TestRevision_CAS(t *testing.T) {
	ctx := context.Background()
	typeURL, a1 := mustAny(t, wrapperspb.String("one"))
	_, a2 := mustAny(t, wrapperspb.String("two"))

	type step struct {
		revision uint64
		wantCode codes.Code // OK == codes.OK
		wantRev  uint64
	}
	cases := []struct {
		name  string
		steps []step
	}{
		{
			name: "create then next revision",
			steps: []step{
				{revision: 1, wantRev: 1},
				{revision: 2, wantRev: 2},
			},
		},
		{
			name: "equal to current is a conflict",
			steps: []step{
				{revision: 1, wantRev: 1},
				{revision: 1, wantCode: codes.Aborted},
			},
		},
		{
			name: "below current is invalid",
			steps: []step{
				{revision: 1, wantRev: 1},
				{revision: 2, wantRev: 2},
				{revision: 1, wantCode: codes.InvalidArgument},
			},
		},
		{
			name: "gap above next is invalid",
			steps: []step{
				{revision: 1, wantRev: 1},
				{revision: 3, wantCode: codes.InvalidArgument},
			},
		},
		{
			name: "create with a gap is invalid",
			steps: []step{
				{revision: 2, wantCode: codes.InvalidArgument},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSchema(t)
			for i, st := range c.steps {
				payload := a1
				if i%2 == 1 {
					payload = a2
				}
				got, err := s.Put(ctx, typeURL, "k", st.revision, payload)
				if st.wantCode != codes.OK {
					require.Error(t, err)
					assert.True(t, grpcutil.IsError(st.wantCode, err), "step %d: want %s, got %v", i, st.wantCode, err)
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, st.wantRev, got.GetRevision())
			}
		})
	}
}

func TestRevision_NoOpUpdateKeepsRevision(t *testing.T) {
	ctx := context.Background()
	s := newSchema(t)
	typeURL, a := mustAny(t, wrapperspb.String("same"))

	first, err := s.Put(ctx, typeURL, "k", 0, a)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.GetRevision())

	_, sameContent := mustAny(t, wrapperspb.String("same"))
	second, err := s.Put(ctx, typeURL, "k", 0, sameContent)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), second.GetRevision())
	assert.Equal(t, first.GetHash(), second.GetHash())
}

func TestRevision_GetByRevision(t *testing.T) {
	ctx := context.Background()
	s := newSchema(t)
	typeURL, a1 := mustAny(t, wrapperspb.String("one"))
	_, a2 := mustAny(t, wrapperspb.String("two"))

	_, err := s.Put(ctx, typeURL, "k", 0, a1)
	require.NoError(t, err)
	_, err = s.Put(ctx, typeURL, "k", 0, a2)
	require.NoError(t, err)

	old, err := s.GetRevision(ctx, typeURL, "k", 1)
	require.NoError(t, err)
	assert.Empty(t, anyDiff(a1, old.GetObj()))

	latest, err := s.Get(ctx, typeURL, "k")
	require.NoError(t, err)
	assert.Empty(t, anyDiff(a2, latest.GetObj()))
	assert.Equal(t, uint64(2), latest.GetRevision())
}

func TestRevision_DeleteThenRecreateIsMonotonic(t *testing.T) {
	ctx := context.Background()
	s := newSchema(t)
	typeURL, a := mustAny(t, wrapperspb.String("v"))

	_, err := s.Put(ctx, typeURL, "k", 0, a)
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, typeURL, "k"))

	_, err = s.Get(ctx, typeURL, "k")
	require.True(t, grpcutil.IsErrorNotFound(err))

	recreated, err := s.Put(ctx, typeURL, "k", 0, a)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), recreated.GetRevision())
}
