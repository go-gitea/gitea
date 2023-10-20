// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"strings"
	"time"

	"code.gitea.io/gitea/models/avatars"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httpcache"
)

func cacheableRedirect(ctx *context.Context, location string) {
	// here we should not use `setting.StaticCacheTime`, it is pretty long (default: 6 hours)
	// we must make sure the redirection cache time is short enough, otherwise a user won't see the updated avatar in 6 hours
	// it's OK to make the cache time short, it is only a redirection, and doesn't cost much to make a new request
	httpcache.SetCacheControlInHeader(ctx.Resp.Header(), 5*time.Minute)
	ctx.Redirect(location)
}

// AvatarByUserName redirect browser to user avatar of requested size
func AvatarByUserName(ctx *context.Context) {
	userName := ctx.Params(":username")
	size := int(ctx.ParamsInt64(":size"))

	var user *user_model.User
	if strings.ToLower(userName) != user_model.GhostUserLowerName {
		var err error
		if user, err = user_model.GetUserByName(ctx, userName); err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName", err)
				return
			}
			ctx.ServerError("Invalid user: "+userName, err)
			return
		}
	} else {
		user = user_model.NewGhostUser()
	}

	cacheableRedirect(ctx, user.AvatarLinkWithSize(ctx, size))
}

// AvatarByEmailHash redirects the browser to the email avatar link
func AvatarByEmailHash(ctx *context.Context) {
	hash := ctx.Params(":hash")
	email, err := avatars.GetEmailForHash(ctx, hash)
	if err != nil {
		ctx.ServerError("invalid avatar hash: "+hash, err)
		return
	}
	size := ctx.FormInt("size")
	cacheableRedirect(ctx, avatars.GenerateEmailAvatarFinalLink(ctx, email, size))
}
