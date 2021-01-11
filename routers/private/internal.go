// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"net/http"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/go-chi/binding"
	"gitea.com/macaron/macaron"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tokens := req.Header.Get("Authorization")
		fields := strings.Fields(tokens)
		if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
			log.Debug("Forbidden attempt to access internal url: Authorization header: %s", tokens)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		}
	})
}

// wrap converts an install route to a chi route
func wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx := GetContext(req)
		ctx.Resp = resp
		for _, handler := range handlers {
			switch t := handler.(type) {
			case func(ctx *Context):
				// TODO: if ctx.Written return immediately
				t(ctx)
			case func(resp http.ResponseWriter, req *http.Request):
				t(resp, req)
			}
		}
	})
}

// bind binding an obj to a handler
func bind(obj interface{}, handler func(ctx *Context, form interface{})) http.HandlerFunc {
	var tp = reflect.TypeOf(obj).Elem()
	return wrap(func(ctx *Context) {
		var theObj = reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		handler(ctx, theObj)
	})
}

// RegisterRoutes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func RegisterRoutes(m *macaron.Macaron) {
	m.Group("/", func() {
		m.Post("/ssh/authorized_keys", AuthorizedPublicKeyByContent)
		m.Post("/ssh/:id/update/:repoid", UpdatePublicKeyInRepo)
		m.Post("/hook/pre-receive/:owner/:repo", bind(&private.HookOptions{}, HookPreReceive))
		m.Post("/hook/post-receive/:owner/:repo", bind(&private.HookOptions{}, HookPostReceive))
		m.Post("/hook/set-default-branch/:owner/:repo/:branch", SetDefaultBranch)
		m.Get("/serv/none/:keyid", ServNoCommand)
		m.Get("/serv/command/:keyid/:owner/:repo", ServCommand)
		m.Post("/manager/shutdown", Shutdown)
		m.Post("/manager/restart", Restart)
		m.Post("/manager/flush-queues", bind(&private.FlushOptions{}, FlushQueues))
		m.Post("/manager/pause-logging", PauseLogging)
		m.Post("/manager/resume-logging", ResumeLogging)
		m.Post("/manager/release-and-reopen-logging", ReleaseReopenLogging)
		m.Post("/manager/add-logger", bind(&private.LoggerOptions{}, AddLogger))
		m.Post("/manager/remove-logger/:group/:name", RemoveLogger)
		m.Post("/mail/send", SendEmail)
	}, CheckInternalToken)
}
