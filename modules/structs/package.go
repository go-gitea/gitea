// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Package represents a package
type Package struct {
	// The unique identifier of the package
	ID int64 `json:"id"`
	// The owner of the package
	Owner *User `json:"owner"`
	// The repository that contains this package
	Repository *Repository `json:"repository"`
	// The user who created this package
	Creator *User `json:"creator"`
	// The type of the package (e.g., npm, maven, docker)
	Type string `json:"type"`
	// The name of the package
	Name string `json:"name"`
	// The version of the package
	Version string `json:"version"`
	// The HTML URL to view the package
	HTMLURL string `json:"html_url"`
	// swagger:strfmt date-time
	// The date and time when the package was created
	CreatedAt time.Time `json:"created_at"`
}

// PackageFile represents a package file
type PackageFile struct {
	// The unique identifier of the package file
	ID int64 `json:"id"`
	// The size of the package file in bytes
	Size int64 `json:"size"`
	// The name of the package file
	Name string `json:"name"`
	// The MD5 hash of the package file
	HashMD5 string `json:"md5"`
	// The SHA1 hash of the package file
	HashSHA1 string `json:"sha1"`
	// The SHA256 hash of the package file
	HashSHA256 string `json:"sha256"`
	// The SHA512 hash of the package file
	HashSHA512 string `json:"sha512"`
}
