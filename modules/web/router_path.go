// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-chi/chi/v5"
)

type RouterPathGroup struct {
	r         *Router
	pathParam string
	matchers  []*routerPathMatcher
}

func (g *RouterPathGroup) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	chiCtx := chi.RouteContext(req.Context())
	path := chiCtx.URLParam(g.pathParam)
	for _, m := range g.matchers {
		if m.matchPath(chiCtx, path) {
			handler := m.handlerFunc
			for i := len(m.middlewares) - 1; i >= 0; i-- {
				handler = m.middlewares[i](handler).ServeHTTP
			}
			handler(resp, req)
			return
		}
	}
	g.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
}

type RouterPathGroupPattern struct {
	re          *regexp.Regexp
	params      []routerPathParam
	middlewares []any
}

// MatchPath matches the request method, and uses regexp to match the path.
// The pattern uses "<...>" to define path parameters, for example, "/<name>" (different from chi router)
// It is only designed to resolve some special cases that chi router can't handle.
// For most cases, it shouldn't be used because it needs to iterate all rules to find the matched one (inefficient).
func (g *RouterPathGroup) MatchPath(methods, pattern string, h ...any) {
	g.MatchPattern(methods, g.PatternRegexp(pattern), h...)
}

func (g *RouterPathGroup) MatchPattern(methods string, pattern *RouterPathGroupPattern, h ...any) {
	g.matchers = append(g.matchers, newRouterPathMatcher(methods, pattern, h...))
}

type routerPathParam struct {
	name         string
	captureGroup int
}

type routerPathMatcher struct {
	methods     container.Set[string]
	re          *regexp.Regexp
	params      []routerPathParam
	middlewares []func(http.Handler) http.Handler
	handlerFunc http.HandlerFunc
}

func (p *routerPathMatcher) matchPath(chiCtx *chi.Context, path string) bool {
	if !p.methods.Contains(chiCtx.RouteMethod) {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	pathMatches := p.re.FindStringSubmatchIndex(path) // Golang regexp match pairs [start, end, start, end, ...]
	if pathMatches == nil {
		return false
	}
	var paramMatches [][]int
	for i := 2; i < len(pathMatches); {
		paramMatches = append(paramMatches, []int{pathMatches[i], pathMatches[i+1]})
		pmIdx := len(paramMatches) - 1
		end := pathMatches[i+1]
		i += 2
		for ; i < len(pathMatches); i += 2 {
			if pathMatches[i] >= end {
				break
			}
			paramMatches[pmIdx] = append(paramMatches[pmIdx], pathMatches[i], pathMatches[i+1])
		}
	}
	for i, pm := range paramMatches {
		groupIdx := p.params[i].captureGroup * 2
		chiCtx.URLParams.Add(p.params[i].name, path[pm[groupIdx]:pm[groupIdx+1]])
	}
	return true
}

func isValidMethod(name string) bool {
	switch name {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace:
		return true
	}
	return false
}

func newRouterPathMatcher(methods string, patternRegexp *RouterPathGroupPattern, h ...any) *routerPathMatcher {
	middlewares, handlerFunc := wrapMiddlewareAndHandler(patternRegexp.middlewares, h)
	p := &routerPathMatcher{methods: make(container.Set[string]), middlewares: middlewares, handlerFunc: handlerFunc}
	for method := range strings.SplitSeq(methods, ",") {
		method = strings.TrimSpace(method)
		if !isValidMethod(method) {
			panic("invalid HTTP method: " + method)
		}
		p.methods.Add(method)
	}
	p.re, p.params = patternRegexp.re, patternRegexp.params
	return p
}

func patternRegexp(pattern string, h ...any) *RouterPathGroupPattern {
	p := &RouterPathGroupPattern{middlewares: slices.Clone(h)}
	re := []byte{'^'}
	lastEnd := 0
	for lastEnd < len(pattern) {
		start := strings.IndexByte(pattern[lastEnd:], '<')
		if start == -1 {
			re = append(re, regexp.QuoteMeta(pattern[lastEnd:])...)
			break
		}
		end := strings.IndexByte(pattern[lastEnd+start:], '>')
		if end == -1 {
			panic("invalid pattern: " + pattern)
		}
		re = append(re, regexp.QuoteMeta(pattern[lastEnd:lastEnd+start])...)
		partName, partExp, _ := strings.Cut(pattern[lastEnd+start+1:lastEnd+start+end], ":")
		lastEnd += start + end + 1

		// TODO: it could support to specify a "capture group" for the name, for example: "/<name[2]:(\d)-(\d)>"
		// it is not used so no need to implement it now
		param := routerPathParam{}
		if partExp == "*" {
			re = append(re, "(.*?)/?"...)
			if lastEnd < len(pattern) && pattern[lastEnd] == '/' {
				lastEnd++ // the "*" pattern is able to handle the last slash, so skip it
			}
		} else {
			partExp = util.IfZero(partExp, "[^/]+")
			re = append(re, '(')
			re = append(re, partExp...)
			re = append(re, ')')
		}
		param.name = partName
		p.params = append(p.params, param)
	}
	re = append(re, '$')
	p.re = regexp.MustCompile(string(re))
	return p
}

func (g *RouterPathGroup) PatternRegexp(pattern string, h ...any) *RouterPathGroupPattern {
	return patternRegexp(pattern, h...)
}
