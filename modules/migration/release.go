// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"time"
)

// Release represents a release
type Release struct {
	TagName         string `yaml:"tag_name"`         // SECURITY: This must pass git.IsValidRefPattern
	TargetCommitish string `yaml:"target_commitish"` // SECURITY: This must pass git.IsValidRefPattern
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
	PublisherID     int64  `yaml:"publisher_id"`
	PublisherName   string `yaml:"publisher_name"`
	PublisherEmail  string `yaml:"publisher_email"`
	Assets          []*Asset
	Created         time.Time
	Published       time.Time
}

// GetExternalName ExternalUserMigrated interface
func (r *Release) GetExternalName() string { return r.PublisherName }

// GetExternalID ExternalUserMigrated interface
func (r *Release) GetExternalID() int64 { return r.PublisherID }
