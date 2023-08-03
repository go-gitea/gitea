// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

// ErrUserOwnRepos represents a "UserOwnRepos" kind of error.
type ErrUserOwnRepos struct {
	UID int64
}

// IsErrUserOwnRepos checks if an error is a ErrUserOwnRepos.
func IsErrUserOwnRepos(err error) bool {
	_, ok := err.(ErrUserOwnRepos)
	return ok
}

func (err ErrUserOwnRepos) Error() string {
	return fmt.Sprintf("user still has ownership of repositories [uid: %d]", err.UID)
}

// ErrUserHasOrgs represents a "UserHasOrgs" kind of error.
type ErrUserHasOrgs struct {
	UID int64
}

// IsErrUserHasOrgs checks if an error is a ErrUserHasOrgs.
func IsErrUserHasOrgs(err error) bool {
	_, ok := err.(ErrUserHasOrgs)
	return ok
}

func (err ErrUserHasOrgs) Error() string {
	return fmt.Sprintf("user still has membership of organizations [uid: %d]", err.UID)
}

// ErrUserOwnPackages notifies that the user (still) owns the packages.
type ErrUserOwnPackages struct {
	UID int64
}

// IsErrUserOwnPackages checks if an error is an ErrUserOwnPackages.
func IsErrUserOwnPackages(err error) bool {
	_, ok := err.(ErrUserOwnPackages)
	return ok
}

func (err ErrUserOwnPackages) Error() string {
	return fmt.Sprintf("user still has ownership of packages [uid: %d]", err.UID)
}

// ErrNoPendingRepoTransfer is an error type for repositories without a pending
// transfer request
type ErrNoPendingRepoTransfer struct {
	RepoID int64
}

func (err ErrNoPendingRepoTransfer) Error() string {
	return fmt.Sprintf("repository doesn't have a pending transfer [repo_id: %d]", err.RepoID)
}

// IsErrNoPendingTransfer is an error type when a repository has no pending
// transfers
func IsErrNoPendingTransfer(err error) bool {
	_, ok := err.(ErrNoPendingRepoTransfer)
	return ok
}

func (err ErrNoPendingRepoTransfer) Unwrap() error {
	return util.ErrNotExist
}

// ErrRepoTransferInProgress represents the state of a repository that has an
// ongoing transfer
type ErrRepoTransferInProgress struct {
	Uname string
	Name  string
}

// IsErrRepoTransferInProgress checks if an error is a ErrRepoTransferInProgress.
func IsErrRepoTransferInProgress(err error) bool {
	_, ok := err.(ErrRepoTransferInProgress)
	return ok
}

func (err ErrRepoTransferInProgress) Error() string {
	return fmt.Sprintf("repository is already being transferred [uname: %s, name: %s]", err.Uname, err.Name)
}

func (err ErrRepoTransferInProgress) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrInvalidCloneAddr represents a "InvalidCloneAddr" kind of error.
type ErrInvalidCloneAddr struct {
	Host               string
	IsURLError         bool
	IsInvalidPath      bool
	IsProtocolInvalid  bool
	IsPermissionDenied bool
	LocalPath          bool
}

// IsErrInvalidCloneAddr checks if an error is a ErrInvalidCloneAddr.
func IsErrInvalidCloneAddr(err error) bool {
	_, ok := err.(*ErrInvalidCloneAddr)
	return ok
}

func (err *ErrInvalidCloneAddr) Error() string {
	if err.IsInvalidPath {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided path is invalid", err.Host)
	}
	if err.IsProtocolInvalid {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided url protocol is not allowed", err.Host)
	}
	if err.IsPermissionDenied {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed.", err.Host)
	}
	if err.IsURLError {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided url is invalid", err.Host)
	}

	return fmt.Sprintf("migration/cloning from '%s' is not allowed", err.Host)
}

func (err *ErrInvalidCloneAddr) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrUpdateTaskNotExist represents a "UpdateTaskNotExist" kind of error.
type ErrUpdateTaskNotExist struct {
	UUID string
}

// IsErrUpdateTaskNotExist checks if an error is a ErrUpdateTaskNotExist.
func IsErrUpdateTaskNotExist(err error) bool {
	_, ok := err.(ErrUpdateTaskNotExist)
	return ok
}

func (err ErrUpdateTaskNotExist) Error() string {
	return fmt.Sprintf("update task does not exist [uuid: %s]", err.UUID)
}

