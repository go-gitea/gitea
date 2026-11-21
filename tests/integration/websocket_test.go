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
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/pubsub"
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

func TestWebSocketSnapshotFollowsBacklog(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&pubsub.DefaultBroker, pubsub.Broker(&staleBacklogBroker{pubsub.NewMemoryBroker()}))()
	session := loginUser(t, "user2")
	server := httptest.NewServer(testWebRoutes)
	defer server.Close()

	header := http.Header{}
	header.Set("Cookie", session.GetRawCookie(setting.SessionConfig.CookieName).String())
	conn, _, err := websocket.Dial(t.Context(), strings.Replace(server.URL, "http", "ws", 1)+"/-/ws", &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the body, it is nil after the upgrade
	require.NoError(t, err)
	defer conn.CloseNow()

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
