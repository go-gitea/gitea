// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

const (
	tplNotices base.TplName = "admin/notice"
)

// Notices show notices for admin
func Notices(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.notices")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminNotices"] = true

	total := models.CountNotices()
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	notices, err := models.Notices(page, setting.UI.Admin.NoticePagingNum)
	if err != nil {
		ctx.ServerError("Notices", err)
		return
	}
	ctx.Data["Notices"] = notices

	ctx.Data["Total"] = total

	ctx.Data["Page"] = context.NewPagination(int(total), setting.UI.Admin.NoticePagingNum, page, 5)

	ctx.HTML(200, tplNotices)
}

// DeleteNotices delete the specific notices
func DeleteNotices(ctx *context.Context) {
	strs := ctx.QueryStrings("ids[]")
	ids := make([]int64, 0, len(strs))
	for i := range strs {
		id := com.StrTo(strs[i]).MustInt64()
		if id > 0 {
			ids = append(ids, id)
		}
	}

	if err := models.DeleteNoticesByIDs(ids); err != nil {
		ctx.Flash.Error("DeleteNoticesByIDs: " + err.Error())
		ctx.Status(500)
	} else {
		ctx.Flash.Success(ctx.Tr("admin.notices.delete_success"))
		ctx.Status(200)
	}
}

// EmptyNotices delete all the notices
func EmptyNotices(ctx *context.Context) {
	if err := models.DeleteNotices(0, 0); err != nil {
		ctx.ServerError("DeleteNotices", err)
		return
	}

	log.Trace("System notices deleted by admin (%s): [start: %d]", ctx.User.Name, 0)
	ctx.Flash.Success(ctx.Tr("admin.notices.delete_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/notices")
}
