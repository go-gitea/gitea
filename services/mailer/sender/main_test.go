// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package sender

import (
	"os"
	"testing"

	"gitea.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.SetupGiteaTestEnv()
	os.Exit(m.Run())
}
