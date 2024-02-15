// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"path"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

type passkeyEndpoints struct {
	Enroll string `json:"enroll"`
	Manage string `json:"manage"`
}

func PasskeyEndpoints(ctx *context.Context) {
	ctx.JSON(http.StatusOK, passkeyEndpoints{
		Enroll: path.Join(setting.AppURL, setting.AppSubURL, "user/settings/security"),
		Manage: path.Join(setting.AppURL, setting.AppSubURL, "user/settings/security"),
	})
}
