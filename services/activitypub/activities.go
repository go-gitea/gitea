// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Create and send Follow activity
func Follow(actorUser, followUser *user_model.User) *ap.Follow {
	object := ap.PersonNew(ap.IRI(followUser.LoginName))
	follow := ap.FollowNew("", object)
	follow.Type = ap.FollowType
	follow.Actor = ap.PersonNew(ap.IRI(setting.AppURL + "api/v1/activitypub/user/" + actorUser.Name))
	follow.To = ap.ItemCollection{ap.Item(ap.IRI(followUser.LoginName + "/inbox"))}
	return follow
}

// Create and send Undo Follow activity
func Unfollow(actorUser, followUser *user_model.User) *ap.Undo {
	object := ap.PersonNew(ap.IRI(followUser.LoginName))
	follow := ap.FollowNew("", object)
	follow.Actor = ap.PersonNew(ap.IRI(setting.AppURL + "api/v1/activitypub/user/" + actorUser.Name))
	unfollow := ap.UndoNew("", follow)
	unfollow.Type = ap.UndoType
	unfollow.To = ap.ItemCollection{ap.Item(ap.IRI(followUser.LoginName + "/inbox"))}
	return unfollow
}
