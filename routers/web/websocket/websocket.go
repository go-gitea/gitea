// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"

	"github.com/olahol/melody"
)

var m *melody.Melody

func Init(r *web.Route) {
	m = melody.New()
	r.Any("/ws", WebSocket)
	m.HandleMessage(HandleMessage)
}

func WebSocket(ctx *context.Context) {
	m.HandleRequest(ctx.Resp, ctx.Req)
}

func HandleMessage(s *melody.Session, msg []byte) {
	m.Broadcast(msg)
}
