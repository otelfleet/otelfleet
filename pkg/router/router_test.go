package router_test

import (
	"regexp"
	"testing"

	"github.com/otelfleet/otelfleet/pkg/api/route/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/common/v1alpha1"
)

func TestRouter_Empty(t *testing.T) {
	pb := &v1alpha1.Router{
		Root:      nil,
		Defs:      []*v1alpha1.Route{},
		ConfigRef: "foo",
	}

	matcher, err := router.NewMatcher(pb)
	assert.NoError(t, err)
	assert.NotNil(t, matcher)
}

func TestRouter_Matching(t *testing.T) {
	pb := &v1alpha1.Router{
		ConfigRef: "default_foo",
		Root: &v1alpha1.Route{
			Name: "matcher",
			Filters: []*commonv1alpha1.LabelFilter{
				{
					Type: commonv1alpha1.LabelType_LabelTypeIdentifying,
					Filters: []*commonv1alpha1.KeyPairFilter{
						{
							MatchType: commonv1alpha1.MatchType_MATCH_TYPE_EQ,
							Label: &commonv1alpha1.Label{
								Key:   "foo",
								Value: "bar",
							},
						},
					},
				},
			},
			ConfigRef: "nested_foo",
		},
	}

	type testcase struct {
		name    string
		labels  router.CollectorLabels
		wantRef string
	}

	tcs := []testcase{
		{
			name:    "no matchers",
			labels:  router.CollectorLabels{},
			wantRef: "default_foo",
		},
		{
			name: "identifying label matches",
			labels: router.CollectorLabels{
				Identifying: map[string]string{"foo": "bar"},
			},
			wantRef: "nested_foo",
		},
		{
			name: "right key wrong value",
			labels: router.CollectorLabels{
				Identifying: map[string]string{"foo": "baz"},
			},
			wantRef: "default_foo",
		},
		{
			name: "right pair in the wrong label bucket",
			labels: router.CollectorLabels{
				NonIdentifying: map[string]string{"foo": "bar"},
				Otelfleet:      map[string]string{"foo": "bar"},
			},
			wantRef: "default_foo",
		},
		{
			name: "unrelated labels present",
			labels: router.CollectorLabels{
				Identifying: map[string]string{"other": "bar"},
			},
			wantRef: "default_foo",
		},
	}

	m, err := router.NewMatcher(pb)
	require.NoError(t, err)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantRef, m.Match(t.Context(), tc.labels).ConfigRef)
		})
	}

}

func filter(lt commonv1alpha1.LabelType, mt commonv1alpha1.MatchType, key, value string) *commonv1alpha1.LabelFilter {
	return &commonv1alpha1.LabelFilter{
		Type: lt,
		Filters: []*commonv1alpha1.KeyPairFilter{
			{
				MatchType: mt,
				Label:     &commonv1alpha1.Label{Key: key, Value: value},
			},
		},
	}
}

