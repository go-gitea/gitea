// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	gocontext "context"
	"net/http"
	"time"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/graceful"
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

func requireUpgrade(ctx *context.Context) bool {
	// Answer plain GETs (health checks, crawlers) here; letting Accept reject them
	// would log an error per request. Same reply it would have sent.
	if ctx.Req.Header.Get("Upgrade") == "" {
		ctx.Resp.Header().Set("Connection", "Upgrade")
		ctx.Resp.Header().Set("Upgrade", "websocket")
		ctx.Resp.WriteHeader(http.StatusUpgradeRequired)
		return false
	}
	return true
}

func Serve(ctx *context.Context) {
	if !requireUpgrade(ctx) {
		return
	}
	if !ctx.IsSigned {
		rejectUnauthenticated(ctx)
		return
	}

	serve(ctx, pubsub.UserTopic(ctx.Doer.ID), ctx.Session.ID(), func(wsCtx gocontext.Context) [][]byte {
		return websocket_service.UserData(wsCtx, ctx.Doer)
	})
}

func ServePullHead(ctx *context.Context) {
	if !requireUpgrade(ctx) {
		return
	}
	pr, err := issues_model.GetPullRequestByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if pr.Issue.IsClosed {
		ctx.NotFound()
		return
	}

	serve(ctx, pubsub.PullHeadTopic(pr.ID), "", func(wsCtx gocontext.Context) [][]byte {
		head, err := ctx.Repo.GitRepo.GetRefCommitID(wsCtx, pr.GetGitHeadRefName())
		if err != nil {
			log.Trace("websocket: read pull request head: %v", err)
			return nil
		}
		return [][]byte{websocket_service.PullHeadEvent(head)}
	})
}

func serve(ctx *context.Context, topic, sessionID string, snapshot func(gocontext.Context) [][]byte) {
	ch, cancel := pubsub.DefaultBroker.Subscribe(topic) // before the snapshot, so no change falls between them
	defer cancel()

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close

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
		writeCtx, cancelWrite := gocontext.WithTimeout(wsCtx, writeTimeout) // a stalled peer must not starve the ping ticker
		defer cancelWrite()
		err := conn.Write(writeCtx, gitea_ws.MessageText, eventDataBytes)
		if err != nil {
			log.Trace("websocket: write failed: %v", err)
		}
		return err
	}

	var backlog [][]byte // published before the snapshot read, so it must not follow the snapshot
	for len(ch) > 0 {
		backlog = append(backlog, <-ch)
	}
	for _, brokerPayload := range append(backlog, snapshot(wsCtx)...) {
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
