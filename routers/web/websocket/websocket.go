// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/websocket"
)

func Init(r *web.Router) {
	m := websocket.Init()

	r.Any("/-/ws", func(ctx *context.Context) {
		err := m.HandleRequest(ctx.Resp, ctx.Req)
		if err != nil {
			ctx.ServerError("HandleRequest", err)
			return
		}
	})
}
