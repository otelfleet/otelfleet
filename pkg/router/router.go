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

// func match

type Matcher interface {
	Match(ctx context.Context, labels CollectorLabels) string
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
		matchers:  retMatchers,
		configRef: route.ConfigRef,
	}, nil
}

type node struct {
	matchers  []compiledMatcher
	configRef string
	children  []node
}

func (n node) Match(ctx context.Context, labels CollectorLabels) (string, bool) {
	if n.matchSelf(ctx, labels) {
		return n.configRef, true
	}

	for _, child := range n.children {
		configRef, ok := child.Match(ctx, labels)
		if ok {
			return configRef, true
		}
	}

	return "", false
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

func (m matcher) Match(ctx context.Context, labels CollectorLabels) string {
	configRef, ok := m.root.Match(ctx, labels)
	if ok {
		return configRef
	}

	return m.defaultConfigRef
}
