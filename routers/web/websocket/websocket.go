// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
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

// filterLogout forwards a session-free logout only to the targeted connection
// (its own session, or every session when SessionID is empty) and drops it for
// the rest. Non-logout messages pass through untouched.
func filterLogout(eventType string, eventDataBytes []byte, connSessionID string) []byte {
	if eventType != websocket_service.EventLogout {
		return eventDataBytes
	}
	var lm websocket_service.UserEventMessage[websocket_service.LogoutEventData]
	if err := json.Unmarshal(eventDataBytes, &lm); err != nil {
		return eventDataBytes
	}
	if lm.EventData.SessionID == "" || lm.EventData.SessionID == connSessionID {
		return []byte(`{"eventType":"logout"}`)
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
		case brokerPayload, ok := <-ch:
			if !ok {
				return
			}
			eventType, eventDataBytes := websocket_service.ExtractUserEventMessage(brokerPayload)
			eventDataBytes = filterLogout(eventType, eventDataBytes, sessionID)
			if eventDataBytes == nil {
				continue
			}
			// Bound the write so a stalled/slow peer can't block this goroutine
			// indefinitely and starve the ping ticker.
			writeCtx, cancelWrite := gocontext.WithTimeout(wsCtx, writeTimeout)
			err := conn.Write(writeCtx, gitea_ws.MessageText, eventDataBytes)
			cancelWrite()
			if err != nil {
				log.Trace("websocket: write failed: %v", err)
				return
			}
		}
	}
}
