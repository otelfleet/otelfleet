package opamp

import (
	"context"
	"log/slog"
	"regexp"
	"sort"
	"sync/atomic"
	"time"

	resourcesv1alpha1 "github.com/otelfleet/otelfleet/pkg/api/resources/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage"
	"github.com/otelfleet/otelfleet/pkg/storage/schema"
	stypes "github.com/otelfleet/otelfleet/pkg/storage/types"
)

const defaultConfigFilterSyncInterval = 30 * time.Second

type AgentLabels struct {
	Identifying    map[string]string
	NonIdentifying map[string]string
	Otelfleet      map[string]string
}

type ConfigFilterSyncOptions struct {
	Logger   *slog.Logger
	Storage  schema.SchemaProto
	Interval time.Duration
}

type ConfigFilterSync struct {
	ConfigFilterSyncOptions

	store    stypes.KeyValue[*resourcesv1alpha1.ConfigFilter]
	snapshot atomic.Pointer[filterSnapshot]
}

func NewConfigFilterSync(opts ConfigFilterSyncOptions) *ConfigFilterSync {
	s := &ConfigFilterSync{
		ConfigFilterSyncOptions: opts,
		store:                   storage.NewProtoKVFromSchemaImpl[*resourcesv1alpha1.ConfigFilter](opts.Storage),
	}
	s.snapshot.Store(&filterSnapshot{})
	return s
}

func (s *ConfigFilterSync) start(ctx context.Context) error {
	return s.sync(ctx)
}

func (s *ConfigFilterSync) stopping() {
	s.snapshot.Store(&filterSnapshot{})
}

func (s *ConfigFilterSync) running(ctx context.Context) error {
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := s.sync(ctx); err != nil {
				s.Logger.With("err", err).Error("failed to sync config filters")
			}
		}
	}
}

func (s *ConfigFilterSync) sync(ctx context.Context) error {
	keys, err := s.store.ListKeys(ctx)
	if err != nil {
		return err
	}
	sort.Strings(keys)

	snap := &filterSnapshot{}
	for _, key := range keys {
		filter, err := s.store.Get(ctx, key)
		if err != nil {
			s.Logger.With("key", key, "err", err).Warn("skipping unreadable config filter")
			continue
		}
		compiled, err := compileFilter(key, filter)
		if err != nil {
			s.Logger.With("key", key, "err", err).Warn("skipping invalid config filter")
			continue
		}
		snap.filters = append(snap.filters, compiled)
		if filter.GetDefault() && snap.defaultFilter == nil {
			snap.defaultFilter = compiled
		}
	}
	s.snapshot.Store(snap)
	return nil
}

// Match returns the filter matching the most labels, or the default filter when nothing matches.
func (s *ConfigFilterSync) Match(labels AgentLabels) *resourcesv1alpha1.ConfigFilter {
	snap := s.snapshot.Load()

	var best *compiledFilter
	bestScore := 0
	for _, f := range snap.filters {
		score := f.score(labels)
		if score > bestScore {
			best, bestScore = f, score
		}
	}
	if best == nil {
		best = snap.defaultFilter
	}
	if best == nil {
		return nil
	}
	return best.filter
}

type filterSnapshot struct {
	filters       []*compiledFilter
	defaultFilter *compiledFilter
}

type compiledFilter struct {
	key    string
	filter *resourcesv1alpha1.ConfigFilter
	groups []labelMatcherGroup
}

type labelMatcherGroup struct {
	identifying    []labelMatcher
	nonIdentifying []labelMatcher
	otelfleet      []labelMatcher
}

type labelMatcher struct {
	matchType resourcesv1alpha1.MatchType
	key       string
	value     string
	re        *regexp.Regexp
}

func compileFilter(key string, filter *resourcesv1alpha1.ConfigFilter) (*compiledFilter, error) {
	compiled := &compiledFilter{key: key, filter: filter}
	for _, lf := range filter.GetFilters() {
		group := labelMatcherGroup{}
		for _, set := range []struct {
			labels map[string]string
			out    *[]labelMatcher
		}{
			{lf.GetOpampIdLabels(), &group.identifying},
			{lf.GetOpampNonIdLabels(), &group.nonIdentifying},
			{lf.GetOtelfleetLabels(), &group.otelfleet},
		} {
			matchers, err := compileMatchers(lf.GetType(), set.labels)
			if err != nil {
				return nil, err
			}
			*set.out = matchers
		}
		compiled.groups = append(compiled.groups, group)
	}
	return compiled, nil
}

func compileMatchers(matchType resourcesv1alpha1.MatchType, labels map[string]string) ([]labelMatcher, error) {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	matchers := make([]labelMatcher, 0, len(keys))
	for _, k := range keys {
		m := labelMatcher{matchType: matchType, key: k, value: labels[k]}
		if matchType == resourcesv1alpha1.MatchType_MATCH_TYPE_RE || matchType == resourcesv1alpha1.MatchType_MATCH_TYPE_NR {
			re, err := regexp.Compile(labels[k])
			if err != nil {
				return nil, err
			}
			m.re = re
		}
		matchers = append(matchers, m)
	}
	return matchers, nil
}

// score sums the matched labels across every label filter that fully matches.
func (c *compiledFilter) score(labels AgentLabels) int {
	total := 0
	for _, group := range c.groups {
		score, ok := group.score(labels)
		if !ok {
			return 0
		}
		total += score
	}
	return total
}

func (g labelMatcherGroup) score(labels AgentLabels) (int, bool) {
	score := 0
	for _, set := range []struct {
		matchers []labelMatcher
		labels   map[string]string
	}{
		{g.identifying, labels.Identifying},
		{g.nonIdentifying, labels.NonIdentifying},
		{g.otelfleet, labels.Otelfleet},
	} {
		for _, m := range set.matchers {
			if !m.matches(set.labels) {
				return 0, false
			}
			score++
		}
	}
	return score, score > 0
}

func (m labelMatcher) matches(labels map[string]string) bool {
	value, ok := labels[m.key]
	switch m.matchType {
	case resourcesv1alpha1.MatchType_MATCH_TYPE_EQ:
		return ok && value == m.value
	case resourcesv1alpha1.MatchType_MATCH_TYPE_NEQ:
		return !ok || value != m.value
	case resourcesv1alpha1.MatchType_MATCH_TYPE_RE:
		return ok && m.re.MatchString(value)
	case resourcesv1alpha1.MatchType_MATCH_TYPE_NR:
		return !ok || !m.re.MatchString(value)
	}
	return false
}
