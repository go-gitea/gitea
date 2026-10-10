// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	gocontext "context"
	"net/http"
	"time"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/services/context"
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

func Serve(ctx *context.Context) {
	// Answer plain GETs (health checks, crawlers) here; letting Accept reject them
	// would log an error per request. Same reply it would have sent.
	if ctx.Req.Header.Get("Upgrade") == "" {
		ctx.Resp.Header().Set("Connection", "Upgrade")
		ctx.Resp.Header().Set("Upgrade", "websocket")
		ctx.Resp.WriteHeader(http.StatusUpgradeRequired)
		return
	}

	if !ctx.IsSigned {
		rejectUnauthenticated(ctx)
		return
	}

	// subscribe before the handshake, so no event can fire before we are listening
	ch, cancel := websocket_service.SubscribeUser(ctx.Doer.ID)
	defer cancel()

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close

	sessionID := ctx.Session.ID()

	// Ping requires a concurrent reader to observe the pong frame; CloseRead
	// spawns one and cancels its context when the peer goes away.
	wsCtx := conn.CloseRead(ctx.Req.Context())
	shutdownCtx := graceful.GetManager().ShutdownContext()
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	send := func(brokerPayload []byte) error {
		eventDataBytes := websocket_service.EventForSession(brokerPayload, sessionID)
		if eventDataBytes == nil {
			return nil
		}
		// bound the write so a stalled peer can't starve the ping ticker
		writeCtx, cancelWrite := gocontext.WithTimeout(wsCtx, writeTimeout)
		defer cancelWrite()
		err := conn.Write(writeCtx, gitea_ws.MessageText, eventDataBytes)
		if err != nil {
			log.Trace("websocket: write failed: %v", err)
		}
		return err
	}

	for _, brokerPayload := range websocket_service.UserData(ctx, ctx.Doer) {
		if err := send(brokerPayload); err != nil {
			return
		}
	}

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
			if err := send(brokerPayload); err != nil {
				return
			}
		}
	}
}

// rejectUnauthenticated upgrades only to close, so clients stop reconnecting.
func rejectUnauthenticated(ctx *context.Context) {
	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	_ = conn.Close(closeCodeUnauthenticated, "unauthenticated")
}
