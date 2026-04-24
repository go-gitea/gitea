// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	gocontext "context"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/pubsub"
	websocket_service "code.gitea.io/gitea/services/websocket"

	gitea_ws "github.com/coder/websocket"
)

// pingInterval is how often the server sends a WebSocket ping to keep the
// connection alive through idle-timeout proxies and load balancers.
const pingInterval = 30 * time.Second

// pingTimeout bounds how long the server waits for a pong response before
// considering the connection dead and closing it.
const pingTimeout = 10 * time.Second

// logoutBrokerMsg is the internal broker message published by PublishLogout.
type logoutBrokerMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

// logoutClientMsg is sent to the WebSocket client so the browser can tell
// whether the logout originated from this tab ("here") or another ("elsewhere").
type logoutClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// rewriteLogout intercepts a broker logout message and rewrites it to the
// client format using "here"/"elsewhere" instead of the raw session ID.
// If sessionID is empty the logout applies to all sessions ("here" for all).
func rewriteLogout(msg []byte, connSessionID string) []byte {
	var lm logoutBrokerMsg
	if err := json.Unmarshal(msg, &lm); err != nil || lm.Type != websocket_service.EventLogout {
		return msg
	}
	where := "elsewhere"
	if lm.SessionID == "" || lm.SessionID == connSessionID {
		where = "here"
	}
	out, err := json.Marshal(logoutClientMsg{Type: websocket_service.EventLogout, Data: where})
	if err != nil {
		return msg
	}
	return out
}

// Serve handles WebSocket upgrade and real-time event delivery.
// Mirrors the former /user/events SSE endpoint: requires a signed-in user and
// rejects unauthenticated requests with 401 (reqSignIn middleware isn't used
// here because it returns a 303 redirect which breaks the WebSocket upgrade).
func Serve(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusUnauthorized)
		return
	}

	routing.MarkLongPolling(ctx.Resp, ctx.Req)

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, &gitea_ws.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // CloseNow is best-effort; error is intentionally ignored

	sessionID := ctx.Session.ID()
	ch, cancel := pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(ctx.Doer.ID))
	defer cancel()

	wsCtx := ctx.Req.Context()
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-wsCtx.Done():
			return
		case <-pingTicker.C:
			pingCtx, cancelPing := gocontext.WithTimeout(wsCtx, pingTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()
			if err != nil {
				log.Trace("websocket: ping failed: %v", err)
				return
			}
		case msg, ok := <-ch:
			if !ok {
				return
			}
			msg = rewriteLogout(msg, sessionID)
			if err := conn.Write(wsCtx, gitea_ws.MessageText, msg); err != nil {
				log.Trace("websocket: write failed: %v", err)
				return
			}
		}
	}
}
