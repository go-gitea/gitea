// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"encoding/json"
	"time"
)

// User represents the brief API response fields for models.User
// swagger:model
type User struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's login
	Login string `json:"login"`
	// the user's username
	UserName string `json:"username"`
	// the user's full name
	FullName string `json:"full_name"`
	// swagger:strfmt email
	Email string `json:"email"`
	// Is the user an administrator
	IsAdmin bool `json:"is_admin"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// URL to the user's API endpoint
	URL string `json:"url"`
	// URL to the user's Gitea HTML page
	HTMLURL string `json:"html_url"`
	// Type of the user, User or Org
	Type string `json:"type"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated,omitempty"`
	// swagger:strfmt date-time
	LastLogin time.Time `json:"last_login,omitempty"`
}

// UserDetails represents all the API response fields for a models.User
// swagger:model
type UserDetails struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's login
	Login string `json:"login"`
	// the user's username
	UserName string `json:"username"`
	// the user's full name
	FullName string `json:"full_name"`
	// swagger:strfmt email
	Email string `json:"email"`
	// Is the user an administrator
	HideEmail bool `json:"hide_email"`
	IsAdmin bool `json:"is_admin"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// URL to the user's API endpoint
	URL string `json:"url"`
	// URL to the user's Gitea HTML page
	HTMLURL string `json:"html_url"`
	// Biography about the user
	Description string `json:"bio"`
	// Website of the user
	Website string `json:"website"`
	// Location of the user
	Location string `json:"location"`
	// User locale
	Language string `json:"language"`
	// URL to the user's repos API endpoint
	ReposURL string `json:"repos_url"`
	// URL to the user's followers API endpoint
	FollowersURL string `json:"followers_url"`
	// URL to the user's following API endpoint
	FollowingURL string `json:"following_url"`
	// URL to the user's starred API endpoint
	StarredURL string `json:"starred_url"`
	// URL to the user's subscriptions API endpoint
	SubscriptionsURL string `json:"subscriptions_url"`
	// URL to the user's organizations API endpoint
	OrganizationsURL string `json:"organizations_url"`
	// URL to user's heatmap API endpoint
	HeatmapURL string `json:"heatmap_url"`
	// The user's followers count
	Followers int `json:"followers"`
	// The user's following count
	Following int `json:"following"`
	// The user's public repo count
	PublicRepos int64 `json:"public_repos"`
	// Type of the user, User or Org
	Type string `json:"type"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated,omitempty"`
	// swagger:strfmt date-time
	LastLogin time.Time `json:"last_login,omitempty"`
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
