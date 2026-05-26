// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteLogout(t *testing.T) {
	cases := []struct {
		name       string
		brokerMsg  string
		connSessID string
		wantData   string // expected "data" field after rewrite; "" means message unchanged
		wantDrop   bool
	}{
		{
			name:       "originating session sees here",
			brokerMsg:  `{"type":"logout","sessionID":"sess-A"}`,
			connSessID: "sess-A",
			wantData:   "here",
		},
		{
			name:       "other session sees elsewhere",
			brokerMsg:  `{"type":"logout","sessionID":"sess-A"}`,
			connSessID: "sess-B",
			wantData:   "elsewhere",
		},
		{
			name:       "empty sessionID broadcasts as here",
			brokerMsg:  `{"type":"logout"}`,
			connSessID: "sess-A",
			wantData:   "here",
		},
		{
			name:       "non-logout message passes through unchanged",
			brokerMsg:  `{"type":"notification-count","count":3}`,
			connSessID: "sess-A",
			wantData:   "",
		},
		{
			name:       "malformed JSON with logout marker passes through unchanged",
			brokerMsg:  `not json but mentions "type":"logout" somewhere`,
			connSessID: "sess-A",
			wantData:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := rewriteLogout([]byte(tc.brokerMsg), tc.connSessID)
			if tc.wantDrop {
				assert.Nil(t, out)
				return
			}
			require.NotNil(t, out)
			if tc.wantData == "" {
				assert.Equal(t, tc.brokerMsg, string(out), "unchanged passthrough expected")
				return
			}
			var m logoutClientMsg
			require.NoError(t, json.Unmarshal(out, &m))
			assert.Equal(t, "logout", m.Type)
			assert.Equal(t, tc.wantData, m.Data)
		})
	}
}
