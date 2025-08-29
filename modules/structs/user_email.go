// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Email an email address belonging to a user
type Email struct {
	// swagger:strfmt email
	Email    string `json:"email"`
	Verified bool   `json:"verified"`
	Primary  bool   `json:"primary"`
	UserID   int64  `json:"user_id"`
	// username of the user
	UserName string `json:"username"`
}

// CreateEmailOption options when creating email addresses
type CreateEmailOption struct {
	// email addresses to add
	Emails []string `json:"emails"`
}

// DeleteEmailOption options when deleting email addresses
type DeleteEmailOption struct {
	// email addresses to delete
	Emails []string `json:"emails"`
}
