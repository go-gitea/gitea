// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"regexp"
	"slices"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/util"

	"github.com/go-chi/chi/v5"
)

type RouterPathGroup struct {
	r         *Router
	pathParam string
	matchers  []*routerPathMatcher

	curGroupPrefix string
	curMiddlewares []any
}

func (g *RouterPathGroup) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	matcher := g.findMatcher(req)
	if matcher != nil {
		g.serveMatch(resp, req, matcher)
		return
	}
	g.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
}

func (g *RouterPathGroup) findMatcher(req *http.Request) *routerPathMatcher {
	chiCtx := chi.RouteContext(req.Context())
	path := chiCtx.URLParam(g.pathParam)
	for _, m := range g.matchers {
		if m.matchPath(chiCtx, path) {
			return m
		}
	}
	return nil
}

func (g *RouterPathGroup) findMatchingMatcher(req *http.Request) *routerPathMatcher {
	chiCtx := chi.RouteContext(req.Context())
	path := chiCtx.URLParam(g.pathParam)
	for _, m := range g.matchers {
		if m.matchesPath(chiCtx.RouteMethod, path) {
			return m
		}
	}
	return nil
}

func (g *RouterPathGroup) hasPathMatch(req *http.Request) bool {
	chiCtx := chi.RouteContext(req.Context())
	path := chiCtx.URLParam(g.pathParam)
	for _, m := range g.matchers {
		if m.matchesRoutePath(path) {
			return true
		}
	}
	return false
}

func (g *RouterPathGroup) serveMatch(resp http.ResponseWriter, req *http.Request, matcher *routerPathMatcher) {
	chiCtx := chi.RouteContext(req.Context())
	chiCtx.RoutePatterns = append(chiCtx.RoutePatterns, matcher.pattern)
	executeMiddlewaresHandler(resp, req, matcher.middlewares, matcher.handlerFunc)
}

func (g *RouterPathGroup) markMatchedRoutePattern(req *http.Request, matcher *routerPathMatcher) {
	chiCtx := chi.RouteContext(req.Context())
	chiCtx.RoutePatterns = append(chiCtx.RoutePatterns, matcher.pattern)
}

type RouterPathGroupPattern struct {
	pattern     string
	re          *regexp.Regexp
	params      []routerPathParam
	middlewares []any

	literalChars   int
	wildcardParams int
	totalParams    int
}

// MatchPath matches the request method, and uses regexp to match the path.
// The pattern uses "<...>" to define path parameters, for example, "/<name>" (different from chi router)
// It is only designed to resolve some special cases that chi router can't handle.
// For most cases, it shouldn't be used because it needs to iterate all rules to find the matched one (inefficient).
func (g *RouterPathGroup) MatchPath(methods, pattern string, h ...any) {
	g.MatchPattern(methods, g.PatternRegexp(pattern), h...)
}

func (g *RouterPathGroup) MatchPattern(methods string, pattern *RouterPathGroupPattern, h ...any) {
	g.addMatcher(newRouterPathMatcher(methods, pattern, g.curMiddlewares, h...))
}

func (g *RouterPathGroup) addMatcher(matcher *routerPathMatcher) {
	for i, existing := range g.matchers {
		if matcher.moreSpecificThan(existing) {
			g.matchers = append(g.matchers, nil)
			copy(g.matchers[i+1:], g.matchers[i:])
			g.matchers[i] = matcher
			return
		}
	}
	g.matchers = append(g.matchers, matcher)
}

// Group creates a path matcher sub-group along a "pattern" string.
func (g *RouterPathGroup) Group(pattern string, fn func(), middlewares ...any) {
	previousGroupPrefix := g.curGroupPrefix
	previousMiddlewares := g.curMiddlewares
	g.curGroupPrefix = joinPattern(g.curGroupPrefix, pattern)
	g.curMiddlewares = append(g.curMiddlewares, middlewares...)

	fn()

	g.curGroupPrefix = previousGroupPrefix
	g.curMiddlewares = previousMiddlewares
}

func (g *RouterPathGroup) Head(pattern string, h ...any) {
	g.MatchPath("HEAD", pattern, h...)
}

func (g *RouterPathGroup) Get(pattern string, h ...any) {
	g.MatchPath("GET", pattern, h...)
}

func (g *RouterPathGroup) Post(pattern string, h ...any) {
	g.MatchPath("POST", pattern, h...)
}

func (g *RouterPathGroup) Put(pattern string, h ...any) {
	g.MatchPath("PUT", pattern, h...)
}

func (g *RouterPathGroup) Patch(pattern string, h ...any) {
	g.MatchPath("PATCH", pattern, h...)
}

func (g *RouterPathGroup) Delete(pattern string, h ...any) {
	g.MatchPath("DELETE", pattern, h...)
}

func (g *RouterPathGroup) Methods(methods, pattern string, h ...any) {
	g.MatchPath(methods, pattern, h...)
}

