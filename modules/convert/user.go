// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/markup"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUser convert models.User to api.User
// signed shall only be set if requester is logged in. authed shall only be set if user is site admin or user himself
func ToUser(user *models.User, signed, authed bool) *api.User {
	if user == nil {
		return nil
	}
	result := &api.User{
		ID:        user.ID,
		UserName:  user.Name,
		FullName:  markup.Sanitize(user.FullName),
		Email:     user.GetEmail(),
		AvatarURL: user.AvatarLink(),
		Created:   user.CreatedUnix.AsTime(),
	}
	// hide primary email if API caller is anonymous or user keep email private
	if signed && (!user.KeepEmailPrivate || authed) {
		result.Email = user.Email
	}
	// only site admin will get these information and possibly user himself
	if authed {
		result.IsAdmin = user.IsAdmin
		result.LastLogin = user.LastLoginUnix.AsTime()
		result.Language = user.Language
	}
	return result
}
