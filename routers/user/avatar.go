// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// Avatar redirect browser to user avatar of requested size
func Avatar(ctx *context.Context) {
	userName := ctx.Params(":username")
	size, err := strconv.Atoi(ctx.Params(":size"))
	if err != nil {
		ctx.ServerError("Invalid avatar size", err)
		return
	}

	log.Debug("Asked avatar for user %v and size %v", userName, size)

	user, err := models.GetUserByName(userName)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.ServerError("Requested avatar for invalid user", err)
		} else {
			ctx.ServerError("Retrieving user by name", err)
		}
		return
	}

	ctx.Redirect(user.RealSizedAvatarLink(size))
}
