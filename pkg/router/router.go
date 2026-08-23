package router

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"

	"github.com/otelfleet/otelfleet/pkg/api/route/v1alpha1"
)

type CollectorLabels struct {
	Identifying    map[string]string
	NonIdentifying map[string]string
	Otelfleet      map[string]string
}

type MatchResult struct {
	ConfigRef string
	// Names of the routes traversed, from the root to the matched route.
	Path []string
	// Index of each traversed route in its parent's routes, from the root down.
	IndexPath []uint32
	Matched   bool
}

type Matcher interface {
	Match(ctx context.Context, labels CollectorLabels) MatchResult
}

func NewMatcher(pb *v1alpha1.Router) (Matcher, error) {
	return convert(pb)
}

func convert(pb *v1alpha1.Router) (matcher, error) {
	defs := pb.GetDefs()
	defsMap := lo.Associate(defs, func(r *v1alpha1.Route) (string, *v1alpha1.Route) {
		return r.GetName(), r
	})
	if len(defs) != len(defsMap) {
		return matcher{}, fmt.Errorf("duplicate defs in Router")
	}

	root := pb.GetRoot()
	curPath := map[string]struct{}{}
	node, err := convertNode(root, defsMap, curPath)
	if err != nil {
		return matcher{}, err
	}
	return matcher{
		root:             node,
		defaultConfigRef: pb.GetConfigRef(),
	}, nil

}

func convertNode(route *v1alpha1.Route, defs map[string]*v1alpha1.Route, activePath map[string]struct{}) (node, error) {
	toConvert := route
	if useRef := route.GetUse(); useRef != "" {
		actualRoute, ok := defs[useRef]
		if !ok {
			return node{}, fmt.Errorf("route %s references a def that doesn't exist %s", route.GetName(), useRef)
		}
		if _, cycle := activePath[useRef]; cycle {
			return node{}, fmt.Errorf("route %s: cyclic reference to def %s", route.GetName(), useRef)
		}
		activePath[useRef] = struct{}{}
		defer delete(activePath, useRef)

		toConvert = proto.Clone(actualRoute).(*v1alpha1.Route)
		toConvert.Name = route.Name
	}

	curNode, err := convertRaw(toConvert)
	if err != nil {
		return node{}, err
	}
	curNode.children = make([]node, len(toConvert.GetRoutes()))
	for idx, nestedRoute := range toConvert.GetRoutes() {
		nestedNode, err := convertNode(nestedRoute, defs, activePath)
		if err != nil {
			return node{}, err
		}
		curNode.children[idx] = nestedNode
	}

	return curNode, nil
}

func convertRaw(route *v1alpha1.Route) (node, error) {
	retMatchers := []compiledMatcher{}
	for _, routeFilter := range route.GetFilters() {
		matchers, err := compileMatcher(routeFilter)
		if err != nil {
			return node{}, err
		}
		retMatchers = append(retMatchers, matchers...)
	}
	return node{
		name:      route.GetName(),
		matchers:  retMatchers,
		configRef: route.ConfigRef,
	}, nil
}

type node struct {
	name      string
	matchers  []compiledMatcher
	configRef string
	children  []node
}

func (n node) Match(ctx context.Context, labels CollectorLabels) (MatchResult, bool) {
	for idx, child := range n.children {
		res, ok := child.Match(ctx, labels)
		if ok {
			res.Path = append([]string{n.name}, res.Path...)
			res.IndexPath = append([]uint32{uint32(idx)}, res.IndexPath...)
			return res, true
		}
	}

	if n.matchSelf(ctx, labels) {
		return MatchResult{
			ConfigRef: n.configRef,
			Path:      []string{n.name},
			Matched:   true,
		}, true
	}

	return MatchResult{}, false
}

func (n node) matchSelf(ctx context.Context, labels CollectorLabels) bool {
	for _, matcher := range n.matchers {
		if !matcher.Match(ctx, labels) {
			return false
		}
	}
	return true
}

type matcher struct {
	root             node
	defaultConfigRef string
}

func (m matcher) Match(ctx context.Context, labels CollectorLabels) MatchResult {
	res, ok := m.root.Match(ctx, labels)
	if ok {
		return res
	}

	return MatchResult{ConfigRef: m.defaultConfigRef}
}
