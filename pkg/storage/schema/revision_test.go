package schema_test

import (
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/go-cmp/cmp"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"
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

func TestRevision_History(t *testing.T) {
	s := newSchema(t)
	type testcase struct {
		limit    uint64
		offset   uint64
		expected []*anypb.Any
	}
	ctx := t.Context()
	for i := range 50 {
		typeURL, v := mustAny(t, &wrappers.Int64Value{
			Value: int64(i),
		})
		_, err := s.Put(ctx, typeURL, "foo", 0, v)
		require.NoError(t, err)
	}

	val := func(i int64) *anypb.Any {
		return mustAnyNoType(t, &wrappers.Int64Value{Value: i})
	}

	tcs := []testcase{
		{
			limit:    5,
			offset:   0,
			expected: []*anypb.Any{val(49), val(48), val(47), val(46), val(45)},
		},
		{
			limit:    5,
			offset:   5,
			expected: []*anypb.Any{val(44), val(43), val(42), val(41), val(40)},
		},
		{
			limit:    0,
			offset:   49,
			expected: []*anypb.Any{val(0)},
		},
		{
			limit:    0,
			offset:   50,
			expected: []*anypb.Any{},
		},
	}
	typeURL, _ := mustAny(t, &wrappers.Int64Value{
		Value: int64(0),
	})
	for _, tc := range tcs {
		resp, err := s.History(ctx, typeURL, "foo", tc.offset, tc.limit)
		require.NoError(t, err)
		got := make([]*anypb.Any, len(resp.Objs))
		for i, obj := range resp.Objs {
			got[i] = obj.GetObj()
		}
		assert.Empty(t, cmp.Diff(tc.expected, got, protocmp.Transform()))
	}

}
