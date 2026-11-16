// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.SetupGiteaTestEnv()
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	m.Run()
}
