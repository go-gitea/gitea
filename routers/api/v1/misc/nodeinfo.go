// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package misc

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

const cacheKeyNodeInfoUsage = "API_NodeInfoUsage"

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

	nodeInfoUsage := structs.NodeInfoUsage{}
	if setting.Federation.ShareUserStatistics {
		info, ok := ctx.Cache.Get(cacheKeyNodeInfoUsage).(structs.NodeInfoUsage)
		if !ok {
			usersTotal := int(user_model.CountUsers(nil))
			now := time.Now()
			timeOneMonthAgo := now.AddDate(0, -1, 0).Unix()
			timeHaveYearAgo := now.AddDate(0, -6, 0).Unix()
			usersActiveMonth := int(user_model.CountUsers(&user_model.CountUserFilter{LastLoginSince: &timeOneMonthAgo}))
			usersActiveHalfyear := int(user_model.CountUsers(&user_model.CountUserFilter{LastLoginSince: &timeHaveYearAgo}))

			allIssues, _ := models.CountIssues(&models.IssuesOptions{})
			allComments, _ := models.CountComments(&models.FindCommentsOptions{})

			info = structs.NodeInfoUsage{
				Users: structs.NodeInfoUsageUsers{
					Total:          usersTotal,
					ActiveMonth:    usersActiveMonth,
					ActiveHalfyear: usersActiveHalfyear,
				},
				LocalPosts:    int(allIssues),
				LocalComments: int(allComments),
			}
			if err := ctx.Cache.Put(cacheKeyNodeInfoUsage, nodeInfoUsage, 180); err != nil {
				ctx.InternalServerError(err)
				return
			}
		}
		nodeInfoUsage = info
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
			Outbound: []string{"rss2.0"},
		},
		OpenRegistrations: setting.Service.ShowRegistrationButton,
		Usage:             nodeInfoUsage,
	}
	ctx.JSON(http.StatusOK, nodeInfo)
}
