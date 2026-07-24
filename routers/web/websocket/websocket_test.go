// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterLogout(t *testing.T) {
	cases := []struct {
		name       string
		brokerMsg  string
		connSessID string
		want       string // expected payload forwarded to the client
		wantDrop   bool   // message dropped, nothing forwarded
	}{
		{
			name:       "originating session gets a session-free logout",
			brokerMsg:  `{"type":"logout","sessionID":"sess-A"}`,
			connSessID: "sess-A",
			want:       `{"type":"logout"}`,
		},
		{
			name:       "other session is dropped",
			brokerMsg:  `{"type":"logout","sessionID":"sess-A"}`,
			connSessID: "sess-B",
			wantDrop:   true,
		},
		{
			name:       "empty sessionID reaches every session",
			brokerMsg:  `{"type":"logout"}`,
			connSessID: "sess-A",
			want:       `{"type":"logout"}`,
		},
		{
			name:       "non-logout message passes through unchanged",
			brokerMsg:  `{"type":"notification-count","count":3}`,
			connSessID: "sess-A",
			want:       `{"type":"notification-count","count":3}`,
		},
		{
			name:       "malformed JSON with logout marker passes through unchanged",
			brokerMsg:  `not json but mentions "type":"logout" somewhere`,
			connSessID: "sess-A",
			want:       `not json but mentions "type":"logout" somewhere`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := filterLogout([]byte(tc.brokerMsg), tc.connSessID)
			if tc.wantDrop {
				assert.Nil(t, out)
				return
			}
			require.NotNil(t, out)
			assert.Equal(t, tc.want, string(out))
		})
	}
}
