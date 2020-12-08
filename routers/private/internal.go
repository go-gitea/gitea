// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/binding"
	"gitea.com/macaron/macaron"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(ctx *macaron.Context) {
	tokens := ctx.Req.Header.Get("Authorization")
	fields := strings.Fields(tokens)
	if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
		log.Debug("Forbidden attempt to access internal url: Authorization header: %s", tokens)
		ctx.Error(403)
	}
}

// RegisterRoutes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func RegisterRoutes(m *macaron.Macaron) {
	bind := binding.Bind

	m.Group("/", func() {
		m.Post("/ssh/authorized_keys", AuthorizedPublicKeyByContent)
		m.Post("/ssh/:id/update/:repoid", UpdatePublicKeyInRepo)
		m.Post("/hook/pre-receive/:owner/:repo", bind(private.HookOptions{}), HookPreReceive)
		m.Post("/hook/post-receive/:owner/:repo", bind(private.HookOptions{}), HookPostReceive)
		m.Post("/hook/set-default-branch/:owner/:repo/:branch", SetDefaultBranch)
		m.Get("/serv/none/:keyid", ServNoCommand)
		m.Get("/serv/command/:keyid/:owner/:repo", ServCommand)
		m.Post("/manager/shutdown", Shutdown)
		m.Post("/manager/restart", Restart)
		m.Post("/manager/flush-queues", bind(private.FlushOptions{}), FlushQueues)
		m.Post("/manager/pause-logging", PauseLogging)
		m.Post("/manager/resume-logging", ResumeLogging)
		m.Post("/manager/release-and-reopen-logging", ReleaseReopenLogging)
		m.Post("/manager/add-logger", bind(private.LoggerOptions{}), AddLogger)
		m.Post("/manager/remove-logger/:group/:name", RemoveLogger)
		m.Post("/mail/send", SendEmail)
	}, CheckInternalToken)
}