func TestRouter_MatchingTree(t *testing.T) {
	const (
		identifying    = commonv1alpha1.LabelType_LabelTypeIdentifying
		nonIdentifying = commonv1alpha1.LabelType_LabelTypeNonIdentifying
		otelfleet      = commonv1alpha1.LabelType_LabelTypeOtelfleet

		eq  = commonv1alpha1.MatchType_MATCH_TYPE_EQ
		neq = commonv1alpha1.MatchType_MATCH_TYPE_NEQ
		re  = commonv1alpha1.MatchType_MATCH_TYPE_RE
	)

	pb := &v1alpha1.Router{
		ConfigRef: "default_cfg",
		Defs: []*v1alpha1.Route{
			{
				Name:      "shared",
				Filters:   []*commonv1alpha1.LabelFilter{filter(identifying, eq, "k6", "v6")},
				ConfigRef: "shared_cfg",
				Routes: []*v1alpha1.Route{
					{
						Name:      "shared_child",
						Filters:   []*commonv1alpha1.LabelFilter{filter(identifying, eq, "k6", "v7")},
						ConfigRef: "shared_child_cfg",
					},
				},
			},
		},
		Root: &v1alpha1.Route{
			Name:      "root",
			Filters:   []*commonv1alpha1.LabelFilter{filter(identifying, eq, "k1", "v1")},
			ConfigRef: "root_cfg",
			Routes: []*v1alpha1.Route{
				{
					Name:      "regex",
					Filters:   []*commonv1alpha1.LabelFilter{filter(identifying, re, "k2", `^v2-\d+$`)},
					ConfigRef: "regex_cfg",
				},
				{
					Name: "conjunction",
					Filters: []*commonv1alpha1.LabelFilter{
						filter(identifying, eq, "k3", "v3"),
						filter(otelfleet, eq, "k4", "v4"),
					},
					ConfigRef: "conjunction_cfg",
				},
				{
					Name:      "otelfleet_bucket",
					Filters:   []*commonv1alpha1.LabelFilter{filter(otelfleet, eq, "k1", "v1")},
					ConfigRef: "otelfleet_cfg",
				},
				{
					Name:      "no_config_ref",
					Filters:   []*commonv1alpha1.LabelFilter{filter(identifying, eq, "k5", "v5")},
					ConfigRef: "",
				},
				{
					Name: "from_def",
					Use:  "shared",
				},
				{
					Name:      "negated",
					Filters:   []*commonv1alpha1.LabelFilter{filter(nonIdentifying, neq, "k7", "v8")},
					ConfigRef: "negated_cfg",
				},
			},
		},
	}

	m, err := router.NewMatcher(pb)
	require.NoError(t, err)

	type testcase struct {
		name    string
		labels  router.CollectorLabels
		wantRef string
	}

	// the last child matches k7 != v8, so cases that should not reach it carry k7=v8
	blockNegated := map[string]string{"k7": "v8"}

	tcs := []testcase{
		{
			name: "deepest match wins over the root",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k1": "v1", "k2": "v2-01"},
				NonIdentifying: blockNegated,
			},
			wantRef: "regex_cfg",
		},
		{
			name: "root matches when no child does",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k1": "v1"},
				NonIdentifying: blockNegated,
			},
			wantRef: "root_cfg",
		},
		{
			name: "regex child",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k2": "v2-01"},
				NonIdentifying: blockNegated,
			},
			wantRef: "regex_cfg",
		},
		{
			name: "regex child anchored, no partial match",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k2": "xx-v2-01-yy"},
				NonIdentifying: blockNegated,
			},
			wantRef: "default_cfg",
		},
		{
			name: "filters on one route are ANDed",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k3": "v3"},
				NonIdentifying: blockNegated,
				Otelfleet:      map[string]string{"k4": "v4"},
			},
			wantRef: "conjunction_cfg",
		},
		{
			name: "one half of the conjunction is not enough",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k3": "v3"},
				NonIdentifying: blockNegated,
			},
			wantRef: "default_cfg",
		},
		{
			name: "otelfleet bucket is matched separately from identifying",
			labels: router.CollectorLabels{
				NonIdentifying: blockNegated,
				Otelfleet:      map[string]string{"k1": "v1"},
			},
			wantRef: "otelfleet_cfg",
		},
		{
			name: "matching route with no config_ref does not fall back to the default",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k5": "v5"},
				NonIdentifying: blockNegated,
			},
			wantRef: "",
		},
		{
			name: "route expanded from a def",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k6": "v6"},
				NonIdentifying: blockNegated,
			},
			wantRef: "shared_cfg",
		},
		{
			name: "child of a route expanded from a def",
			labels: router.CollectorLabels{
				Identifying:    map[string]string{"k6": "v7"},
				NonIdentifying: blockNegated,
			},
			wantRef: "shared_child_cfg",
		},
		{
			name: "negated match, key present with another value",
			labels: router.CollectorLabels{
				NonIdentifying: map[string]string{"k7": "v9"},
			},
			wantRef: "negated_cfg",
		},
		{
			name: "negated match, key equal",
			labels: router.CollectorLabels{
				NonIdentifying: blockNegated,
			},
			wantRef: "default_cfg",
		},
		{
			name:    "negated match, key absent",
			labels:  router.CollectorLabels{},
			wantRef: "default_cfg",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantRef, m.Match(t.Context(), tc.labels).ConfigRef)
		})
	}
}

func TestRouter_Invalid(t *testing.T) {
	t.Run("invalid regex", func(t *testing.T) {

		_, err := regexp.Compile("[invalid")
		assert.Error(t, err)

		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Root: &v1alpha1.Route{
				Name: "foo",
				Filters: []*commonv1alpha1.LabelFilter{
					{
						Type: commonv1alpha1.LabelType_LabelTypeIdentifying,
						Filters: []*commonv1alpha1.KeyPairFilter{
							{
								MatchType: commonv1alpha1.MatchType_MATCH_TYPE_RE,
								Label: &commonv1alpha1.Label{
									Key:   "foo",
									Value: "[invalid",
								},
							},
						},
					},
				},
			},
		}

		_, err = router.NewMatcher(pb)
		assert.Error(t, err)
	})

	t.Run("cyclic", func(t *testing.T) {
		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Defs: []*v1alpha1.Route{
				{
					Name: "goto_bar",
					Routes: []*v1alpha1.Route{
						{
							Name: "bar",
							Use:  "goto_foo",
						},
					},
				},
				{
					Name: "bar",
					Routes: []*v1alpha1.Route{
						{
							Name: "asd",
							Use:  "goto_foo",
						},
					},
				},

				{
					Name: "goto_foo",
					Routes: []*v1alpha1.Route{
						{
							Name: "todo",
							Use:  "goto_bar",
						},
					},
				},
			},

			Root: &v1alpha1.Route{
				Name: "root",
				Use:  "goto_bar",
			},
		}

		_, err := router.NewMatcher(pb)
		assert.Error(t, err)
	})

	t.Run("cyclic with distinct referrer names", func(t *testing.T) {
		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Defs: []*v1alpha1.Route{
				{
					Name:   "a",
					Routes: []*v1alpha1.Route{{Name: "n1", Use: "b"}},
				},
				{
					Name:   "b",
					Routes: []*v1alpha1.Route{{Name: "n2", Use: "c"}},
				},
				{
					Name:   "c",
					Routes: []*v1alpha1.Route{{Name: "n3", Use: "a"}},
				},
			},
			Root: &v1alpha1.Route{Name: "root", Use: "a"},
		}

		_, err := router.NewMatcher(pb)
		assert.Error(t, err)
	})
}

