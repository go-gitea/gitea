// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"time"

	"code.gitea.io/gitea/models/avatars"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/services/context"
)

func cacheableRedirect(ctx *context.Context, location string) {
	// here we should not use `setting.StaticCacheTime`, it is pretty long (default: 6 hours)
	// we must make sure the redirection cache time is short enough, otherwise a user won't see the updated avatar in 6 hours
	// it's OK to make the cache time short, it is only a redirection, and doesn't cost much to make a new request
	httpcache.SetCacheControlInHeader(ctx.Resp.Header(), 5*time.Minute)
	ctx.Redirect(location)
}

// AvatarByUsernameSize redirect browser to user avatar of requested size
func AvatarByUsernameSize(ctx *context.Context) {
	username := ctx.PathParam("username")
	user := user_model.GetSystemUserByName(username)
	if user == nil {
		var err error
		if user, err = user_model.GetUserByName(ctx, username); err != nil {
			ctx.NotFoundOrServerError("GetUserByName", user_model.IsErrUserNotExist, err)
			return
		}
	}
	cacheableRedirect(ctx, user.AvatarLinkWithSize(ctx, int(ctx.PathParamInt64("size"))))
}

// AvatarByEmailHash redirects the browser to the email avatar link
func AvatarByEmailHash(ctx *context.Context) {
	hash := ctx.PathParam("hash")
	email, err := avatars.GetEmailForHash(ctx, hash)
	if err != nil {
		ctx.ServerError("invalid avatar hash: "+hash, err)
		return
	}
	size := ctx.FormInt("size")
	cacheableRedirect(ctx, avatars.GenerateEmailAvatarFinalLink(ctx, email, size))
}
