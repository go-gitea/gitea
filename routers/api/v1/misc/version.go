// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/sdk/gitea"
)

// Version shows the version of the Gitea server
func Version(ctx *context.APIContext) {
	// swagger:route GET /version miscellaneous getVersion
	//
	// Return Gitea running version.
	//
	// This show current running Gitea application version.
	//
	//     Produces:
	//     - application/json
	//
	//     Responses:
	//       200: ServerVersion

	ctx.JSON(200, &gitea.ServerVersion{Version: setting.AppVer})
}
