// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/pubsub"

	gitea_ws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Serve handles WebSocket upgrade and event delivery for the signed-in user.
func Serve(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.Status(401)
		return
	}

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, &gitea_ws.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck

	topic := fmt.Sprintf("user-%d", ctx.Doer.ID)
	ch, cancel := pubsub.DefaultBroker.Subscribe(topic)
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
			if err := wsjson.Write(wsCtx, conn, json.RawMessage(msg)); err != nil {
				log.Trace("websocket: write failed: %v", err)
				return
			}
		}
	}
}