func (g *RouterPathGroup) getPattern(pattern string) string {
	newPattern := joinPattern(g.curGroupPrefix, pattern)
	if !strings.HasPrefix(newPattern, "/") {
		newPattern = "/" + newPattern
	}
	if newPattern == "/" {
		return newPattern
	}
	return strings.TrimSuffix(newPattern, "/")
}

func (g *RouterPathGroup) Combo(pattern string, h ...any) ICombo {
	return &RouterPathGroupCombo{
		r:       g,
		pattern: pattern,
		h:       h,
	}
}

type RouterPathGroupCombo struct {
	r       *RouterPathGroup
	pattern string
	h       []any
}

func (c *RouterPathGroupCombo) Get(h ...any) ICombo {
	c.r.Get(c.pattern, append(c.h, h...)...)
	return c
}

func (c *RouterPathGroupCombo) Post(h ...any) ICombo {
	c.r.Post(c.pattern, append(c.h, h...)...)
	return c
}

func (c *RouterPathGroupCombo) Patch(h ...any) ICombo {
	c.r.Patch(c.pattern, append(c.h, h...)...)
	return c
}

func (c *RouterPathGroupCombo) Put(h ...any) ICombo {
	c.r.Put(c.pattern, append(c.h, h...)...)
	return c
}

func (c *RouterPathGroupCombo) Delete(h ...any) ICombo {
	c.r.Delete(c.pattern, append(c.h, h...)...)
	return c
}

type routerPathParam struct {
	name         string
	pathSepEnd   bool
	captureGroup int
}

type routerPathMatcher struct {
	methods        container.Set[string]
	pattern        string
	re             *regexp.Regexp
	params         []routerPathParam
	preMiddlewares []middlewareProvider
	middlewares    []middlewareProvider
	handlerFunc    http.HandlerFunc

	literalChars   int
	wildcardParams int
	totalParams    int
}

func (p *routerPathMatcher) moreSpecificThan(other *routerPathMatcher) bool {
	switch {
	case p.literalChars != other.literalChars:
		return p.literalChars > other.literalChars
	case p.wildcardParams != other.wildcardParams:
		return p.wildcardParams < other.wildcardParams
	case p.totalParams != other.totalParams:
		return p.totalParams < other.totalParams
	default:
		return len(p.pattern) > len(other.pattern)
	}
}

func (p *routerPathMatcher) matchesPath(method, path string) bool {
	return p.methods.Contains(method) && p.matchesRoutePath(path)
}

func (p *routerPathMatcher) matchesRoutePath(path string) bool {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return p.re.MatchString(path)
}

