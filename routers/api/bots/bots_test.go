// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"testing"

	"code.gitea.io/gitea/routers/api/bots/ping"
)

func TestPingService(t *testing.T) {
	ping.MainServiceTest(t, Routes(context.Background(), ""))
}