func (err ErrUpdateTaskNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrInvalidTagName represents a "InvalidTagName" kind of error.
type ErrInvalidTagName struct {
	TagName string
}

// IsErrInvalidTagName checks if an error is a ErrInvalidTagName.
func IsErrInvalidTagName(err error) bool {
	_, ok := err.(ErrInvalidTagName)
	return ok
}

func (err ErrInvalidTagName) Error() string {
	return fmt.Sprintf("release tag name is not valid [tag_name: %s]", err.TagName)
}

func (err ErrInvalidTagName) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrProtectedTagName represents a "ProtectedTagName" kind of error.
type ErrProtectedTagName struct {
	TagName string
}

// IsErrProtectedTagName checks if an error is a ErrProtectedTagName.
func IsErrProtectedTagName(err error) bool {
	_, ok := err.(ErrProtectedTagName)
	return ok
}

func (err ErrProtectedTagName) Error() string {
	return fmt.Sprintf("release tag name is protected [tag_name: %s]", err.TagName)
}

func (err ErrProtectedTagName) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrRepoFileAlreadyExists represents a "RepoFileAlreadyExist" kind of error.
type ErrRepoFileAlreadyExists struct {
	Path string
}

// IsErrRepoFileAlreadyExists checks if an error is a ErrRepoFileAlreadyExists.
func IsErrRepoFileAlreadyExists(err error) bool {
	_, ok := err.(ErrRepoFileAlreadyExists)
	return ok
}

func (err ErrRepoFileAlreadyExists) Error() string {
	return fmt.Sprintf("repository file already exists [path: %s]", err.Path)
}

func (err ErrRepoFileAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrRepoFileDoesNotExist represents a "RepoFileDoesNotExist" kind of error.
type ErrRepoFileDoesNotExist struct {
	Path string
	Name string
}

// IsErrRepoFileDoesNotExist checks if an error is a ErrRepoDoesNotExist.
func IsErrRepoFileDoesNotExist(err error) bool {
	_, ok := err.(ErrRepoFileDoesNotExist)
	return ok
}

func (err ErrRepoFileDoesNotExist) Error() string {
	return fmt.Sprintf("repository file does not exist [path: %s]", err.Path)
}

func (err ErrRepoFileDoesNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrFilenameInvalid represents a "FilenameInvalid" kind of error.
type ErrFilenameInvalid struct {
	Path string
}

// IsErrFilenameInvalid checks if an error is an ErrFilenameInvalid.
func IsErrFilenameInvalid(err error) bool {
	_, ok := err.(ErrFilenameInvalid)
	return ok
}

func (err ErrFilenameInvalid) Error() string {
	return fmt.Sprintf("path contains a malformed path component [path: %s]", err.Path)
}

func (err ErrFilenameInvalid) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrUserCannotCommit represents "UserCannotCommit" kind of error.
type ErrUserCannotCommit struct {
	UserName string
}

// IsErrUserCannotCommit checks if an error is an ErrUserCannotCommit.
func IsErrUserCannotCommit(err error) bool {
	_, ok := err.(ErrUserCannotCommit)
	return ok
}

func (err ErrUserCannotCommit) Error() string {
	return fmt.Sprintf("user cannot commit to repo [user: %s]", err.UserName)
}

func (err ErrUserCannotCommit) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrFilePathInvalid represents a "FilePathInvalid" kind of error.
type ErrFilePathInvalid struct {
	Message string
	Path    string
	Name    string
	Type    git.EntryMode
}

// IsErrFilePathInvalid checks if an error is an ErrFilePathInvalid.
func IsErrFilePathInvalid(err error) bool {
	_, ok := err.(ErrFilePathInvalid)
	return ok
}

func (err ErrFilePathInvalid) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("path is invalid [path: %s]", err.Path)
}

func (err ErrFilePathInvalid) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrFilePathProtected represents a "FilePathProtected" kind of error.
type ErrFilePathProtected struct {
	Message string
	Path    string
}

// IsErrFilePathProtected checks if an error is an ErrFilePathProtected.
func IsErrFilePathProtected(err error) bool {
	_, ok := err.(ErrFilePathProtected)
	return ok
}

func (err ErrFilePathProtected) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("path is protected and can not be changed [path: %s]", err.Path)
}

func (err ErrFilePathProtected) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrDisallowedToMerge represents an error that a branch is protected and the current user is not allowed to modify it.
type ErrDisallowedToMerge struct {
	Reason string
}

// IsErrDisallowedToMerge checks if an error is an ErrDisallowedToMerge.
func IsErrDisallowedToMerge(err error) bool {
	_, ok := err.(ErrDisallowedToMerge)
	return ok
}

func (err ErrDisallowedToMerge) Error() string {
	return fmt.Sprintf("not allowed to merge [reason: %s]", err.Reason)
}

func (err ErrDisallowedToMerge) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrTagAlreadyExists represents an error that tag with such name already exists.
type ErrTagAlreadyExists struct {
	TagName string
}

// IsErrTagAlreadyExists checks if an error is an ErrTagAlreadyExists.
func IsErrTagAlreadyExists(err error) bool {
	_, ok := err.(ErrTagAlreadyExists)
	return ok
}

func (err ErrTagAlreadyExists) Error() string {
	return fmt.Sprintf("tag already exists [name: %s]", err.TagName)
}

func (err ErrTagAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrSHADoesNotMatch represents a "SHADoesNotMatch" kind of error.
type ErrSHADoesNotMatch struct {
	Path       string
	GivenSHA   string
	CurrentSHA string
}

// IsErrSHADoesNotMatch checks if an error is a ErrSHADoesNotMatch.
func IsErrSHADoesNotMatch(err error) bool {
	_, ok := err.(ErrSHADoesNotMatch)
	return ok
}

func (err ErrSHADoesNotMatch) Error() string {
	return fmt.Sprintf("sha does not match [given: %s, expected: %s]", err.GivenSHA, err.CurrentSHA)
}

// ErrSHANotFound represents a "SHADoesNotMatch" kind of error.
type ErrSHANotFound struct {
	SHA string
}

// IsErrSHANotFound checks if an error is a ErrSHANotFound.
func IsErrSHANotFound(err error) bool {
	_, ok := err.(ErrSHANotFound)
	return ok
}

func (err ErrSHANotFound) Error() string {
	return fmt.Sprintf("sha not found [%s]", err.SHA)
}

func (err ErrSHANotFound) Unwrap() error {
	return util.ErrNotExist
}

// ErrCommitIDDoesNotMatch represents a "CommitIDDoesNotMatch" kind of error.
type ErrCommitIDDoesNotMatch struct {
	GivenCommitID   string
	CurrentCommitID string
}

// IsErrCommitIDDoesNotMatch checks if an error is a ErrCommitIDDoesNotMatch.
func IsErrCommitIDDoesNotMatch(err error) bool {
	_, ok := err.(ErrCommitIDDoesNotMatch)
	return ok
}

func (err ErrCommitIDDoesNotMatch) Error() string {
	return fmt.Sprintf("file CommitID does not match [given: %s, expected: %s]", err.GivenCommitID, err.CurrentCommitID)
}

// ErrSHAOrCommitIDNotProvided represents a "SHAOrCommitIDNotProvided" kind of error.
type ErrSHAOrCommitIDNotProvided struct{}

// IsErrSHAOrCommitIDNotProvided checks if an error is a ErrSHAOrCommitIDNotProvided.
func IsErrSHAOrCommitIDNotProvided(err error) bool {
	_, ok := err.(ErrSHAOrCommitIDNotProvided)
	return ok
}

func (err ErrSHAOrCommitIDNotProvided) Error() string {
	return "a SHA or commit ID must be proved when updating a file"
}

// ErrInvalidMergeStyle represents an error if merging with disabled merge strategy
type ErrInvalidMergeStyle struct {
	ID    int64
	Style repo_model.MergeStyle
}

// IsErrInvalidMergeStyle checks if an error is a ErrInvalidMergeStyle.
func IsErrInvalidMergeStyle(err error) bool {
	_, ok := err.(ErrInvalidMergeStyle)
	return ok
}

func (err ErrInvalidMergeStyle) Error() string {
	return fmt.Sprintf("merge strategy is not allowed or is invalid [repo_id: %d, strategy: %s]",
		err.ID, err.Style)
}

func (err ErrInvalidMergeStyle) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrMergeConflicts represents an error if merging fails with a conflict
type ErrMergeConflicts struct {
	Style  repo_model.MergeStyle
	StdOut string
	StdErr string
	Err    error
}

// IsErrMergeConflicts checks if an error is a ErrMergeConflicts.
func IsErrMergeConflicts(err error) bool {
	_, ok := err.(ErrMergeConflicts)
	return ok
}

func (err ErrMergeConflicts) Error() string {
	return fmt.Sprintf("Merge Conflict Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// ErrMergeUnrelatedHistories represents an error if merging fails due to unrelated histories
type ErrMergeUnrelatedHistories struct {
	Style  repo_model.MergeStyle
	StdOut string
	StdErr string
	Err    error
}

// IsErrMergeUnrelatedHistories checks if an error is a ErrMergeUnrelatedHistories.
func IsErrMergeUnrelatedHistories(err error) bool {
	_, ok := err.(ErrMergeUnrelatedHistories)
	return ok
}

func (err ErrMergeUnrelatedHistories) Error() string {
	return fmt.Sprintf("Merge UnrelatedHistories Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// ErrRebaseConflicts represents an error if rebase fails with a conflict
type ErrRebaseConflicts struct {
	Style     repo_model.MergeStyle
	CommitSHA string
	StdOut    string
	StdErr    string
	Err       error
}

// IsErrRebaseConflicts checks if an error is a ErrRebaseConflicts.
func IsErrRebaseConflicts(err error) bool {
	_, ok := err.(ErrRebaseConflicts)
	return ok
}

func (err ErrRebaseConflicts) Error() string {
	return fmt.Sprintf("Rebase Error: %v: Whilst Rebasing: %s\n%s\n%s", err.Err, err.CommitSHA, err.StdErr, err.StdOut)
}

// ErrPullRequestHasMerged represents a "PullRequestHasMerged"-error
type ErrPullRequestHasMerged struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestHasMerged checks if an error is a ErrPullRequestHasMerged.
func IsErrPullRequestHasMerged(err error) bool {
	_, ok := err.(ErrPullRequestHasMerged)
	return ok
}

// Error does pretty-printing :D
func (err ErrPullRequestHasMerged) Error() string {
	return fmt.Sprintf("pull request has merged [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}
