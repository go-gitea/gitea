// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// ActionCrossRepoAccess represents cross-repository access rules
type ActionCrossRepoAccess struct {
	ID           int64  `xorm:"pk autoincr"`
	OrgID        int64  `xorm:"INDEX NOT NULL"`
	SourceRepoID int64  `xorm:"INDEX NOT NULL"` // Repo that wants access
	TargetRepoID int64  `xorm:"INDEX NOT NULL"` // Repo being accessed
	
	// Access level: 0=none, 1=read, 2=write
	AccessLevel int `xorm:"NOT NULL DEFAULT 0"`
	
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// PackageRepoLink links packages to repositories
type PackageRepoLink struct {
	ID        int64  `xorm:"pk autoincr"`
	PackageID int64  `xorm:"INDEX NOT NULL"`
	RepoID    int64  `xorm:"INDEX NOT NULL"`
	
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(ActionCrossRepoAccess))
	db.RegisterModel(new(PackageRepoLink))
}

// ListCrossRepoAccessRules lists all cross-repo access rules for an organization
func ListCrossRepoAccessRules(ctx context.Context, orgID int64) ([]*ActionCrossRepoAccess, error) {
	rules := make([]*ActionCrossRepoAccess, 0, 10)
	err := db.GetEngine(ctx).
		Where("org_id = ?", orgID).
		Find(&rules)
	return rules, err
}

// GetCrossRepoAccessByID retrieves a specific cross-repo access rule
func GetCrossRepoAccessByID(ctx context.Context, id int64) (*ActionCrossRepoAccess, error) {
	rule := &ActionCrossRepoAccess{ID: id}
	has, err := db.GetEngine(ctx).Get(rule)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "cross_repo_access", ID: id}
	}
	return rule, nil
}

// CheckCrossRepoAccess checks if source repo can access target repo
// Returns access level: 0=none, 1=read, 2=write
func CheckCrossRepoAccess(ctx context.Context, sourceRepoID, targetRepoID int64) (int, error) {
	// If accessing same repo, always allow
	// This is an optimization - no need to check rules
	if sourceRepoID == targetRepoID {
		return 2, nil // Full access to own repo
	}
	
	rule := &ActionCrossRepoAccess{}
	has, err := db.GetEngine(ctx).
		Where("source_repo_id = ? AND target_repo_id = ?", sourceRepoID, targetRepoID).
		Get(rule)
	
	if err != nil {
		return 0, err
	}
	
	if !has {
		// No rule found - deny access by default (secure default)
		// This is intentional - cross-repo access must be explicitly granted
		return 0, nil
	}
	
	return rule.AccessLevel, nil
}

// CreateCrossRepoAccess creates a new cross-repo access rule
func CreateCrossRepoAccess(ctx context.Context, rule *ActionCrossRepoAccess) error {
	// Check if rule already exists
	// We don't want duplicate rules for the same source-target pair
	existing := &ActionCrossRepoAccess{}
	has, err := db.GetEngine(ctx).
		Where("org_id = ? AND source_repo_id = ? AND target_repo_id = ?",
			rule.OrgID, rule.SourceRepoID, rule.TargetRepoID).
		Get(existing)
	
	if err != nil {
		return err
	}
	
	if has {
		// Update existing rule instead of creating duplicate
		existing.AccessLevel = rule.AccessLevel
		_, err = db.GetEngine(ctx).ID(existing.ID).Update(existing)
		return err
	}
	
	// Create new rule
	_, err = db.GetEngine(ctx).Insert(rule)
	return err
}

// DeleteCrossRepoAccess deletes a cross-repo access rule
func DeleteCrossRepoAccess(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&ActionCrossRepoAccess{})
	return err
}

// Package-Repository Link Functions

// LinkPackageToRepo creates a link between a package and repository
// This allows Actions from that repository to access the package
func LinkPackageToRepo(ctx context.Context, packageID, repoID int64) error {
	// Check if link already exists
	existing := &PackageRepoLink{}
	has, err := db.GetEngine(ctx).
		Where("package_id = ? AND repo_id = ?", packageID, repoID).
		Get(existing)
	
	if err != nil {
		return err
	}
	
	if has {
		// Already linked - this is idempotent
		return nil
	}
	
	link := &PackageRepoLink{
		PackageID: packageID,
		RepoID:    repoID,
	}
	
	_, err = db.GetEngine(ctx).Insert(link)
	return err
}

// UnlinkPackageFromRepo removes a link between package and repository
func UnlinkPackageFromRepo(ctx context.Context, packageID, repoID int64) error {
	_, err := db.GetEngine(ctx).
		Where("package_id = ? AND repo_id = ?", packageID, repoID).
		Delete(&PackageRepoLink{})
	return err
}

// IsPackageLinkedToRepo checks if a package is linked to a repository
func IsPackageLinkedToRepo(ctx context.Context, packageID, repoID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("package_id = ? AND repo_id = ?", packageID, repoID).
		Exist(&PackageRepoLink{})
}

// GetPackageLinkedRepos returns all repos linked to a package
func GetPackageLinkedRepos(ctx context.Context, packageID int64) ([]int64, error) {
	links := make([]*PackageRepoLink, 0, 10)
	err := db.GetEngine(ctx).
		Where("package_id = ?", packageID).
		Find(&links)
	
	if err != nil {
		return nil, err
	}
	
	repoIDs := make([]int64, len(links))
	for i, link := range links {
		repoIDs[i] = link.RepoID
	}
	
	return repoIDs, nil
}

// GetRepoLinkedPackages returns all packages linked to a repository
func GetRepoLinkedPackages(ctx context.Context, repoID int64) ([]int64, error) {
	links := make([]*PackageRepoLink, 0, 10)
	err := db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		Find(&links)
	
	if err != nil {
		return nil, err
	}
	
	packageIDs := make([]int64, len(links))
	for i, link := range links {
		packageIDs[i] = link.PackageID
	}
	
	return packageIDs, nil
}

// CanAccessPackage checks if a repository's Actions can access a package
//
// Access is granted if ANY of these conditions are met:
// 1. Package is directly linked to the repository
// 2. Package is linked to another repo that allows cross-repo access to this repo
//
// This implements the security model from:
// https://github.com/go-gitea/gitea/issues/24635
func CanAccessPackage(ctx context.Context, repoID, packageID int64, needWrite bool) (bool, error) {
	// Check direct linking
	linked, err := IsPackageLinkedToRepo(ctx, packageID, repoID)
	if err != nil {
		return false, err
	}
	
	if linked {
		// Package is directly linked - access granted!
		// Note: Direct linking grants both read and write access
		// This is intentional - if you link a package to your repo,
		// you probably want to be able to publish to it
		return true, nil
	}
	
	// Check indirect access via cross-repo rules
	// Get all repos linked to this package
	linkedRepos, err := GetPackageLinkedRepos(ctx, packageID)
	if err != nil {
		return false, err
	}
	
	// Check if we have cross-repo access to any of those repos
	for _, targetRepoID := range linkedRepos {
		accessLevel, err := CheckCrossRepoAccess(ctx, repoID, targetRepoID)
		if err != nil {
			continue // Skip on error, check next repo
		}
		
		if accessLevel > 0 {
			// We have some level of access to the target repo
			if needWrite && accessLevel < 2 {
				// We need write but only have read - not enough
				continue
			}
			
			// Access granted via cross-repo rule!
			return true, nil
		}
	}
	
	// No access found
	return false, nil
}
