// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"xorm.io/xorm"
)

// Permission copied from models.actions.Permission
type Permission int

const (
	PermissionUnspecified Permission = iota
	PermissionNone
	PermissionRead
	PermissionWrite
)

// Permissions copied from models.actions.Permissions
type Permissions struct {
	Actions            Permission `yaml:"actions"`
	Checks             Permission `yaml:"checks"`
	Contents           Permission `yaml:"contents"`
	Deployments        Permission `yaml:"deployments"`
	IDToken            Permission `yaml:"id-token"`
	Issues             Permission `yaml:"issues"`
	Discussions        Permission `yaml:"discussions"`
	Packages           Permission `yaml:"packages"`
	Pages              Permission `yaml:"pages"`
	PullRequests       Permission `yaml:"pull-requests"`
	RepositoryProjects Permission `yaml:"repository-projects"`
	SecurityEvents     Permission `yaml:"security-events"`
	Statuses           Permission `yaml:"statuses"`
}

func AddPermissions(x *xorm.Engine) error {
	type ActionRunJob struct {
		Permissions Permissions `xorm:"JSON TEXT"`
	}

	return x.Sync(new(ActionRunJob))
}
