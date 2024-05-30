// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markdown

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/markup"
)

func TestMain(m *testing.M) {
	markup.Init(&markup.ProcessorHelper{
		IsUsernameMentionable: func(ctx context.Context, username string) bool {
			return username == "r-lyeh"
		},
	})
	unittest.MainTest(m)
}
