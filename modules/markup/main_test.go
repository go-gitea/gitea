// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"os"
	"testing"

	"codenest.dev/modules/markup"
	"codenest.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.SetupCodeNestTestEnv()
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	markup.RefreshFileNamePatterns()
	os.Exit(m.Run())
}
