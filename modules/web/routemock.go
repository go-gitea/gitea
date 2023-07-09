// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"
)

// MockAfterMiddlewares is a general mock point, it's between middlewares and the handler
const MockAfterMiddlewares = "MockAfterMiddlewares"

var routeMockPoints = map[string]func(next http.Handler) http.Handler{}

// RouteMockPoint registers a mock point as a middleware for testing, example:
//
//	r.Use(web.RouteMockPoint("my-mock-point-1"))
//	r.Get("/foo", middleware2, web.RouteMockPoint("my-mock-point-2"), middleware2, handler)
//
// Then use web.RouteMock to mock the route execution.
// It only takes effect in testing mode (setting.IsInTesting == true).
func RouteMockPoint(pointName string) func(next http.Handler) http.Handler {
	if !setting.IsInTesting {
		return nil
	}
	routeMockPoints[pointName] = nil
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h := routeMockPoints[pointName]; h != nil {
				h(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// RouteMock uses the registered mock point to mock the route execution, example:
//
//	defer web.RouteMockReset()
//	web.RouteMock(web.MockAfterMiddlewares, func(ctx *context.Context) {
//		ctx.WriteResponse(...)
//	}
//
// Then the mock function will be executed as a middleware at the mock point.
// It only takes effect in testing mode (setting.IsInTesting == true).
func RouteMock(pointName string, h any) {
	if _, ok := routeMockPoints[pointName]; !ok {
		panic("route mock point not found: " + pointName)
	}
	routeMockPoints[pointName] = toHandlerProvider(h)
}

// RouteMockReset resets all mock points (no mock anymore)
func RouteMockReset() {
	for k := range routeMockPoints {
		routeMockPoints[k] = nil // keep the keys because RouteMock will check the keys to make sure no misspelling
	}
}
