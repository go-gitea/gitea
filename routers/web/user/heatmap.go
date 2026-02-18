// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"net/http"
	"net/url"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func prepareHeatmapURL(ctx *context.Context) {
	ctx.Data["EnableHeatmap"] = setting.Service.EnableUserHeatmap
	if !setting.Service.EnableUserHeatmap {
		return
	}

	if ctx.Org.Organization == nil {
		// for individual user
		ctx.Data["HeatmapURL"] = ctx.Doer.HomeLink() + "/-/heatmap"
		return
	}

	// for org or team
	heatmapURL := ctx.Org.Organization.OrganisationLink() + "/dashboard/-/heatmap"
	if ctx.Org.Team != nil {
		heatmapURL += "/" + url.PathEscape(ctx.Org.Team.LowerName)
	}
	ctx.Data["HeatmapURL"] = heatmapURL
}

func writeHeatmapJSON(ctx *context.Context, hdata []*activities_model.UserHeatmapData) {
	data := make([][2]int64, len(hdata))
	var total int64
	for i, v := range hdata {
		data[i] = [2]int64{int64(v.Timestamp), v.Contributions}
		total += v.Contributions
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"heatmapData":        data,
		"totalContributions": total,
	})
}

// DashboardHeatmap returns heatmap data as JSON, for the individual user, organization or team dashboard.
func DashboardHeatmap(ctx *context.Context) {
	if !setting.Service.EnableUserHeatmap {
		ctx.NotFound(nil)
		return
	}
	var data []*activities_model.UserHeatmapData
	var err error
	if ctx.Org.Organization == nil {
		data, err = activities_model.GetUserHeatmapDataByUser(ctx, ctx.ContextUser, ctx.Doer)
	} else {
		data, err = activities_model.GetUserHeatmapDataByOrgTeam(ctx, ctx.Org.Organization, ctx.Org.Team, ctx.Doer)
	}
	if err != nil {
		ctx.ServerError("GetUserHeatmapData", err)
		return
	}
	writeHeatmapJSON(ctx, data)
}
