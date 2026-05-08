// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	gocontext "context"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/pubsub"
	websocket_service "code.gitea.io/gitea/services/websocket"

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

type logoutBrokerMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID,omitempty"`
}

type logoutClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// logoutPrefix lets us skip the full JSON Unmarshal for every non-logout event.
var logoutPrefix = []byte(`{"type":"` + websocket_service.EventLogout + `"`)

// Translates the raw session ID into "here"/"elsewhere" so the client can tell
// whether logout originated from this tab. Empty sessionID targets all sessions.
func rewriteLogout(msg []byte, connSessionID string) []byte {
	if !bytes.HasPrefix(msg, logoutPrefix) {
		return msg
	}
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
			msg = rewriteLogout(msg, sessionID)
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
