// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/context"
)

// ListSubscribers list a repo's subscribers (i.e. watchers)
func ListSubscribers(ctx *context.APIContext) {
	subscribers, err := ctx.Repo.Repository.GetWatchers(0)
	if err != nil {
		ctx.Error(500, "GetWatchers", err)
		return
	}
	users := make([]*api.User, len(subscribers))
	for i, subscriber := range subscribers {
		users[i] = subscriber.APIFormat()
	}
	ctx.JSON(200, users)
}
