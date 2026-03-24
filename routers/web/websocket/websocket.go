// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"net/http"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/pubsub"

	gitea_ws "github.com/coder/websocket"
)

// Serve handles WebSocket upgrade and event delivery for the signed-in user.
func Serve(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.Status(http.StatusUnauthorized)
		return
	}
	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, &gitea_ws.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // CloseNow is best-effort; error is intentionally ignored

	ch, cancel := pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(ctx.Doer.ID))
	defer cancel()

	wsCtx := ctx.Req.Context()
	for {
		select {
		case <-wsCtx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.Write(wsCtx, gitea_ws.MessageText, msg); err != nil {
				log.Trace("websocket: write failed: %v", err)
				return
			}
		}
	}
}
