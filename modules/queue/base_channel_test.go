// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package queue

import "testing"

func TestBaseChannel(t *testing.T) {
	testQueueBasic(t, newBaseChannelSimple, &BaseConfig{ManagedName: "baseChannel", Length: 10}, false)
	testQueueBasic(t, newBaseChannelUnique, &BaseConfig{ManagedName: "baseChannel", Length: 10}, true)
}
