// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
)

// ErrWikiAlreadyExist represents a "WikiAlreadyExist" kind of error.
type ErrWikiAlreadyExist struct {
	Title string
}

// IsErrWikiAlreadyExist checks if an error is an ErrWikiAlreadyExist.
func IsErrWikiAlreadyExist(err error) bool {
	_, ok := err.(ErrWikiAlreadyExist)
	return ok
}

func (err ErrWikiAlreadyExist) Error() string {
	return fmt.Sprintf("wiki page already exists [title: %s]", err.Title)
}

func (err ErrWikiAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrWikiReservedName represents a reserved name error.
type ErrWikiReservedName struct {
	Title string
}

// IsErrWikiReservedName checks if an error is an ErrWikiReservedName.
func IsErrWikiReservedName(err error) bool {
	_, ok := err.(ErrWikiReservedName)
	return ok
}

func (err ErrWikiReservedName) Error() string {
	return "wiki title is reserved: " + err.Title
}

func (err ErrWikiReservedName) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrWikiInvalidFileName represents an invalid wiki file name.
type ErrWikiInvalidFileName struct {
	FileName string
}

// IsErrWikiInvalidFileName checks if an error is an ErrWikiInvalidFileName.
func IsErrWikiInvalidFileName(err error) bool {
	_, ok := err.(ErrWikiInvalidFileName)
	return ok
}

func (err ErrWikiInvalidFileName) Error() string {
	return "Invalid wiki filename: " + err.FileName
}

func (err ErrWikiInvalidFileName) Unwrap() error {
	return util.ErrInvalidArgument
}

// WikiCloneLink returns clone URLs of repository wiki.
func (repo *Repository) WikiCloneLink(ctx context.Context, doer *user_model.User) *CloneLink {
	return repo.cloneLink(ctx, doer, repo.Name+".wiki")
}

func RelativeWikiPath(ownerName, repoName string) string {
	return strings.ToLower(ownerName) + "/" + strings.ToLower(repoName) + ".wiki.git"
}

// WikiStorageRepo returns the storage repo for the wiki
// The wiki repository should have the same object format as the code repository
func (repo *Repository) WikiStorageRepo() StorageRepo {
	return StorageRepo(RelativeWikiPath(repo.OwnerName, repo.Name))
}
