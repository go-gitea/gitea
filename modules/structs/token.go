// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// CurrentAccessToken represents the metadata of the currently authenticated token.
// swagger:model CurrentAccessToken
type CurrentAccessToken struct {
	// The unique identifier of the access token
	ID int64 `json:"id"`
	// The name of the access token
	Name string `json:"name"`
	// The scopes granted to this access token
	Scopes []string `json:"scopes"`
	// The timestamp when the token was created
	CreatedAt time.Time `json:"created_at"`
	// The timestamp when the token was last used
	LastUsedAt time.Time `json:"last_used_at"`
	// The owner of the access token
	User *UserMeta `json:"user"`
}

// UserMeta represents minimal user information for the token owner.
type UserMeta struct {
	// The unique identifier of the user
	ID int64 `json:"id"`
	// The username of the user
	Login string `json:"login"`
}
