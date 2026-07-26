// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"gitea.dev/services/websocket"

	"github.com/stretchr/testify/assert"
)

func TestPayloadForSession(t *testing.T) {
	cases := []struct {
		name       string
		brokerMsg  []byte
		connSessID string
		want       []byte // payload forwarded to the client, nil to drop
	}{
		{
			name:       "targeted session receives the payload",
			brokerMsg:  websocket.MakeBrokerMessage("sess-A", websocket.EventLogout, nil),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"logout","eventData":null}`),
		},
		{
			name:       "other session is dropped",
			brokerMsg:  websocket.MakeBrokerMessage("sess-A", websocket.EventLogout, nil),
			connSessID: "sess-B",
			want:       nil,
		},
		{
			name:       "no target reaches every session",
			brokerMsg:  websocket.MakeBrokerMessage("", websocket.EventLogout, nil),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"logout","eventData":null}`),
		},
		{
			name:       "routing never reaches the client",
			brokerMsg:  websocket.MakeBrokerMessage("sess-A", websocket.EventNotificationCount, map[string]any{"count": 1}),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"notification-count","eventData":{"count":1}}`),
		},
		{
			name:       "malformed broker message is dropped",
			brokerMsg:  []byte("not json"),
			connSessID: "sess-A",
			want:       nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, payloadForSession(tc.brokerMsg, tc.connSessID))
		})
	}
}
