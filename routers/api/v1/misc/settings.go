// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

// SettingGetsAllowedReactions return allowed reactions
func SettingGetsAllowedReactions(ctx *context.APIContext) {
	// swagger:operation GET /settings/allowed_reactions miscellaneous getAllowedReactions
	// ---
	// summary: Returns the version of the Gitea application
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/StringSlice"
	ctx.JSON(200, setting.UI.Reactions)
}
