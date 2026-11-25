// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"bytes"
	gocontext "context"
	"net/http"
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

	maxTopicSubscriptions = 64

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

	ch, cancel := pubsub.DefaultBroker.Subscribe(pubsub.UserTopic(ctx.Doer.ID)) // before the snapshot, so no change falls between them
	defer cancel()

	conn, err := gitea_ws.Accept(ctx.Resp, ctx.Req, nil)
	if err != nil {
		log.Error("websocket: accept failed: %v", err)
		return
	}
	defer conn.CloseNow() //nolint:errcheck // best-effort close

	sessionID := ctx.Session.ID()

	wsCtx, cancelConnection := gocontext.WithCancel(ctx.Req.Context())
	defer cancelConnection()
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

	go readClientMessages(wsCtx, cancelConnection, conn, ctx.Doer.ID, !ctx.DoerNeedTwoFactorAuth(), send) // also the concurrent reader Ping needs to observe pongs

	var backlog [][]byte // published before the snapshot read, so it must not follow the snapshot
	for len(ch) > 0 {
		backlog = append(backlog, <-ch)
	}
	for _, brokerPayload := range append(backlog, websocket_service.UserData(wsCtx, ctx.Doer)...) {
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

type pullHeadMessage struct { // keep in sync with web_src/js/features/repo-diff.ts
	PullID int64 `json:"pullID"`
}

func readClientMessages(ctx gocontext.Context, cancelConnection gocontext.CancelFunc, conn *gitea_ws.Conn, doerID int64, canSubscribe bool, send func([]byte) error) {
	defer cancelConnection()
	subscriptions := map[string]gocontext.CancelFunc{}
	for {
		messageType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if !canSubscribe || messageType != gitea_ws.MessageText {
			continue
		}
		msgType, msgJSON, _ := bytes.Cut(data, []byte("\n"))
		switch string(msgType) {
		case "subscribe-pull-head", "unsubscribe-pull-head":
			var msg pullHeadMessage
			if json.Unmarshal(msgJSON, &msg) != nil {
				continue
			}
			topic := websocket_service.PullHeadTopic(msg.PullID)
			if cancel := subscriptions[topic]; cancel != nil {
				cancel()
				delete(subscriptions, topic)
			}
			if string(msgType) == "subscribe-pull-head" && len(subscriptions) < maxTopicSubscriptions { // a restarted stream sends a joining tab the current event
				topicCtx, cancel := gocontext.WithCancel(ctx)
				subscriptions[topic] = cancel
				go streamTopic(topicCtx, cancelConnection, topic, func(ctx gocontext.Context) []byte { return websocket_service.PullHeadEvent(ctx, doerID, msg.PullID) }, send)
			}
		}
	}
}

func streamTopic(ctx gocontext.Context, cancelConnection gocontext.CancelFunc, topic string, currentEvent func(gocontext.Context) []byte, send func([]byte) error) {
	ch, unsubscribe := pubsub.DefaultBroker.Subscribe(topic)
	defer unsubscribe()
	for {
		if send(currentEvent(ctx)) != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				cancelConnection()
				return
			}
		}
	}
}
