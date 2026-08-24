package router

import (
	"context"
	"fmt"
	"regexp"

	"github.com/otelfleet/otelfleet/pkg/api/common/v1alpha1"
)

type compiledMatcher struct {
	labelType v1alpha1.LabelType
	key       string
	value     string
	re        *regexp.Regexp
	negate    bool
}

func (c compiledMatcher) Match(ctx context.Context, labels CollectorLabels) bool {
	switch c.labelType {
	case v1alpha1.LabelType_LabelTypeIdentifying:
		return c.matchLabels(ctx, labels.Identifying)
	case v1alpha1.LabelType_LabelTypeNonIdentifying:
		return c.matchLabels(ctx, labels.NonIdentifying)
	case v1alpha1.LabelType_LabelTypeOtelfleet:
		return c.matchLabels(ctx, labels.Otelfleet)
	default:
		panic(fmt.Sprintf("unhandled label type : %T", c.labelType))
	}
}

func (c compiledMatcher) withNegate(in bool) bool {
	if c.negate {
		return !in
	}
	return in
}

func (c compiledMatcher) matchLabels(ctx context.Context, labels map[string]string) bool {
	val, ok := labels[c.key]
	if !ok {
		return false
	}
	if c.re != nil {
		return c.withNegate(c.re.MatchString(val))
	}
	return c.withNegate(val == c.value)
}

func compileMatcher(pb *v1alpha1.LabelFilter) ([]compiledMatcher, error) {
	ret := make([]compiledMatcher, len(pb.GetFilters()))
	for idx, filter := range pb.GetFilters() {
		c := compiledMatcher{
			labelType: pb.Type,
			key:       filter.GetLabel().GetKey(),
		}
		value := filter.GetLabel().GetValue()
		switch filter.MatchType {
		case v1alpha1.MatchType_MATCH_TYPE_EQ:
			c.value = value
		case v1alpha1.MatchType_MATCH_TYPE_NEQ:
			c.value = value
			c.negate = true
		case v1alpha1.MatchType_MATCH_TYPE_RE:
			re, err := regexp.Compile(value)
			if err != nil {
				return nil, err
			}
			c.re = re
		case v1alpha1.MatchType_MATCH_TYPE_NRE:
			c.negate = true
			re, err := regexp.Compile(value)
			if err != nil {
				return nil, err
			}
			c.re = re
		}
		ret[idx] = c
	}

	return ret, nil
}
