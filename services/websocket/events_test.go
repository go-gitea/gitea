// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The marshaled bytes are a contract with web_src/js/types.ts; pin the exact
// output so a struct/tag change can't silently shift what clients receive.
func TestClientEventWireFormat(t *testing.T) {
	t.Run("userEvent envelope keeps the data key present", func(t *testing.T) {
		b, err := json.Marshal(userEvent[int]{Type: "example", Data: 5})
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"example","data":5}`, string(b))
	})

	t.Run("stopwatches empty", func(t *testing.T) {
		b, err := json.Marshal(userEvent[api.StopWatches]{Type: EventStopwatches, Data: api.StopWatches{}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"stopwatches","data":[]}`, string(b))
	})

	t.Run("notification-count stays a flat count", func(t *testing.T) {
		b, err := json.Marshal(notificationCountEvent{Type: EventNotificationCount, Count: 3})
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"notification-count","count":3}`, string(b))
	})
}
