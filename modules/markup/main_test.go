// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup_test

import (
	"os"
	"testing"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
)

func TestMain(m *testing.M) {
	setting.IsInTesting = true
	markup.RenderBehaviorForTesting.DisableAdditionalAttributes = true
	setting.Markdown.FileNamePatterns = []string{"*.md"}
	markup.RefreshFileNamePatterns()
	os.Exit(m.Run())
}
