// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"os"
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.IsInTesting = true
	setting.StaticRootPath = "../../../" // AssetFS reads the emoji data from public/
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	os.Exit(m.Run())
}