func (p *routerPathMatcher) matchPath(chiCtx *chi.Context, path string) bool {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !p.methods.Contains(chiCtx.RouteMethod) {
		return false
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
		if pm[groupIdx] == -1 || pm[groupIdx+1] == -1 {
			chiCtx.URLParams.Add(p.params[i].name, "")
			continue
		}
		val := path[pm[groupIdx]:pm[groupIdx+1]]
		if p.params[i].pathSepEnd {
			val = strings.TrimSuffix(val, "/")
		}
		chiCtx.URLParams.Add(p.params[i].name, val)
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

func newRouterPathMatcher(methods string, patternRegexp *RouterPathGroupPattern, curMiddlewares []any, h ...any) *routerPathMatcher {
	allMiddlewares := slices.Clone(curMiddlewares)
	allMiddlewares = append(allMiddlewares, patternRegexp.middlewares...)
	preMiddlewares, normalMiddlewares, hasPreMiddlewares := wrapMiddlewaresSplit(nil, allMiddlewares, h)
	middlewares := normalMiddlewares[:len(normalMiddlewares)-1]
	handlerFunc := normalMiddlewares[len(normalMiddlewares)-1](nil).ServeHTTP
	if mockPoint := RouterMockPoint(MockAfterMiddlewares); mockPoint != nil {
		middlewares = append(middlewares, mockPoint)
	}
	p := &routerPathMatcher{
		methods:        make(container.Set[string]),
		preMiddlewares: util.Iif(hasPreMiddlewares, preMiddlewares, nil),
		middlewares:    middlewares,
		handlerFunc:    handlerFunc,
	}
	for method := range strings.SplitSeq(methods, ",") {
		method = strings.TrimSpace(method)
		if !isValidMethod(method) {
			panic("invalid HTTP method: " + method)
		}
		p.methods.Add(method)
	}
	p.pattern, p.re, p.params = patternRegexp.pattern, patternRegexp.re, patternRegexp.params
	p.literalChars = patternRegexp.literalChars
	p.wildcardParams = patternRegexp.wildcardParams
	p.totalParams = patternRegexp.totalParams
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
		p.literalChars += start
		lastEnd += start + end + 1

		// TODO: it could support to specify a "capture group" for the name, for example: "/<name[2]:(\d)-(\d)>"
		// it is not used so no need to implement it now
		param := routerPathParam{}
		p.totalParams++
		if partExp == "*" {
			// "<part:*>" is a shorthand for optionally matching any string (but not greedy)
			partExp = ".*?"
			p.wildcardParams++
			if lastEnd < len(pattern) && pattern[lastEnd] == '/' {
				// if this param part ends with path separator "/", then consider it together: "(.*?/)"
				partExp += "/"
				param.pathSepEnd = true
				lastEnd++
			}
			re = append(re, '(')
			re = append(re, partExp...)
			re = append(re, ')', '?') // the wildcard matching is optional
		} else {
			// the pattern is user-provided regexp, defaults to a path part (separated by "/")
			partExp = util.IfZero(partExp, "[^/]+")
			re = append(re, '(')
			re = append(re, partExp...)
			re = append(re, ')')
		}
		param.name = partName
		p.params = append(p.params, param)
	}
	p.literalChars += len(pattern) - lastEnd
	re = append(re, '$')
	p.pattern, p.re = pattern, regexp.MustCompile(string(re))
	return p
}

func (g *RouterPathGroup) PatternRegexp(pattern string, h ...any) *RouterPathGroupPattern {
	return patternRegexp(g.getPattern(pattern), h...)
}

type routerPathGroupEntry struct {
	group             *RouterPathGroup
	usePreMiddlewares []middlewareProvider
	useMiddlewares    []middlewareProvider
	preMiddlewares    []middlewareProvider
	middlewares       []middlewareProvider
}

type routerPathGroupsHandler struct {
	r       *Router
	entries []*routerPathGroupEntry
}

func (h *routerPathGroupsHandler) Add(useMiddlewares, curMiddlewares, routeMiddlewares []any, group *RouterPathGroup) {
	usePreMiddlewares, useNormalMiddlewares, _ := wrapMiddlewaresSplit(nil, nil, useMiddlewares)
	preMiddlewares, middlewares, _ := wrapMiddlewaresSplit(nil, curMiddlewares, routeMiddlewares)
	h.entries = append(h.entries, &routerPathGroupEntry{
		group:             group,
		usePreMiddlewares: usePreMiddlewares,
		useMiddlewares:    useNormalMiddlewares,
		preMiddlewares:    preMiddlewares,
		middlewares:       middlewares,
	})
}

func (h *routerPathGroupsHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if len(h.entries) == 1 {
		entry := h.entries[0]
		executeMiddlewaresHandler(resp, req, entry.usePreMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
			executeMiddlewaresHandler(resp, req, entry.preMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
				matcher := entry.group.findMatchingMatcher(req)
				if matcher == nil {
					executeMiddlewaresHandler(resp, req, entry.useMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
						executeMiddlewaresHandler(resp, req, entry.middlewares, func(resp http.ResponseWriter, req *http.Request) {
							if entry.group.hasPathMatch(req) {
								h.r.chiRouter.MethodNotAllowedHandler().ServeHTTP(resp, req)
							} else {
								h.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
							}
						})
					})
					return
				}

				executeMiddlewaresHandler(resp, req, matcher.preMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
					resolvedMatcher := entry.group.findMatcher(req)
					if resolvedMatcher == nil {
						h.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
						return
					}
					entry.group.markMatchedRoutePattern(req, resolvedMatcher)
					executeMiddlewaresHandler(resp, req, entry.useMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
						executeMiddlewaresHandler(resp, req, entry.middlewares, func(resp http.ResponseWriter, req *http.Request) {
							executeMiddlewaresHandler(resp, req, resolvedMatcher.middlewares, resolvedMatcher.handlerFunc)
						})
					})
				})
			})
		})
		return
	}

	var matchedEntry *routerPathGroupEntry
	var matchedMatcher *routerPathMatcher
	hasPathMatch := false
	for _, entry := range h.entries {
		hasPathMatch = hasPathMatch || entry.group.hasPathMatch(req)
		matcher := entry.group.findMatchingMatcher(req)
		if matcher == nil {
			continue
		}
		if matchedMatcher == nil || matcher.moreSpecificThan(matchedMatcher) {
			matchedEntry = entry
			matchedMatcher = matcher
		}
	}
	if matchedEntry != nil {
		executeMiddlewaresHandler(resp, req, matchedEntry.usePreMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
			executeMiddlewaresHandler(resp, req, matchedEntry.preMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
				executeMiddlewaresHandler(resp, req, matchedMatcher.preMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
					resolvedMatcher := matchedEntry.group.findMatcher(req)
					if resolvedMatcher == nil {
						h.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
						return
					}
					matchedEntry.group.markMatchedRoutePattern(req, resolvedMatcher)
					executeMiddlewaresHandler(resp, req, matchedEntry.useMiddlewares, func(resp http.ResponseWriter, req *http.Request) {
						executeMiddlewaresHandler(resp, req, matchedEntry.middlewares, func(resp http.ResponseWriter, req *http.Request) {
							executeMiddlewaresHandler(resp, req, resolvedMatcher.middlewares, resolvedMatcher.handlerFunc)
						})
					})
				})
			})
		})
		return
	}
	if hasPathMatch {
		h.r.chiRouter.MethodNotAllowedHandler().ServeHTTP(resp, req)
		return
	}
	h.r.chiRouter.NotFoundHandler().ServeHTTP(resp, req)
}
