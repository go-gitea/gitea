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
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/pubsub"
	pull_service "gitea.dev/services/pull"
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

func TestWebSocketTopicSubscription(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	broker := pubsub.NewMemoryBroker()
	defer test.MockVariableValue(&pubsub.DefaultBroker, pubsub.Broker(broker))()
	session := loginUser(t, "user5")
	server := httptest.NewServer(testWebRoutes)
	defer server.Close()

	header := http.Header{}
	header.Set("Cookie", session.GetRawCookie(setting.SessionConfig.CookieName).String())
	conn, _, err := websocket.Dial(t.Context(), strings.Replace(server.URL, "http", "ws", 1)+"/-/ws", &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the body, it is nil after the upgrade
	require.NoError(t, err)
	defer conn.CloseNow()

	const publicPullID, privatePullID = 2, 6
	for _, topic := range []string{pubsub.UserTopic(1), pubsub.PullTopic(privatePullID), pubsub.PullTopic(publicPullID)} {
		require.NoError(t, conn.Write(t.Context(), websocket.MessageText, fmt.Appendf(nil, `{"type":"subscribe","topic":%q}`, topic)))
	}
	readHead := func() string {
		for {
			_, payload, err := conn.Read(t.Context())
			require.NoError(t, err)
			var event struct {
				EventType string     `json:"eventType"`
				EventData json.Value `json:"eventData"`
			}
			require.NoError(t, json.Unmarshal(payload, &event))
			if event.EventType != "pull-head" {
				continue
			}
			var data struct {
				PullID int64  `json:"pullId"`
				Head   string `json:"head"`
			}
			require.NoError(t, json.Unmarshal(event.EventData, &data))
			require.EqualValues(t, publicPullID, data.PullID)
			return data.Head
		}
	}

	assert.Equal(t, "985f0301dba5e7b34be866819cd15ad3d8f508ee", readHead())
	assert.False(t, broker.HasTopicSubscribers(pubsub.UserTopic(1)))
	assert.False(t, broker.HasTopicSubscribers(pubsub.PullTopic(privatePullID)))
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: publicPullID})
	require.NoError(t, pull_service.PushToBaseRepo(t.Context(), pull))
	headCommitID, err := git.GetBranchCommitID(t.Context(), pull.HeadRepo, pull.HeadBranch)
	require.NoError(t, err)
	assert.Equal(t, headCommitID, readHead())
}
