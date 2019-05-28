// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Organization represents an organization
type Organization struct {
	ID          int64  `json:"id"`
	UserName    string `json:"username"`
	FullName    string `json:"full_name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	Visibility  string `json:"visibility"`
}

// CreateOrgOption options for creating an organization
type CreateOrgOption struct {
	// required: true
	UserName    string `json:"username" binding:"Required"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	// possible values are `public` (default), `limited` or `private`
	// enum: public,limited,private
	Visibility string `json:"visibility" binding:"In(,public,limited,private)"`
}

// EditOrgOption options for editing an organization
type EditOrgOption struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Location    string `json:"location"`
	// possible values are `public`, `limited` or `private`
	// enum: public,limited,private
	Visibility string `json:"visibility" binding:"In(,public,limited,private)"`
}
