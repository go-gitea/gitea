// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"time"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/websocket"

	"github.com/olahol/melody"
)

var m *melody.Melody

func Init(r *web.Route) {
	m = melody.New()
	r.Any("/-/ws", webSocket)
	m.HandleConnect(websocket.HandleConnect)
	m.HandleMessage(websocket.HandleMessage)
	m.HandleDisconnect(websocket.HandleDisconnect)

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

func webSocket(ctx *context.Context) {
	err := m.HandleRequest(ctx.Resp, ctx.Req)
	if err != nil {
		ctx.ServerError("HandleRequest", err)
	}
}
