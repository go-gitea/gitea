// Copyright 2020 The Gitea Authors.
// SPDX-License-Identifier: MIT

package admin

import (
	"bytes"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplEmails base.TplName = "admin/emails/list"
)

// Emails show all emails
func Emails(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.emails")
	ctx.Data["PageIsAdminEmails"] = true

	opts := &user_model.SearchEmailOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.Admin.UserPagingNum,
			Page:     ctx.FormInt("page"),
		},
	}

	if opts.Page <= 1 {
		opts.Page = 1
	}

	type ActiveEmail struct {
		user_model.SearchEmailResult
		CanChange bool
	}

	var (
		baseEmails []*user_model.SearchEmailResult
		emails     []ActiveEmail
		count      int64
		err        error
		orderBy    user_model.SearchEmailOrderBy
	)

	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
	case "email":
		orderBy = user_model.SearchEmailOrderByEmail
	case "reverseemail":
		orderBy = user_model.SearchEmailOrderByEmailReverse
	case "username":
		orderBy = user_model.SearchEmailOrderByName
	case "reverseusername":
		orderBy = user_model.SearchEmailOrderByNameReverse
	default:
		ctx.Data["SortType"] = "email"
		orderBy = user_model.SearchEmailOrderByEmail
	}

	opts.Keyword = ctx.FormTrim("q")
	opts.SortType = orderBy
	if len(ctx.FormString("is_activated")) != 0 {
		opts.IsActivated = util.OptionalBoolOf(ctx.FormBool("activated"))
	}
	if len(ctx.FormString("is_primary")) != 0 {
		opts.IsPrimary = util.OptionalBoolOf(ctx.FormBool("primary"))
	}

	if len(opts.Keyword) == 0 || isKeywordValid(opts.Keyword) {
		baseEmails, count, err = user_model.SearchEmails(ctx, opts)
		if err != nil {
			ctx.ServerError("SearchEmails", err)
			return
		}
		emails = make([]ActiveEmail, len(baseEmails))
		for i := range baseEmails {
			emails[i].SearchEmailResult = *baseEmails[i]
			// Don't let the admin deactivate its own primary email address
			// We already know the user is admin
			emails[i].CanChange = ctx.Doer.ID != emails[i].UID || !emails[i].IsPrimary
		}
	}
	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["Total"] = count
	ctx.Data["Emails"] = emails

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplEmails)
}

var nullByte = []byte{0x00}

func isKeywordValid(keyword string) bool {
	return !bytes.Contains([]byte(keyword), nullByte)
}

// ActivateEmail serves a POST request for activating/deactivating a user's email
func ActivateEmail(ctx *context.Context) {
	truefalse := map[string]bool{"1": true, "0": false}

	uid := ctx.FormInt64("uid")
	email := ctx.FormString("email")
	primary, okp := truefalse[ctx.FormString("primary")]
	activate, oka := truefalse[ctx.FormString("activate")]

	if uid == 0 || len(email) == 0 || !okp || !oka {
		ctx.Error(http.StatusBadRequest)
		return
	}

	log.Info("Changing activation for User ID: %d, email: %s, primary: %v to %v", uid, email, primary, activate)

	if err := user_model.ActivateUserEmail(ctx, uid, email, activate); err != nil {
		log.Error("ActivateUserEmail(%v,%v,%v): %v", uid, email, activate, err)
		if user_model.IsErrEmailAlreadyUsed(err) {
			ctx.Flash.Error(ctx.Tr("admin.emails.duplicate_active"))
		} else {
			ctx.Flash.Error(ctx.Tr("admin.emails.not_updated", err))
		}
	} else {
		log.Info("Activation for User ID: %d, email: %s, primary: %v changed to %v", uid, email, primary, activate)
		ctx.Flash.Info(ctx.Tr("admin.emails.updated"))
	}

	redirect, _ := url.Parse(setting.AppSubURL + "/admin/emails")
	q := url.Values{}
	if val := ctx.FormTrim("q"); len(val) > 0 {
		q.Set("q", val)
	}
	if val := ctx.FormTrim("sort"); len(val) > 0 {
		q.Set("sort", val)
	}
	if val := ctx.FormTrim("is_primary"); len(val) > 0 {
		q.Set("is_primary", val)
	}
	if val := ctx.FormTrim("is_activated"); len(val) > 0 {
		q.Set("is_activated", val)
	}
	redirect.RawQuery = q.Encode()
	ctx.Redirect(redirect.String())
}
