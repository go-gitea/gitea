// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"

	"code.gitea.io/gitea/modules/json"
)

// User represents a user
// swagger:model
type User struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's username
	UserName string `json:"login"`
	// the user's authentication sign-in name.
	// default: empty
	LoginName string `json:"login_name"`
	// The ID of the user's Authentication Source
	SourceID int64 `json:"source_id"`
	// the user's full name
	FullName string `json:"full_name"`
	// swagger:strfmt email
	Email string `json:"email"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// URL to the user's gitea page
	HTMLURL string `json:"html_url"`
	// User locale
	Language string `json:"language"`
	// Is the user an administrator
	IsAdmin bool `json:"is_admin"`
	// swagger:strfmt date-time
	LastLogin time.Time `json:"last_login,omitempty"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
	// Is user restricted
	Restricted bool `json:"restricted"`
	// Is user active
	IsActive bool `json:"active"`
	// Is user login prohibited
	ProhibitLogin bool `json:"prohibit_login"`
	// the user's location
	Location string `json:"location"`
	// the user's website
	Website string `json:"website"`
	// the user's description
	Description string `json:"description"`
	// User visibility level option: public, limited, private
	Visibility string `json:"visibility"`

	// user counts
	Followers    int `json:"followers_count"`
	Following    int `json:"following_count"`
	StarredRepos int `json:"starred_repos_count"`
}

// MarshalJSON implements the json.Marshaler interface for User, adding field(s) for backward compatibility
func (u User) MarshalJSON() ([]byte, error) {
	// Re-declaring User to avoid recursion
	type shadow User
	return json.Marshal(struct {
		shadow
		CompatUserName string `json:"username"`
	}{shadow(u), u.UserName})
}

// UserSettings represents user settings
// swagger:model
type UserSettings struct {
	FullName      string `json:"full_name"`
	Website       string `json:"website"`
	Description   string `json:"description"`
	Location      string `json:"location"`
	Language      string `json:"language"`
	Theme         string `json:"theme"`
	DiffViewStyle string `json:"diff_view_style"`
	// Privacy
	HideEmail    bool `json:"hide_email"`
	HideActivity bool `json:"hide_activity"`
}

// UserSettingsOptions represents options to change user settings
// swagger:model
type UserSettingsOptions struct {
	FullName      *string `json:"full_name" binding:"MaxSize(100)"`
	Website       *string `json:"website" binding:"OmitEmpty;ValidUrl;MaxSize(255)"`
	Description   *string `json:"description" binding:"MaxSize(255)"`
	Location      *string `json:"location" binding:"MaxSize(50)"`
	Language      *string `json:"language"`
	Theme         *string `json:"theme"`
	DiffViewStyle *string `json:"diff_view_style"`
	// Privacy
	HideEmail    *bool `json:"hide_email"`
	HideActivity *bool `json:"hide_activity"`
}

// RenameUserOption options when renaming a user
type RenameUserOption struct {
	// New username for this user. This name cannot be in use yet by any other user.
	//
	// required: true
	// unique: true
	NewName string `json:"new_username" binding:"Required"`
}

// UpdateUserAvatarUserOption options when updating the user avatar
type UpdateUserAvatarOption struct {
	// image must be base64 encoded
	Image string `json:"image" binding:"Required"`
}

// Badge represents a user badge
// swagger:model
type Badge struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

// UserBadge represents a user badge
// swagger:model
type UserBadge struct {
	ID      int64 `json:"id"`
	BadgeID int64 `json:"badge_id"`
	UserID  int64 `json:"user_id"`
}

// UserBadgeOption options for link between users and badges
type UserBadgeOption struct {
	// example: ["badge1","badge2"]
	BadgeSlugs []string `json:"badge_slugs" binding:"Required"`
}
