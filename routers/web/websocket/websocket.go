// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	gocontext "context"
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/services/context"
	"gitea.dev/services/pubsub"
	websocket_service "gitea.dev/services/websocket"

	gitea_ws "github.com/coder/websocket"
)

const (
	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
	writeTimeout = 10 * time.Second

	// First code in the IANA library/framework reserved range (3000–3999).
	// Sentinel for an unauthenticated session so the SharedWorker can tell
	// "your cookie is gone" apart from a transient network failure and stop
	// reconnecting in a tight loop.
	closeCodeUnauthenticated gitea_ws.StatusCode = 3000
)

var logoutTypeMarker = []byte(`"type":"logout"`)

// Bare client payload; the broker's session ID is never serialized to the browser.
var logoutClientPayload = []byte(`{"type":"logout"}`)

// filterLogout forwards a session-free logout only to the targeted connection
// (its own session, or every session when SessionID is empty) and drops it for
// the rest. Non-logout messages pass through untouched.
func filterLogout(msg []byte, connSessionID string) []byte {
	if !bytes.Contains(msg, logoutTypeMarker) {
		return msg
	}
	var lm websocket_service.LogoutBrokerMsg
	if err := json.Unmarshal(msg, &lm); err != nil || lm.Type != websocket_service.EventLogout {
		return msg
	}
	if lm.SessionID == "" || lm.SessionID == connSessionID {
		return logoutClientPayload
	}
	return nil
}

func Serve(ctx *context.Context) {
	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close

	if !ctx.IsSigned {
		_ = conn.Close(closeCodeUnauthenticated, "unauthenticated")
		return
	}

	sessionID := ctx.Session.ID()
	ch, cancel := pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(ctx.Doer.ID))
	defer cancel()

	// Ping requires a concurrent reader to observe the pong frame; CloseRead
	// spawns one and cancels its context when the peer goes away.
	wsCtx := conn.CloseRead(ctx.Req.Context())
	shutdownCtx := graceful.GetManager().ShutdownContext()
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case <-wsCtx.Done():
			return
		case <-shutdownCtx.Done():
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
			msg = filterLogout(msg, sessionID)
			if msg == nil {
				continue
			}
			// Bound the write so a stalled/slow peer can't block this goroutine
			// indefinitely and starve the ping ticker.
			writeCtx, cancelWrite := gocontext.WithTimeout(wsCtx, writeTimeout)
			err := conn.Write(writeCtx, gitea_ws.MessageText, msg)
			cancelWrite()
			if err != nil {
				log.Trace("websocket: write failed: %v", err)
				return
			}
		}
	}
}
