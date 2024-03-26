// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
)

// ToUser convert user_model.User to api.User
// if doer is set, private information is added if the doer has the permission to see it
func ToUser(ctx context.Context, user, doer *user_model.User) *api.User {
	if user == nil {
		return nil
	}
	authed := false
	signed := false
	if doer != nil {
		signed = true
		authed = doer.ID == user.ID || doer.IsAdmin
	}
	return toUser(ctx, user, signed, authed)
}

// ToUsers convert list of user_model.User to list of api.User
func ToUsers(ctx context.Context, doer *user_model.User, users []*user_model.User) []*api.User {
	result := make([]*api.User, len(users))
	for i := range users {
		result[i] = ToUser(ctx, users[i], doer)
	}
	return result
}

// ToUserWithAccessMode convert user_model.User to api.User
// AccessMode is not none show add some more information
func ToUserWithAccessMode(ctx context.Context, user *user_model.User, accessMode perm.AccessMode) *api.User {
	if user == nil {
		return nil
	}
	return toUser(ctx, user, accessMode != perm.AccessModeNone, false)
}

// toUser convert user_model.User to api.User
// signed shall only be set if requester is logged in. authed shall only be set if user is site admin or user himself
func toUser(ctx context.Context, user *user_model.User, signed, authed bool) *api.User {
	result := &api.User{
		ID:          user.ID,
		UserName:    user.Name,
		FullName:    user.FullName,
		Email:       user.GetPlaceholderEmail(),
		AvatarURL:   user.AvatarLink(ctx),
		Created:     user.CreatedUnix.AsTime(),
		Restricted:  user.IsRestricted,
		Location:    user.Location,
		Website:     user.Website,
		Description: user.Description,
		// counter's
		Followers:    user.NumFollowers,
		Following:    user.NumFollowing,
		StarredRepos: user.NumStars,
	}

	result.Visibility = user.Visibility.String()

	// hide primary email if API caller is anonymous or user keep email private
	if signed && (!user.KeepEmailPrivate || authed) {
		result.Email = user.Email
	}

	// only site admin will get these information and possibly user himself
	if authed {
		result.IsAdmin = user.IsAdmin
		result.LoginName = user.LoginName
		result.LastLogin = user.LastLoginUnix.AsTime()
		result.Language = user.Language
		result.IsActive = user.IsActive
		result.ProhibitLogin = user.ProhibitLogin
	}
	return result
}

// User2UserSettings return UserSettings based on a user
func User2UserSettings(user *user_model.User) api.UserSettings {
	return api.UserSettings{
		FullName:      user.FullName,
		Website:       user.Website,
		Location:      user.Location,
		Language:      user.Language,
		Description:   user.Description,
		Theme:         user.Theme,
		HideEmail:     user.KeepEmailPrivate,
		HideActivity:  user.KeepActivityPrivate,
		DiffViewStyle: user.DiffViewStyle,
	}
}

// ToUserAndPermission return User and its collaboration permission for a repository
func ToUserAndPermission(ctx context.Context, user, doer *user_model.User, accessMode perm.AccessMode) api.RepoCollaboratorPermission {
	return api.RepoCollaboratorPermission{
		User:       ToUser(ctx, user, doer),
		Permission: accessMode.String(),
		RoleName:   accessMode.String(),
	}
}

// ToStarList convert repo_model.StarList to api.StarList
func ToStarList(ctx context.Context, starList *repo_model.StarList, doer *user_model.User) *api.StarList {
	return &api.StarList{
		ID:              starList.ID,
		Name:            starList.Name,
		Description:     starList.Description,
		IsPrivate:       starList.IsPrivate,
		RepositoryCount: starList.RepositoryCount,
		User:            ToUser(ctx, starList.User, doer),
	}
}

// ToStarLists convert repo_model.StarListSLice to list of api.StarList
func ToStarLists(ctx context.Context, starLists repo_model.StarListSlice, doer *user_model.User) []*api.StarList {
	apiList := make([]*api.StarList, len(starLists))
	for i, list := range starLists {
		apiList[i] = ToStarList(ctx, list, doer)
	}
	return apiList
}
