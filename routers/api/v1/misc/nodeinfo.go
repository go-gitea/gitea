// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"net/http"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// NodeInfo returns the NodeInfo for the Gitea instance to allow for federation
func NodeInfo(ctx *context.APIContext) {
	// swagger:operation GET /nodeinfo miscellaneous getNodeInfo
	// ---
	// summary: Returns the nodeinfo of the Gitea application
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/NodeInfo"

	infoUsageUsers := structs.NodeInfoUsageUsers{}
	if setting.Federation.ShareUserStatistics {
		infoUsageUsers.Total = int(user_model.CountUsers(nil))
		now := time.Now()
		timeOneMonthAgo := now.AddDate(0, -1, 0).Unix()
		timeHaveYearAgo := now.AddDate(0, -6, 0).Unix()
		infoUsageUsers.ActiveMonth = int(user_model.CountUsers(&user_model.CountUserFilter{LastLoginSince: &timeOneMonthAgo}))
		infoUsageUsers.ActiveHalfyear = int(user_model.CountUsers(&user_model.CountUserFilter{LastLoginSince: &timeHaveYearAgo}))
	}

	nodeInfo := &structs.NodeInfo{
		Version: "2.1",
		Software: structs.NodeInfoSoftware{
			Name:       "gitea",
			Version:    setting.AppVer,
			Repository: "https://github.com/go-gitea/gitea.git",
			Homepage:   "https://gitea.io/",
		},
		Protocols: []string{"activitypub"},
		Services: structs.NodeInfoServices{
			Inbound:  []string{},
			Outbound: []string{},
		},
		OpenRegistrations: setting.Service.ShowRegistrationButton,
		Usage: structs.NodeInfoUsage{
			Users: infoUsageUsers,
		},
	}
	ctx.JSON(http.StatusOK, nodeInfo)
}
