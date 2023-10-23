// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	timezone_module "code.gitea.io/gitea/modules/timezone"
	"code.gitea.io/gitea/services/convert"
)

// Returns a list of all timezones
func ListTimezones(ctx *context.APIContext) {
	// swagger:operation GET /timezones miscellaneous listTimezones
	// ---
	// summary: Returns a list of all timeszones
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/TimeZoneList"
	zoneList, err := timezone_module.GetTimeZoneList()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTimeZoneList", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToTimeZoneList(zoneList))
}
