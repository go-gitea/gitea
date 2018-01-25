// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"github.com/Unknwon/com"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	// tplMembers template for organization members page
	tplMembers base.TplName = "org/member/members"
)

// Members render orgnization users page
func Members(ctx *context.Context) {
	org := ctx.Org.Organization
	ctx.Data["Title"] = org.FullName
	ctx.Data["PageIsOrgMembers"] = true

	if err := org.GetMembers(); err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}
	ctx.Data["Members"] = org.Members

	ctx.HTML(200, tplMembers)
}

// MembersAction response for operation to a member of orgnization
func MembersAction(ctx *context.Context) {
	uid := com.StrTo(ctx.Query("uid")).MustInt64()
	if uid == 0 {
		ctx.Redirect(ctx.Org.OrgLink + "/members")
		return
	}

	org := ctx.Org.Organization
	var err error
	switch ctx.Params(":action") {
	case "private":
		if ctx.User.ID != uid && !ctx.Org.IsOwner {
			ctx.Error(404)
			return
		}
		err = models.ChangeOrgUserStatus(org.ID, uid, false)
	case "public":
		if ctx.User.ID != uid && !ctx.Org.IsOwner {
			ctx.Error(404)
			return
		}
		err = models.ChangeOrgUserStatus(org.ID, uid, true)
	case "remove":
		if !ctx.Org.IsOwner {
			ctx.Error(404)
			return
		}
		err = org.RemoveMember(uid)
		if models.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.Redirect(ctx.Org.OrgLink + "/members")
			return
		}
	case "leave":
		err = org.RemoveMember(ctx.User.ID)
		if models.IsErrLastOrgOwner(err) {
			ctx.Flash.Error(ctx.Tr("form.last_org_owner"))
			ctx.Redirect(ctx.Org.OrgLink + "/members")
			return
		}
	}

	if err != nil {
		log.Error(4, "Action(%s): %v", ctx.Params(":action"), err)
		ctx.JSON(200, map[string]interface{}{
			"ok":  false,
			"err": err.Error(),
		})
		return
	}

	if ctx.Params(":action") != "leave" {
		ctx.Redirect(ctx.Org.OrgLink + "/members")
	} else {
		ctx.Redirect(setting.AppSubURL + "/")
	}
}
