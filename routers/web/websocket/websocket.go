// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	notify_service "code.gitea.io/gitea/services/notify"
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
	notify_service.RegisterNotifier(websocket.NewNotifier(m))
}

func webSocket(ctx *context.Context) {
	err := m.HandleRequest(ctx.Resp, ctx.Req)
	if err != nil {
		ctx.ServerError("HandleRequest", err)
	}
}
