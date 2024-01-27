// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"

	"github.com/olahol/melody"
)

var m *melody.Melody

func Init(r *web.Route) {
	m = melody.New()
	r.Any("/ui-updates", WebSocket)
	m.HandleMessage(HandleMessage)

	go func() {
		for {
			// TODO: send proper updated html
			err := m.Broadcast([]byte("<div hx-swap-oob=\"beforebegin:.timeline-item.comment.form\"><div class=\"hello\">hello world!</div></div>"))
			if err != nil {
				break
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func WebSocket(ctx *context.Context) {
	err := m.HandleRequest(ctx.Resp, ctx.Req)
	if err != nil {
		ctx.ServerError("HandleRequest", err)
	}
}

func HandleMessage(s *melody.Session, msg []byte) {
	// TODO: Handle incoming messages
}
