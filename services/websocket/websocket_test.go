// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventForSessionTargetsAndKeepsEmptyData(t *testing.T) {
	targeted := makeEventMessage("targeted-event", "sess-A", nil)
	assert.JSONEq(t, `{"eventType":"targeted-event"}`, string(EventForSession(targeted, "sess-A")))
	assert.Nil(t, EventForSession(targeted, "sess-B"))

	broadcast := makeEventMessage("list-event", "", []int{})
	assert.JSONEq(t, `{"eventType":"list-event","eventData":[]}`, string(EventForSession(broadcast, "sess-A")))
}
