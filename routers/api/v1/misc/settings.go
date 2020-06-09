// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// SettingGetsAllowedReactions return allowed reactions
func SettingGetsAllowedReactions(ctx *context.APIContext) {
	// swagger:operation GET /settings/allowed_reactions miscellaneous getAllowedReactions
	// ---
	// summary: Returns string array of allowed reactions
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	ctx.JSON(http.StatusOK, setting.UI.Reactions)
}

// GetGeneralRepoSettings returns instance's global settings for repositories
func GetGeneralRepoSettings(ctx *context.APIContext) {
	// swagger:operation GET /settings/repository miscellaneous getGeneralRepositorySettings
	// ---
	// summary: Get instance's global settings for repositories
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/GeneralRepoSettings"
	ctx.JSON(http.StatusOK, api.GeneralRepoSettings{
		MirrorsDisabled: setting.Repository.DisableMirrors,
		HTTPGitDisabled: setting.Repository.DisableHTTPGit,
	})
}
