// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	gocontext "context"
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
// Anonymous connections are accepted and kept open; user-specific events
// (notification count, stopwatch, logout) are only delivered to signed-in
// users. This allows future public event types to reuse the same endpoint
// without requiring authentication.
func Serve(ctx *context.Context) {
	routing.MarkLongPolling(ctx.Resp, ctx.Req)

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, &gitea_ws.AcceptOptions{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // CloseNow is best-effort; error is intentionally ignored

	// Subscribe to user-specific events only for signed-in users.
	// ch is nil for anonymous users: the receive case below never fires,
	// keeping the connection open for future public event types.
	var ch <-chan []byte
	var sessionID string
	if ctx.IsSigned {
		sessionID = ctx.Session.ID()
		var cancel func()
		ch, cancel = pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(ctx.Doer.ID))
		defer cancel()
	}

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
