// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"gitea.dev/services/websocket"

	"github.com/stretchr/testify/assert"
)

func TestFilterLogout(t *testing.T) {
	cases := []struct {
		name       string
		brokerMsg  []byte
		connSessID string
		want       []byte // expected payload forwarded to the client
	}{
		{
			name:       "originating session gets a session-free logout",
			brokerMsg:  websocket.MakeUserEventMessage("logout", websocket.LogoutEventData{SessionID: "sess-A"}),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"logout"}`),
		},
		{
			name:       "other session is dropped",
			brokerMsg:  websocket.MakeUserEventMessage("logout", websocket.LogoutEventData{SessionID: "sess-A"}),
			connSessID: "sess-B",
			want:       nil,
		},
		{
			name:       "empty sessionID reaches every session",
			brokerMsg:  websocket.MakeUserEventMessage("logout", nil),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"logout"}`),
		},
		{
			name:       "non-logout message passes through unchanged",
			brokerMsg:  websocket.MakeUserEventMessage("other", map[string]any{"k": "v"}),
			connSessID: "sess-A",
			want:       []byte(`{"eventType":"other","eventData":{"k":"v"}}`),
		},
		{
			name:       "malformed payload passes through unchanged",
			brokerMsg:  []byte("not json"),
			connSessID: "sess-A",
			want:       []byte("not json"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, filterLogout(tc.brokerMsg, tc.connSessID))
		})
	}
}
