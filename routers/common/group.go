// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"regexp"

	"gitea.dev/modules/web"
)

var repoRouteRegex = regexp.MustCompile(`(.*?)(/\{(?:reponame|repo)})(.*?)$`)

func RegisterRepoRouteGroup(m *web.Router, pattern string, groupMiddleware any, fn func(), middlewares ...any) {
	var groupMiddlewares []any

	if groupMiddleware != nil {
		groupMiddlewares = make([]any, len(middlewares)+1)

		groupMiddlewares[0] = groupMiddleware
		for i := range middlewares {
			groupMiddlewares[i+1] = middlewares[i]
		}
	} else {
		groupMiddlewares = middlewares
	}
	asGroupRoute := repoRouteRegex.ReplaceAllString(pattern, "$1/group/{group_id}$2$3")

	m.Group(asGroupRoute, fn, groupMiddlewares...)
	m.Group(pattern, fn, middlewares...)
}
