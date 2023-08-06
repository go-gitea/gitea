// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type Permission int

const (
	PermissionUnspecified Permission = iota
	PermissionNone
	PermissionRead
	PermissionWrite
)

// Per https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idpermissions
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

// WorkflowPermissions parses a workflow and returns
// a Permissions struct representing the permissions set
// at the workflow (i.e. file) level
func WorkflowPermissions(contents []byte) (Permissions, error) {
	p := struct {
		Permissions Permissions `yaml:"permissions"`
	}{}
	err := yaml.Unmarshal(contents, &p)
	return p.Permissions, err
}

// Given the contents of a workflow, JobPermissions
// returns a Permissions object representing the permissions
// of THE FIRST job in the file.
func JobPermissions(contents []byte) (Permissions, error) {
	p := struct {
		Jobs []struct {
			Permissions Permissions `yaml:"permissions"`
		} `yaml:"jobs"`
	}{}
	err := yaml.Unmarshal(contents, &p)
	if len(p.Jobs) > 0 {
		return p.Jobs[0].Permissions, err
	}
	return Permissions{}, errors.New("no jobs detected in workflow")
}

func (p *Permission) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data string
	if err := unmarshal(&data); err != nil {
		return err
	}

	switch data {
	case "none":
		*p = PermissionNone
	case "read":
		*p = PermissionRead
	case "write":
		*p = PermissionWrite
	default:
		return fmt.Errorf("invalid permission: %s", data)
	}

	return nil
}

// DefaultAccessPermissive is the default "permissive" set granted to actions on repositories
// per https://docs.github.com/en/actions/security-guides/automatic-token-authentication#permissions-for-the-github_token
// That page also lists a "metadata" permission that I can't find mentioned anywhere else.
// However, it seems to always have "read" permission, so it doesn't really matter.
// Interestingly, it doesn't list "Discussions", so we assume "write" for permissive and "none" for restricted.
var DefaultAccessPermissive = Permissions{
	Actions:            PermissionWrite,
	Checks:             PermissionWrite,
	Contents:           PermissionWrite,
	Deployments:        PermissionWrite,
	IDToken:            PermissionNone,
	Issues:             PermissionWrite,
	Discussions:        PermissionWrite,
	Packages:           PermissionWrite,
	Pages:              PermissionWrite,
	PullRequests:       PermissionWrite,
	RepositoryProjects: PermissionWrite,
	SecurityEvents:     PermissionWrite,
	Statuses:           PermissionWrite,
}

// DefaultAccessRestricted is the default "restrictive" set granted. See docs for
// DefaultAccessPermissive above.
//
// This is not currently used, since Gitea does not have a permissive/restricted setting.
var DefaultAccessRestricted = Permissions{
	Actions:            PermissionNone,
	Checks:             PermissionNone,
	Contents:           PermissionWrite,
	Deployments:        PermissionNone,
	IDToken:            PermissionNone,
	Issues:             PermissionNone,
	Discussions:        PermissionNone,
	Packages:           PermissionRead,
	Pages:              PermissionNone,
	PullRequests:       PermissionNone,
	RepositoryProjects: PermissionNone,
	SecurityEvents:     PermissionNone,
	Statuses:           PermissionNone,
}

var ReadAllPermissions = Permissions{
	Actions:            PermissionRead,
	Checks:             PermissionRead,
	Contents:           PermissionRead,
	Deployments:        PermissionRead,
	IDToken:            PermissionRead,
	Issues:             PermissionRead,
	Discussions:        PermissionRead,
	Packages:           PermissionRead,
	Pages:              PermissionRead,
	PullRequests:       PermissionRead,
	RepositoryProjects: PermissionRead,
	SecurityEvents:     PermissionRead,
	Statuses:           PermissionRead,
}

var WriteAllPermissions = Permissions{
	Actions:            PermissionWrite,
	Checks:             PermissionWrite,
	Contents:           PermissionWrite,
	Deployments:        PermissionWrite,
	IDToken:            PermissionWrite,
	Issues:             PermissionWrite,
	Discussions:        PermissionWrite,
	Packages:           PermissionWrite,
	Pages:              PermissionWrite,
	PullRequests:       PermissionWrite,
	RepositoryProjects: PermissionWrite,
	SecurityEvents:     PermissionWrite,
	Statuses:           PermissionWrite,
}

// FromYAML takes a yaml.Node representing a permissions
// definition and parses it into a Permissions struct
func (p *Permissions) FromYAML(rawPermissions *yaml.Node) error {
	switch rawPermissions.Kind {
	case yaml.ScalarNode:
		var val string
		err := rawPermissions.Decode(&val)
		if err != nil {
			return err
		}
		if val == "read-all" {
			*p = ReadAllPermissions
		}
		if val == "write-all" {
			*p = WriteAllPermissions
		}
		return fmt.Errorf("unexpected `permissions` value: %v", rawPermissions)
	case yaml.MappingNode:
		var perms Permissions
		err := rawPermissions.Decode(&perms)
		if err != nil {
			return err
		}
		return nil
	case 0:
		*p = Permissions{}
		return nil
	default:
		return fmt.Errorf("invalid permissions value: %v", rawPermissions)
	}
}

func merge[T comparable](a, b T) T {
	var zero T
	if a == zero {
		return b
	}
	return a
}

// Merge merges two Permission values
//
// Already set values take precedence over `other`.
// I.e. you want to call jobLevel.Permissions.Merge(topLevel.Permissions)
func (p *Permissions) Merge(other Permissions) {
	p.Actions = merge(p.Actions, other.Actions)
	p.Checks = merge(p.Checks, other.Checks)
	p.Contents = merge(p.Contents, other.Contents)
	p.Deployments = merge(p.Deployments, other.Deployments)
	p.IDToken = merge(p.IDToken, other.IDToken)
	p.Issues = merge(p.Issues, other.Issues)
	p.Discussions = merge(p.Discussions, other.Discussions)
	p.Packages = merge(p.Packages, other.Packages)
	p.Pages = merge(p.Pages, other.Pages)
	p.PullRequests = merge(p.PullRequests, other.PullRequests)
	p.RepositoryProjects = merge(p.RepositoryProjects, other.RepositoryProjects)
	p.SecurityEvents = merge(p.SecurityEvents, other.SecurityEvents)
	p.Statuses = merge(p.Statuses, other.Statuses)
}
