// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	activities_model "gitea.dev/models/activities"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/pubsub"
	pull_service "gitea.dev/services/pull"
	websocket_service "gitea.dev/services/websocket"
	"gitea.dev/tests"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staleBacklogBroker struct {
	pubsub.Broker
}

func (b *staleBacklogBroker) Subscribe(topic string) (<-chan []byte, func()) {
	ch, cancel := b.Broker.Subscribe(topic)
	for range 9 {
		b.Publish(topic, countEvent(-1))
	}
	return ch, cancel
}

func countEvent(count int) []byte {
	return fmt.Appendf(nil, "\n"+`{"eventType":"notification-count","eventData":{"count":%d}}`, count)
}

func dialWebSocket(t *testing.T, userName string) (*websocket.Conn, func()) {
	session := loginUser(t, userName)
	server := httptest.NewServer(testWebRoutes)
	header := http.Header{"Cookie": {session.GetRawCookie(setting.SessionConfig.CookieName).String()}}
	conn, _, err := websocket.Dial(t.Context(), server.URL+"/-/ws", &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the body, it is nil after the upgrade
	require.NoError(t, err)
	return conn, func() {
		_ = conn.CloseNow()
		server.Close()
	}
}

func TestWebSocketSnapshotFollowsBacklog(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&pubsub.DefaultBroker, pubsub.Broker(&staleBacklogBroker{pubsub.NewMemoryBroker()}))()
	conn, closeConn := dialWebSocket(t, "user2")
	defer closeConn()

	const sentinelCount = -2
	lastCount := 0
	for {
		_, payload, err := conn.Read(t.Context())
		require.NoError(t, err)
		var event struct {
			EventType string     `json:"eventType"`
			EventData json.Value `json:"eventData"`
		}
		require.NoError(t, json.Unmarshal(payload, &event))
		if event.EventType == "stopwatches" {
			pubsub.DefaultBroker.Publish(pubsub.UserTopic(2), countEvent(sentinelCount))
			continue
		}
		var data struct {
			Count int `json:"count"`
		}
		require.NoError(t, json.Unmarshal(event.EventData, &data))
		if data.Count == sentinelCount {
			break
		}
		lastCount = data.Count
	}
	assert.Equal(t, unittest.GetCount(t, &activities_model.Notification{UserID: 2, Status: activities_model.NotificationStatusUnread}), lastCount)
}

func TestWebSocketPullHeadSubscription(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&pubsub.DefaultBroker, pubsub.Broker(pubsub.NewMemoryBroker()))()
	require.NoError(t, pull_service.PushToBaseRepo(t.Context(), unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 6})))
	_, readableEvent := websocket_service.Subscription("pull-head", 2, []byte(`{"pullID":6}`))
	_, deniedEvent := websocket_service.Subscription("pull-head", 5, []byte(`{"pullID":6}`))
	assert.NotNil(t, readableEvent(t.Context()))
	assert.Nil(t, deniedEvent(t.Context()))

	conn, closeConn := dialWebSocket(t, "user5")
	defer closeConn()
	subscribe := []byte("subscribe-pull-head\n" + `{"pullID":2}`)
	readPullHead := func() {
		for {
			_, payload, err := conn.Read(t.Context())
			require.NoError(t, err)
			if strings.HasPrefix(string(payload), `{"eventType":"pull-head","eventData":{"pullID":2,"head":`) {
				return
			}
		}
	}
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, subscribe))
	readPullHead()
	require.NoError(t, pull_service.PushToBaseRepo(t.Context(), unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})))
	readPullHead()
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, subscribe))
	readPullHead()
}
