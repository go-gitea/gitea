// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"strings"

	"code.gitea.io/gitea/models/avatars"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/setting"
)

func cacheableRedirect(ctx *context.Context, location string) {
	httpcache.AddCacheControlToHeader(ctx.Resp.Header(), setting.StaticCacheTime)
	ctx.Redirect(location)
}

// AvatarByUserName redirect browser to user avatar of requested size
func AvatarByUserName(ctx *context.Context) {
	userName := ctx.Params(":username")
	size := int(ctx.ParamsInt64(":size"))

	var user *user_model.User
	if strings.ToLower(userName) != "ghost" {
		var err error
		if user, err = user_model.GetUserByName(ctx, userName); err != nil {
			ctx.ServerError("Invalid user: "+userName, err)
			return
		}
	} else {
		user = user_model.NewGhostUser()
	}

	cacheableRedirect(ctx, user.AvatarLinkWithSize(size))
}

// AvatarByEmailHash redirects the browser to the email avatar link
func AvatarByEmailHash(ctx *context.Context) {
	hash := ctx.Params(":hash")
	email, err := avatars.GetEmailForHash(hash)
	if err != nil {
		ctx.ServerError("invalid avatar hash: "+hash, err)
		return
	}
	size := ctx.FormInt("size")
	cacheableRedirect(ctx, avatars.GenerateEmailAvatarFinalLink(email, size))
}
