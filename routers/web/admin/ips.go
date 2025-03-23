// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"

	"xorm.io/xorm"
)

const (
	tplIPs templates.TplName = "admin/ips/list"
)

// trimPortFromIP removes the client port from an IP address
// Handles both IPv4 and IPv6 addresses with ports
func trimPortFromIP(ip string) string {
	// Handle IPv6 with brackets: [IPv6]:port
	if strings.HasPrefix(ip, "[") {
		// If there's no port, return as is
		if !strings.Contains(ip, "]:") {
			return ip
		}
		// Remove the port part after ]:
		return strings.Split(ip, "]:")[0] + "]"
	}

	// Count colons to differentiate between IPv4 and IPv6
	colonCount := strings.Count(ip, ":")

	// Handle IPv4 with port (single colon)
	if colonCount == 1 {
		return strings.Split(ip, ":")[0]
	}

	return ip
}

func buildIPQuery(ctx *context.Context, keyword string) *xorm.Session {
	query := db.GetEngine(ctx).
		Table("user_setting").
		Join("INNER", "user", "user.id = user_setting.user_id").
		Where("user_setting.setting_key = ?", user_model.SignupIP)

	if len(keyword) > 0 {
		query = query.And("(user.lower_name LIKE ? OR user.full_name LIKE ? OR user_setting.setting_value LIKE ?)",
			"%"+strings.ToLower(keyword)+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	return query
}

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
	query := buildIPQuery(ctx, keyword)

	count, err = query.Count(new(user_model.Setting))
	if err != nil {
		ctx.ServerError("Count", err)
		return
	}

	err = buildIPQuery(ctx, keyword).
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
		userIPs[i].IP = trimPortFromIP(userIPs[i].IP)
	}

	ctx.Data["UserIPs"] = userIPs
	ctx.Data["Total"] = count
	ctx.Data["Keyword"] = keyword

	// Setup pagination
	ctx.Data["Page"] = context.NewPagination(int(count), setting.UI.Admin.UserPagingNum, page, 5)

	ctx.HTML(http.StatusOK, tplIPs)
}
