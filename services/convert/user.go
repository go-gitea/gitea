// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"

	"gitea.dev/models/perm"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func UserTypeToString(t user_model.UserType) api.UserTypeString {
	switch t {
	case user_model.UserTypeOrganization, user_model.UserTypeOrganizationReserved:
		return api.UserTypeStringOrganization
	case user_model.UserTypeBot:
		return api.UserTypeStringBot
	default:
		return api.UserTypeStringUser
	}
}

// UserTypeFromString parses a user type an admin may create or convert to
func UserTypeFromString(s api.UserTypeString) (user_model.UserType, error) {
	switch s {
	case api.UserTypeStringUser:
		return user_model.UserTypeIndividual, nil
	case api.UserTypeStringBot:
		return user_model.UserTypeBot, nil
	default:
		return 0, util.NewInvalidArgumentErrorf("invalid user type %q (expected %s, %s)", s, api.UserTypeStringUser, api.UserTypeStringBot)
	}
}

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
		Type:        UserTypeToString(user.Type),
		FullName:    user.FullName,
		Email:       user.GetPlaceholderEmail(),
		AvatarURL:   user.AvatarLink(ctx),
		HTMLURL:     user.HTMLURL(ctx),
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

	result.Visibility = api.VisibilityString(user.Visibility.String())

	// hide primary email if API caller is anonymous or user keep email private
	if signed && (!user.KeepEmailPrivate || authed) {
		result.Email = user.Email
	}

	// only site admin will get these information and possibly user himself
	if authed {
		result.IsAdmin = user.IsAdmin
		result.LoginName = user.LoginName
		result.SourceID = user.LoginSource
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
		Permission: api.AccessLevelName(accessMode.ToString()),
		RoleName:   accessMode.ToString(),
	}
}
