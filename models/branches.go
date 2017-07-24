// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"
)

const (
	// ProtectedBranchRepoID protected Repo ID
	ProtectedBranchRepoID = "GITEA_REPO_ID"
)

// ProtectedBranch struct
type ProtectedBranch struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"UNIQUE(s)"`
	BranchName  string `xorm:"UNIQUE(s)"`
	CanPush     bool
	Created     time.Time `xorm:"-"`
	CreatedUnix int64
	Updated     time.Time `xorm:"-"`
	UpdatedUnix int64
}

// BeforeInsert before protected branch insert create and update time
func (protectBranch *ProtectedBranch) BeforeInsert() {
	protectBranch.CreatedUnix = time.Now().Unix()
	protectBranch.UpdatedUnix = protectBranch.CreatedUnix
}

// BeforeUpdate before protected branch update time
func (protectBranch *ProtectedBranch) BeforeUpdate() {
	protectBranch.UpdatedUnix = time.Now().Unix()
}

// GetProtectedBranchByRepoID getting protected branch by repo ID
func GetProtectedBranchByRepoID(RepoID int64) ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Where("repo_id = ?", RepoID).Desc("updated_unix").Find(&protectedBranches)
}

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(repoID int64, BranchName string) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{RepoID: repoID, BranchName: strings.ToLower(BranchName)}
	has, err := x.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// GetProtectedBranches get all protected branches
func (repo *Repository) GetProtectedBranches() ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Find(&protectedBranches, &ProtectedBranch{RepoID: repo.ID})
}

// IsProtectedBranch checks if branch is protected
func (repo *Repository) IsProtectedBranch(branchName string) (bool, error) {
	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Get(protectedBranch)
	if err != nil {
		return true, err
	} else if has {
		return true, nil
	}

	return false, nil
}

// AddProtectedBranch add protection to branch
func (repo *Repository) AddProtectedBranch(branchName string, canPush bool) error {
	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Get(protectedBranch)
	if err != nil {
		return err
	} else if has {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}
	protectedBranch.CanPush = canPush
	if _, err = sess.InsertOne(protectedBranch); err != nil {
		return err
	}

	return sess.Commit()
}

// ChangeProtectedBranch access mode sets new access mode for the ProtectedBranch.
func (repo *Repository) ChangeProtectedBranch(id int64, canPush bool) error {
	ProtectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}
	has, err := x.Get(ProtectedBranch)
	if err != nil {
		return fmt.Errorf("get ProtectedBranch: %v", err)
	} else if !has {
		return nil
	}

	if ProtectedBranch.CanPush == canPush {
		return nil
	}
	ProtectedBranch.CanPush = canPush

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Id(ProtectedBranch.ID).AllCols().Update(ProtectedBranch); err != nil {
		return fmt.Errorf("update ProtectedBranch: %v", err)
	}

	return sess.Commit()
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func (repo *Repository) DeleteProtectedBranch(id int64) (err error) {
	protectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// newProtectedBranch insert one queue
func newProtectedBranch(protectedBranch *ProtectedBranch) error {
	_, err := x.InsertOne(protectedBranch)
	return err
}

// UpdateProtectedBranch update queue
func UpdateProtectedBranch(protectedBranch *ProtectedBranch) error {
	_, err := x.Update(protectedBranch)
	return err
}
