// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ErrRedirectNotExist represents a "RedirectNotExist" kind of error.
type ErrRedirectNotExist struct {
	OwnerID  int64
	RepoName string
}

// IsErrRedirectNotExist check if an error is an ErrRepoRedirectNotExist.
func IsErrRedirectNotExist(err error) bool {
	_, ok := err.(ErrRedirectNotExist)
	return ok
}

func (err ErrRedirectNotExist) Error() string {
	return fmt.Sprintf("repository redirect does not exist [uid: %d, name: %s]", err.OwnerID, err.RepoName)
}

func (err ErrRedirectNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Redirect represents that a repo name should be redirected to another
type Redirect struct {
	ID             int64  `xorm:"pk autoincr"`
	OwnerID        int64  `xorm:"UNIQUE(s)"`
	LowerName      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RedirectRepoID int64  // repoID to redirect to
}

// TableName represents real table name in database
func (Redirect) TableName() string {
	return "repo_redirect"
}

func init() {
	db.RegisterModel(new(Redirect))
}

// LookupRedirect look up if a repository has a redirect name
func LookupRedirect(ctx context.Context, ownerID int64, repoName string) (int64, error) {
	repoName = strings.ToLower(repoName)
	redirect := &Redirect{OwnerID: ownerID, LowerName: repoName}
	if has, err := db.GetEngine(ctx).Get(redirect); err != nil {
		return 0, err
	} else if !has {
		return 0, ErrRedirectNotExist{OwnerID: ownerID, RepoName: repoName}
	}
	return redirect.RedirectRepoID, nil
}

// NewRedirect create a new repo redirect
func NewRedirect(ctx context.Context, ownerID, repoID int64, oldRepoName, newRepoName string) error {
	oldRepoName = strings.ToLower(oldRepoName)
	newRepoName = strings.ToLower(newRepoName)

	if err := DeleteRedirect(ctx, ownerID, newRepoName); err != nil {
		return err
	}

	return db.Insert(ctx, &Redirect{
		OwnerID:        ownerID,
		LowerName:      oldRepoName,
		RedirectRepoID: repoID,
	})
}

// DeleteRedirect delete any redirect from the specified repo name to
// anything else
func DeleteRedirect(ctx context.Context, ownerID int64, repoName string) error {
	repoName = strings.ToLower(repoName)
	_, err := db.GetEngine(ctx).Delete(&Redirect{OwnerID: ownerID, LowerName: repoName})
	return err
}
