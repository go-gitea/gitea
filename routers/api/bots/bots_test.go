// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"context"
	"testing"

	"code.gitea.io/gitea/routers/api/bots/ping"
)

func TestPingService(t *testing.T) {
	ping.MainServiceTest(t, Routes(context.Background(), ""))
}
