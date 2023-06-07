// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGiteaHookTemplates(t *testing.T) {
	oldAppPath := setting.AppPath
	oldCustomConf := setting.CustomConf
	oldWorkPath := setting.AppWorkPath
	oldCustomPath := setting.CustomPath

	setting.AppPath = "/snap/gitea/1234/gitea"
	setting.CustomConf = "/some/custom/app.ini"
	setting.AppWorkPath = "/some/path/gitea"
	setting.CustomPath = "/some/other/custom"

	defer func() {
		setting.AppPath = oldAppPath
		setting.CustomConf = oldCustomConf
		setting.AppWorkPath = oldWorkPath
		setting.CustomPath = oldCustomPath
	}()

	hookNames, _, giteaHookTpls := getHookTemplates()

	for i, hookName := range hookNames {
		giteaHookTpl := giteaHookTpls[i]
		if giteaHookTpl != "" {
			assert.Contains(t, giteaHookTpl, "/snap/gitea/1234/gitea hook")
			assert.Contains(t, giteaHookTpl, "--config=/some/custom/app.ini")
			assert.Contains(t, giteaHookTpl, "--work-path=/some/path/gitea")
			assert.Contains(t, giteaHookTpl, "--custom-path=/some/other/custom")
			assert.Contains(t, giteaHookTpl, hookName)
		}
	}
}
