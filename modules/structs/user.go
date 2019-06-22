// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"encoding/json"
	"time"
)

// User represents a user
// swagger:model
type User struct {
	// the user's id
	ID int64 `json:"id"`
	// the user's username
	UserName string `json:"login"`
	// the user's full name
	FullName string `json:"full_name"`
	// swagger:strfmt email
	Email string `json:"email"`
	// URL to the user's avatar
	AvatarURL string `json:"avatar_url"`
	// User locale
	Language string `json:"language"`
	// Is the user an administrator
	IsAdmin bool `json:"is_admin"`
	// swagger:strfmt date-time
	LastLogin time.Time `json:"last_login,omitempty"`
	// swagger:strfmt date-time
	Created time.Time `json:"created,omitempty"`
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