func TestRouter_Acyclic(t *testing.T) {
	t.Run("same def used by two siblings", func(t *testing.T) {
		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Defs: []*v1alpha1.Route{
				{
					Name:      "common",
					ConfigRef: "common_cfg",
					Routes:    []*v1alpha1.Route{{Name: "leaf", ConfigRef: "leaf_cfg"}},
				},
			},
			Root: &v1alpha1.Route{
				Name: "root",
				Routes: []*v1alpha1.Route{
					{Name: "left", Use: "common"},
					{Name: "right", Use: "common"},
				},
			},
		}

		_, err := router.NewMatcher(pb)
		assert.NoError(t, err)
	})

	t.Run("repeated names in disjoint subtrees", func(t *testing.T) {
		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Root: &v1alpha1.Route{
				Name: "root",
				Routes: []*v1alpha1.Route{
					{Name: "a", Routes: []*v1alpha1.Route{{Name: "prod", ConfigRef: "p1"}}},
					{Name: "b", Routes: []*v1alpha1.Route{{Name: "prod", ConfigRef: "p2"}}},
				},
			},
		}

		_, err := router.NewMatcher(pb)
		assert.NoError(t, err)
	})

	t.Run("unnamed routes", func(t *testing.T) {
		pb := &v1alpha1.Router{
			ConfigRef: "default",
			Root: &v1alpha1.Route{
				Name: "root",
				Routes: []*v1alpha1.Route{
					{ConfigRef: "a"},
					{ConfigRef: "b"},
				},
			},
		}

		_, err := router.NewMatcher(pb)
		assert.NoError(t, err)
	})
}

func TestRouter_MatchResult(t *testing.T) {
	pb := &v1alpha1.Router{
		ConfigRef: "default",
		Root: &v1alpha1.Route{
			Name:      "root",
			ConfigRef: "root_cfg",
			Filters: []*commonv1alpha1.LabelFilter{
				{
					Type: commonv1alpha1.LabelType_LabelTypeOtelfleet,
					Filters: []*commonv1alpha1.KeyPairFilter{
						{
							MatchType: commonv1alpha1.MatchType_MATCH_TYPE_EQ,
							Label:     &commonv1alpha1.Label{Key: "env", Value: "prod"},
						},
					},
				},
			},
			Routes: []*v1alpha1.Route{
				{
					Name:      "other",
					ConfigRef: "other_cfg",
					Filters: []*commonv1alpha1.LabelFilter{
						{
							Type: commonv1alpha1.LabelType_LabelTypeOtelfleet,
							Filters: []*commonv1alpha1.KeyPairFilter{
								{
									MatchType: commonv1alpha1.MatchType_MATCH_TYPE_EQ,
									Label:     &commonv1alpha1.Label{Key: "env", Value: "other"},
								},
							},
						},
					},
				},
				{
					Name:      "eu",
					ConfigRef: "eu_cfg",
					Filters: []*commonv1alpha1.LabelFilter{
						{
							Type: commonv1alpha1.LabelType_LabelTypeNonIdentifying,
							Filters: []*commonv1alpha1.KeyPairFilter{
								{
									MatchType: commonv1alpha1.MatchType_MATCH_TYPE_EQ,
									Label:     &commonv1alpha1.Label{Key: "host.name", Value: "eu-1"},
								},
							},
						},
					},
				},
			},
		},
	}

	m, err := router.NewMatcher(pb)
	require.NoError(t, err)

	t.Run("nested route", func(t *testing.T) {
		res := m.Match(t.Context(), router.CollectorLabels{
			NonIdentifying: map[string]string{"host.name": "eu-1"},
		})
		assert.True(t, res.Matched)
		assert.Equal(t, "eu_cfg", res.ConfigRef)
		assert.Equal(t, []string{"root", "eu"}, res.Path)
		assert.Equal(t, []uint32{1}, res.IndexPath)
	})

	t.Run("root route", func(t *testing.T) {
		res := m.Match(t.Context(), router.CollectorLabels{
			Otelfleet: map[string]string{"env": "prod"},
		})
		assert.True(t, res.Matched)
		assert.Equal(t, "root_cfg", res.ConfigRef)
		assert.Equal(t, []string{"root"}, res.Path)
		assert.Empty(t, res.IndexPath)
	})

	t.Run("no match falls back to the default", func(t *testing.T) {
		res := m.Match(t.Context(), router.CollectorLabels{})
		assert.False(t, res.Matched)
		assert.Equal(t, "default", res.ConfigRef)
		assert.Empty(t, res.Path)
	})
}
