// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	tplIPs templates.TplName = "admin/ips/list"
)

// IPs show all user signup IPs
func IPs(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.ips.ip")
	ctx.Data["PageIsAdminIPs"] = true
	ctx.Data["RecordUserSignupMetadata"] = setting.RecordUserSignupMetadata

	// If record user signup metadata is disabled, don't show the page
	if !setting.RecordUserSignupMetadata {
		ctx.Redirect(setting.AppSubURL + "/-/admin")
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	// Define the user IP result struct
	type UserIPResult struct {
		UID      int64
		Name     string
		FullName string
		IP       string
	}

	var (
		userIPs  []UserIPResult
		count    int64
		err      error
		orderBy  string
		keyword  = ctx.FormTrim("q")
		sortType = ctx.FormString("sort")
	)

	ctx.Data["SortType"] = sortType
	switch sortType {
	case "ip":
		orderBy = "user_setting.setting_value ASC, user.id ASC"
	case "reverseip":
		orderBy = "user_setting.setting_value DESC, user.id DESC"
	case "username":
		orderBy = "user.lower_name ASC, user.id ASC"
	case "reverseusername":
		orderBy = "user.lower_name DESC, user.id DESC"
	default:
		ctx.Data["SortType"] = "ip"
		orderBy = "user_setting.setting_value ASC, user.id ASC"
	}

	// Get the count and user IPs for pagination
	query := user_model.BuildSignupIPQuery(ctx, keyword)

	count, err = query.Count(new(user_model.Setting))
	if err != nil {
		ctx.ServerError("Count", err)
		return
	}

	err = user_model.BuildSignupIPQuery(ctx, keyword).
		Select("user.id as uid, user.name, user.full_name, user_setting.setting_value as ip").
		OrderBy(orderBy).
		Limit(setting.UI.Admin.UserPagingNum, (page-1)*setting.UI.Admin.UserPagingNum).
		Find(&userIPs)
	if err != nil {
		ctx.ServerError("Find", err)
		return
	}

	for i := range userIPs {
		// Trim the port from the IP
		// FIXME: Maybe have a different helper for this?
		userIPs[i].IP = util.TrimPortFromIP(userIPs[i].IP)
	}

	ctx.Data["UserIPs"] = userIPs
	ctx.Data["Total"] = count
	ctx.Data["Keyword"] = keyword

	// Setup pagination
	pager := context.NewPagination(int(count), setting.UI.Admin.UserPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplIPs)
}
