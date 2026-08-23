// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"context"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/translation"

	_ "gitea.dev/models"
	_ "gitea.dev/models/actions"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{SetUp: func() error {
		setting.Langs = []string{"en-US"}
		setting.Names = []string{"English"}
		translation.InitLocales(context.Background())
		return nil
	}})
}
