// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
)

// ErrNotExist represents a non-exist error.
type ErrNotExist struct {
	ID int64
}

// IsErrNotExist checks if an error is an ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("record does not exist [id: %d]", err.ID)
}

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

//  __      __.__ __   .__
// /  \    /  \__|  | _|__|
// \   \/\/   /  |  |/ /  |
//  \        /|  |    <|  |
//   \__/\  / |__|__|_ \__|
//        \/          \/

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
	return fmt.Sprintf("wiki title is reserved: %s", err.Title)
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
	return fmt.Sprintf("Invalid wiki filename: %s", err.FileName)
}

//    _____                                   ___________     __
//   /  _  \   ____  ____  ____   ______ _____\__    ___/___ |  | __ ____   ____
//  /  /_\  \_/ ___\/ ___\/ __ \ /  ___//  ___/ |    | /  _ \|  |/ // __ \ /    \
// /    |    \  \__\  \__\  ___/ \___ \ \___ \  |    |(  <_> )    <\  ___/|   |  \
// \____|__  /\___  >___  >___  >____  >____  > |____| \____/|__|_ \\___  >___|  /
//         \/     \/    \/    \/     \/     \/                    \/    \/     \/

// ErrAccessTokenNotExist represents a "AccessTokenNotExist" kind of error.
type ErrAccessTokenNotExist struct {
	Token string
}

// IsErrAccessTokenNotExist checks if an error is a ErrAccessTokenNotExist.
func IsErrAccessTokenNotExist(err error) bool {
	_, ok := err.(ErrAccessTokenNotExist)
	return ok
}

func (err ErrAccessTokenNotExist) Error() string {
	return fmt.Sprintf("access token does not exist [sha: %s]", err.Token)
}

// ErrAccessTokenEmpty represents a "AccessTokenEmpty" kind of error.
type ErrAccessTokenEmpty struct{}

// IsErrAccessTokenEmpty checks if an error is a ErrAccessTokenEmpty.
func IsErrAccessTokenEmpty(err error) bool {
	_, ok := err.(ErrAccessTokenEmpty)
	return ok
}

func (err ErrAccessTokenEmpty) Error() string {
	return "access token is empty"
}

//.____   ____________________
//|    |  \_   _____/   _____/
//|    |   |    __) \_____  \
//|    |___|     \  /        \
//|_______ \___  / /_______  /
//        \/   \/          \/

// ErrLFSLockNotExist represents a "LFSLockNotExist" kind of error.
type ErrLFSLockNotExist struct {
	ID     int64
	RepoID int64
	Path   string
}

// IsErrLFSLockNotExist checks if an error is a ErrLFSLockNotExist.
func IsErrLFSLockNotExist(err error) bool {
	_, ok := err.(ErrLFSLockNotExist)
	return ok
}

func (err ErrLFSLockNotExist) Error() string {
	return fmt.Sprintf("lfs lock does not exist [id: %d, rid: %d, path: %s]", err.ID, err.RepoID, err.Path)
}

// ErrLFSUnauthorizedAction represents a "LFSUnauthorizedAction" kind of error.
type ErrLFSUnauthorizedAction struct {
	RepoID   int64
	UserName string
	Mode     perm.AccessMode
}

// IsErrLFSUnauthorizedAction checks if an error is a ErrLFSUnauthorizedAction.
func IsErrLFSUnauthorizedAction(err error) bool {
	_, ok := err.(ErrLFSUnauthorizedAction)
	return ok
}

func (err ErrLFSUnauthorizedAction) Error() string {
	if err.Mode == perm.AccessModeWrite {
		return fmt.Sprintf("User %s doesn't have write access for lfs lock [rid: %d]", err.UserName, err.RepoID)
	}
	return fmt.Sprintf("User %s doesn't have read access for lfs lock [rid: %d]", err.UserName, err.RepoID)
}

// ErrLFSLockAlreadyExist represents a "LFSLockAlreadyExist" kind of error.
type ErrLFSLockAlreadyExist struct {
	RepoID int64
	Path   string
}

// IsErrLFSLockAlreadyExist checks if an error is a ErrLFSLockAlreadyExist.
func IsErrLFSLockAlreadyExist(err error) bool {
	_, ok := err.(ErrLFSLockAlreadyExist)
	return ok
}

func (err ErrLFSLockAlreadyExist) Error() string {
	return fmt.Sprintf("lfs lock already exists [rid: %d, path: %s]", err.RepoID, err.Path)
}

// ErrLFSFileLocked represents a "LFSFileLocked" kind of error.
type ErrLFSFileLocked struct {
	RepoID   int64
	Path     string
	UserName string
}

// IsErrLFSFileLocked checks if an error is a ErrLFSFileLocked.
func IsErrLFSFileLocked(err error) bool {
	_, ok := err.(ErrLFSFileLocked)
	return ok
}

func (err ErrLFSFileLocked) Error() string {
	return fmt.Sprintf("File is lfs locked [repo: %d, locked by: %s, path: %s]", err.RepoID, err.UserName, err.Path)
}

// ErrNoPendingRepoTransfer is an error type for repositories without a pending
// transfer request
type ErrNoPendingRepoTransfer struct {
	RepoID int64
}

func (e ErrNoPendingRepoTransfer) Error() string {
	return fmt.Sprintf("repository doesn't have a pending transfer [repo_id: %d]", e.RepoID)
}

// IsErrNoPendingTransfer is an error type when a repository has no pending
// transfers
func IsErrNoPendingTransfer(err error) bool {
	_, ok := err.(ErrNoPendingRepoTransfer)
	return ok
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

// ErrForkAlreadyExist represents a "ForkAlreadyExist" kind of error.
type ErrForkAlreadyExist struct {
	Uname    string
	RepoName string
	ForkName string
}

// IsErrForkAlreadyExist checks if an error is an ErrForkAlreadyExist.
func IsErrForkAlreadyExist(err error) bool {
	_, ok := err.(ErrForkAlreadyExist)
	return ok
}

func (err ErrForkAlreadyExist) Error() string {
	return fmt.Sprintf("repository is already forked by user [uname: %s, repo path: %s, fork path: %s]", err.Uname, err.RepoName, err.ForkName)
}

// ErrInvalidCloneAddr represents a "InvalidCloneAddr" kind of error.
type ErrInvalidCloneAddr struct {
	Host               string
	IsURLError         bool
	IsInvalidPath      bool
	IsProtocolInvalid  bool
	IsPermissionDenied bool
	LocalPath          bool
	NotResolvedIP      bool
}

// IsErrInvalidCloneAddr checks if an error is a ErrInvalidCloneAddr.
func IsErrInvalidCloneAddr(err error) bool {
	_, ok := err.(*ErrInvalidCloneAddr)
	return ok
}

func (err *ErrInvalidCloneAddr) Error() string {
	if err.NotResolvedIP {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: unknown hostname", err.Host)
	}
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

// ErrReleaseAlreadyExist represents a "ReleaseAlreadyExist" kind of error.
type ErrReleaseAlreadyExist struct {
	TagName string
}

// IsErrReleaseAlreadyExist checks if an error is a ErrReleaseAlreadyExist.
func IsErrReleaseAlreadyExist(err error) bool {
	_, ok := err.(ErrReleaseAlreadyExist)
	return ok
}

func (err ErrReleaseAlreadyExist) Error() string {
	return fmt.Sprintf("release tag already exist [tag_name: %s]", err.TagName)
}

// ErrReleaseNotExist represents a "ReleaseNotExist" kind of error.
type ErrReleaseNotExist struct {
	ID      int64
	TagName string
}

// IsErrReleaseNotExist checks if an error is a ErrReleaseNotExist.
func IsErrReleaseNotExist(err error) bool {
	_, ok := err.(ErrReleaseNotExist)
	return ok
}

func (err ErrReleaseNotExist) Error() string {
	return fmt.Sprintf("release tag does not exist [id: %d, tag_name: %s]", err.ID, err.TagName)
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

// ErrUserDoesNotHaveAccessToRepo represets an error where the user doesn't has access to a given repo.
type ErrUserDoesNotHaveAccessToRepo struct {
	UserID   int64
	RepoName string
}

// IsErrUserDoesNotHaveAccessToRepo checks if an error is a ErrRepoFileAlreadyExists.
func IsErrUserDoesNotHaveAccessToRepo(err error) bool {
	_, ok := err.(ErrUserDoesNotHaveAccessToRepo)
	return ok
}

func (err ErrUserDoesNotHaveAccessToRepo) Error() string {
	return fmt.Sprintf("user doesn't have access to repo [user_id: %d, repo_name: %s]", err.UserID, err.RepoName)
}

// __________                             .__
// \______   \____________    ____   ____ |  |__
//  |    |  _/\_  __ \__  \  /    \_/ ___\|  |  \
//  |    |   \ |  | \// __ \|   |  \  \___|   Y  \
//  |______  / |__|  (____  /___|  /\___  >___|  /
//         \/             \/     \/     \/     \/

// ErrBranchDoesNotExist represents an error that branch with such name does not exist.
type ErrBranchDoesNotExist struct {
	BranchName string
}

// IsErrBranchDoesNotExist checks if an error is an ErrBranchDoesNotExist.
func IsErrBranchDoesNotExist(err error) bool {
	_, ok := err.(ErrBranchDoesNotExist)
	return ok
}

func (err ErrBranchDoesNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [name: %s]", err.BranchName)
}

// ErrBranchAlreadyExists represents an error that branch with such name already exists.
type ErrBranchAlreadyExists struct {
	BranchName string
}

// IsErrBranchAlreadyExists checks if an error is an ErrBranchAlreadyExists.
func IsErrBranchAlreadyExists(err error) bool {
	_, ok := err.(ErrBranchAlreadyExists)
	return ok
}

func (err ErrBranchAlreadyExists) Error() string {
	return fmt.Sprintf("branch already exists [name: %s]", err.BranchName)
}

// ErrBranchNameConflict represents an error that branch name conflicts with other branch.
type ErrBranchNameConflict struct {
	BranchName string
}

// IsErrBranchNameConflict checks if an error is an ErrBranchNameConflict.
func IsErrBranchNameConflict(err error) bool {
	_, ok := err.(ErrBranchNameConflict)
	return ok
}

func (err ErrBranchNameConflict) Error() string {
	return fmt.Sprintf("branch conflicts with existing branch [name: %s]", err.BranchName)
}

// ErrBranchesEqual represents an error that branch name conflicts with other branch.
type ErrBranchesEqual struct {
	BaseBranchName string
	HeadBranchName string
}

// IsErrBranchesEqual checks if an error is an ErrBranchesEqual.
func IsErrBranchesEqual(err error) bool {
	_, ok := err.(ErrBranchesEqual)
	return ok
}

func (err ErrBranchesEqual) Error() string {
	return fmt.Sprintf("branches are equal [head: %sm base: %s]", err.HeadBranchName, err.BaseBranchName)
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

// .___
// |   | ______ ________ __   ____
// |   |/  ___//  ___/  |  \_/ __ \
// |   |\___ \ \___ \|  |  /\  ___/
// |___/____  >____  >____/  \___  >
//          \/     \/            \/

// ErrIssueNotExist represents a "IssueNotExist" kind of error.
type ErrIssueNotExist struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrIssueNotExist checks if an error is a ErrIssueNotExist.
func IsErrIssueNotExist(err error) bool {
	_, ok := err.(ErrIssueNotExist)
	return ok
}

func (err ErrIssueNotExist) Error() string {
	return fmt.Sprintf("issue does not exist [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

// ErrIssueIsClosed represents a "IssueIsClosed" kind of error.
type ErrIssueIsClosed struct {
	ID     int64
	RepoID int64
	Index  int64
}

// IsErrIssueIsClosed checks if an error is a ErrIssueNotExist.
func IsErrIssueIsClosed(err error) bool {
	_, ok := err.(ErrIssueIsClosed)
	return ok
}

func (err ErrIssueIsClosed) Error() string {
	return fmt.Sprintf("issue is closed [id: %d, repo_id: %d, index: %d]", err.ID, err.RepoID, err.Index)
}

// ErrNewIssueInsert is used when the INSERT statement in newIssue fails
type ErrNewIssueInsert struct {
	OriginalError error
}

// IsErrNewIssueInsert checks if an error is a ErrNewIssueInsert.
func IsErrNewIssueInsert(err error) bool {
	_, ok := err.(ErrNewIssueInsert)
	return ok
}

func (err ErrNewIssueInsert) Error() string {
	return err.OriginalError.Error()
}

// ErrIssueWasClosed is used when close a closed issue
type ErrIssueWasClosed struct {
	ID    int64
	Index int64
}

// IsErrIssueWasClosed checks if an error is a ErrIssueWasClosed.
func IsErrIssueWasClosed(err error) bool {
	_, ok := err.(ErrIssueWasClosed)
	return ok
}

func (err ErrIssueWasClosed) Error() string {
	return fmt.Sprintf("Issue [%d] %d was already closed", err.ID, err.Index)
}

// ErrPullWasClosed is used close a closed pull request
type ErrPullWasClosed struct {
	ID    int64
	Index int64
}

// IsErrPullWasClosed checks if an error is a ErrErrPullWasClosed.
func IsErrPullWasClosed(err error) bool {
	_, ok := err.(ErrPullWasClosed)
	return ok
}

func (err ErrPullWasClosed) Error() string {
	return fmt.Sprintf("Pull request [%d] %d was already closed", err.ID, err.Index)
}

// __________      .__  .__ __________                                     __
// \______   \__ __|  | |  |\______   \ ____  ________ __   ____   _______/  |_
//  |     ___/  |  \  | |  | |       _// __ \/ ____/  |  \_/ __ \ /  ___/\   __\
//  |    |   |  |  /  |_|  |_|    |   \  ___< <_|  |  |  /\  ___/ \___ \  |  |
//  |____|   |____/|____/____/____|_  /\___  >__   |____/  \___  >____  > |__|
//                                  \/     \/   |__|           \/     \/

// ErrPullRequestNotExist represents a "PullRequestNotExist" kind of error.
type ErrPullRequestNotExist struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestNotExist checks if an error is a ErrPullRequestNotExist.
func IsErrPullRequestNotExist(err error) bool {
	_, ok := err.(ErrPullRequestNotExist)
	return ok
}

func (err ErrPullRequestNotExist) Error() string {
	return fmt.Sprintf("pull request does not exist [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

// ErrPullRequestAlreadyExists represents a "PullRequestAlreadyExists"-error
type ErrPullRequestAlreadyExists struct {
	ID         int64
	IssueID    int64
	HeadRepoID int64
	BaseRepoID int64
	HeadBranch string
	BaseBranch string
}

// IsErrPullRequestAlreadyExists checks if an error is a ErrPullRequestAlreadyExists.
func IsErrPullRequestAlreadyExists(err error) bool {
	_, ok := err.(ErrPullRequestAlreadyExists)
	return ok
}

// Error does pretty-printing :D
func (err ErrPullRequestAlreadyExists) Error() string {
	return fmt.Sprintf("pull request already exists for these targets [id: %d, issue_id: %d, head_repo_id: %d, base_repo_id: %d, head_branch: %s, base_branch: %s]",
		err.ID, err.IssueID, err.HeadRepoID, err.BaseRepoID, err.HeadBranch, err.BaseBranch)
}

// ErrPullRequestHeadRepoMissing represents a "ErrPullRequestHeadRepoMissing" error
type ErrPullRequestHeadRepoMissing struct {
	ID         int64
	HeadRepoID int64
}

// IsErrErrPullRequestHeadRepoMissing checks if an error is a ErrPullRequestHeadRepoMissing.
func IsErrErrPullRequestHeadRepoMissing(err error) bool {
	_, ok := err.(ErrPullRequestHeadRepoMissing)
	return ok
}

// Error does pretty-printing :D
func (err ErrPullRequestHeadRepoMissing) Error() string {
	return fmt.Sprintf("pull request head repo missing [id: %d, head_repo_id: %d]",
		err.ID, err.HeadRepoID)
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

// _________                                       __
// \_   ___ \  ____   _____   _____   ____   _____/  |_
// /    \  \/ /  _ \ /     \ /     \_/ __ \ /    \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ___/|   |  \  |
//  \______  /\____/|__|_|  /__|_|  /\___  >___|  /__|
//         \/             \/      \/     \/     \/

// ErrCommentNotExist represents a "CommentNotExist" kind of error.
type ErrCommentNotExist struct {
	ID      int64
	IssueID int64
}

// IsErrCommentNotExist checks if an error is a ErrCommentNotExist.
func IsErrCommentNotExist(err error) bool {
	_, ok := err.(ErrCommentNotExist)
	return ok
}

func (err ErrCommentNotExist) Error() string {
	return fmt.Sprintf("comment does not exist [id: %d, issue_id: %d]", err.ID, err.IssueID)
}

//  _________ __                                __         .__
//  /   _____//  |_  ____ ________  _  _______ _/  |_  ____ |  |__
//  \_____  \\   __\/  _ \\____ \ \/ \/ /\__  \\   __\/ ___\|  |  \
//  /        \|  | (  <_> )  |_> >     /  / __ \|  | \  \___|   Y  \
//  /_______  /|__|  \____/|   __/ \/\_/  (____  /__|  \___  >___|  /
// \/             |__|                \/          \/     \/

// ErrStopwatchNotExist represents a "Stopwatch Not Exist" kind of error.
type ErrStopwatchNotExist struct {
	ID int64
}

// IsErrStopwatchNotExist checks if an error is a ErrStopwatchNotExist.
func IsErrStopwatchNotExist(err error) bool {
	_, ok := err.(ErrStopwatchNotExist)
	return ok
}

func (err ErrStopwatchNotExist) Error() string {
	return fmt.Sprintf("stopwatch does not exist [id: %d]", err.ID)
}

// ___________                     __              .______________.__
// \__    ___/___________    ____ |  | __ ____   __| _/\__    ___/|__| _____   ____
// |    |  \_  __ \__  \ _/ ___\|  |/ // __ \ / __ |   |    |   |  |/     \_/ __ \
// |    |   |  | \// __ \\  \___|    <\  ___// /_/ |   |    |   |  |  Y Y  \  ___/
// |____|   |__|  (____  /\___  >__|_ \\___  >____ |   |____|   |__|__|_|  /\___  >
// \/     \/     \/    \/     \/                     \/     \/

// ErrTrackedTimeNotExist represents a "TrackedTime Not Exist" kind of error.
type ErrTrackedTimeNotExist struct {
	ID int64
}

// IsErrTrackedTimeNotExist checks if an error is a ErrTrackedTimeNotExist.
func IsErrTrackedTimeNotExist(err error) bool {
	_, ok := err.(ErrTrackedTimeNotExist)
	return ok
}

func (err ErrTrackedTimeNotExist) Error() string {
	return fmt.Sprintf("tracked time does not exist [id: %d]", err.ID)
}

// .____          ___.          .__
// |    |   _____ \_ |__   ____ |  |
// |    |   \__  \ | __ \_/ __ \|  |
// |    |___ / __ \| \_\ \  ___/|  |__
// |_______ (____  /___  /\___  >____/
//         \/    \/    \/     \/

// ErrRepoLabelNotExist represents a "RepoLabelNotExist" kind of error.
type ErrRepoLabelNotExist struct {
	LabelID int64
	RepoID  int64
}

// IsErrRepoLabelNotExist checks if an error is a RepoErrLabelNotExist.
func IsErrRepoLabelNotExist(err error) bool {
	_, ok := err.(ErrRepoLabelNotExist)
	return ok
}

func (err ErrRepoLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, repo_id: %d]", err.LabelID, err.RepoID)
}

// ErrOrgLabelNotExist represents a "OrgLabelNotExist" kind of error.
type ErrOrgLabelNotExist struct {
	LabelID int64
	OrgID   int64
}

// IsErrOrgLabelNotExist checks if an error is a OrgErrLabelNotExist.
func IsErrOrgLabelNotExist(err error) bool {
	_, ok := err.(ErrOrgLabelNotExist)
	return ok
}

func (err ErrOrgLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, org_id: %d]", err.LabelID, err.OrgID)
}

// ErrLabelNotExist represents a "LabelNotExist" kind of error.
type ErrLabelNotExist struct {
	LabelID int64
}

// IsErrLabelNotExist checks if an error is a ErrLabelNotExist.
func IsErrLabelNotExist(err error) bool {
	_, ok := err.(ErrLabelNotExist)
	return ok
}

func (err ErrLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d]", err.LabelID)
}

//  ____ ___        .__                    .___
// |    |   \______ |  |   _________     __| _/
// |    |   /\____ \|  |  /  _ \__  \   / __ |
// |    |  / |  |_> >  |_(  <_> ) __ \_/ /_/ |
// |______/  |   __/|____/\____(____  /\____ |
//           |__|                   \/      \/
//

// ErrUploadNotExist represents a "UploadNotExist" kind of error.
type ErrUploadNotExist struct {
	ID   int64
	UUID string
}

// IsErrUploadNotExist checks if an error is a ErrUploadNotExist.
func IsErrUploadNotExist(err error) bool {
	_, ok := err.(ErrUploadNotExist)
	return ok
}

func (err ErrUploadNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

// .___                            ________                                   .___                   .__
// |   | ______ ________ __   ____ \______ \   ____ ______   ____   ____    __| _/____   ____   ____ |__| ____   ______
// |   |/  ___//  ___/  |  \_/ __ \ |    |  \_/ __ \\____ \_/ __ \ /    \  / __ |/ __ \ /    \_/ ___\|  |/ __ \ /  ___/
// |   |\___ \ \___ \|  |  /\  ___/ |    `   \  ___/|  |_> >  ___/|   |  \/ /_/ \  ___/|   |  \  \___|  \  ___/ \___ \
// |___/____  >____  >____/  \___  >_______  /\___  >   __/ \___  >___|  /\____ |\___  >___|  /\___  >__|\___  >____  >
//          \/     \/            \/        \/     \/|__|        \/     \/      \/    \/     \/     \/        \/     \/

// ErrDependencyExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyExists struct {
	IssueID      int64
	DependencyID int64
}

// IsErrDependencyExists checks if an error is a ErrDependencyExists.
func IsErrDependencyExists(err error) bool {
	_, ok := err.(ErrDependencyExists)
	return ok
}

func (err ErrDependencyExists) Error() string {
	return fmt.Sprintf("issue dependency does already exist [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrDependencyNotExists represents a "DependencyAlreadyExists" kind of error.
type ErrDependencyNotExists struct {
	IssueID      int64
	DependencyID int64
}

// IsErrDependencyNotExists checks if an error is a ErrDependencyExists.
func IsErrDependencyNotExists(err error) bool {
	_, ok := err.(ErrDependencyNotExists)
	return ok
}

func (err ErrDependencyNotExists) Error() string {
	return fmt.Sprintf("issue dependency does not exist [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrCircularDependency represents a "DependencyCircular" kind of error.
type ErrCircularDependency struct {
	IssueID      int64
	DependencyID int64
}

// IsErrCircularDependency checks if an error is a ErrCircularDependency.
func IsErrCircularDependency(err error) bool {
	_, ok := err.(ErrCircularDependency)
	return ok
}

func (err ErrCircularDependency) Error() string {
	return fmt.Sprintf("circular dependencies exists (two issues blocking each other) [issue id: %d, dependency id: %d]", err.IssueID, err.DependencyID)
}

// ErrDependenciesLeft represents an error where the issue you're trying to close still has dependencies left.
type ErrDependenciesLeft struct {
	IssueID int64
}

// IsErrDependenciesLeft checks if an error is a ErrDependenciesLeft.
func IsErrDependenciesLeft(err error) bool {
	_, ok := err.(ErrDependenciesLeft)
	return ok
}

func (err ErrDependenciesLeft) Error() string {
	return fmt.Sprintf("issue has open dependencies [issue id: %d]", err.IssueID)
}

// ErrUnknownDependencyType represents an error where an unknown dependency type was passed
type ErrUnknownDependencyType struct {
	Type DependencyType
}

// IsErrUnknownDependencyType checks if an error is ErrUnknownDependencyType
func IsErrUnknownDependencyType(err error) bool {
	_, ok := err.(ErrUnknownDependencyType)
	return ok
}

func (err ErrUnknownDependencyType) Error() string {
	return fmt.Sprintf("unknown dependency type [type: %d]", err.Type)
}

//  __________            .__
//  \______   \ _______  _|__| ______  _  __
//  |       _// __ \  \/ /  |/ __ \ \/ \/ /
//  |    |   \  ___/\   /|  \  ___/\     /
//  |____|_  /\___  >\_/ |__|\___  >\/\_/
//  \/     \/             \/

// ErrReviewNotExist represents a "ReviewNotExist" kind of error.
type ErrReviewNotExist struct {
	ID int64
}

// IsErrReviewNotExist checks if an error is a ErrReviewNotExist.
func IsErrReviewNotExist(err error) bool {
	_, ok := err.(ErrReviewNotExist)
	return ok
}

func (err ErrReviewNotExist) Error() string {
	return fmt.Sprintf("review does not exist [id: %d]", err.ID)
}

// ErrNotValidReviewRequest an not allowed review request modify
type ErrNotValidReviewRequest struct {
	Reason string
	UserID int64
	RepoID int64
}

// IsErrNotValidReviewRequest checks if an error is a ErrNotValidReviewRequest.
func IsErrNotValidReviewRequest(err error) bool {
	_, ok := err.(ErrNotValidReviewRequest)
	return ok
}

func (err ErrNotValidReviewRequest) Error() string {
	return fmt.Sprintf("%s [user_id: %d, repo_id: %d]",
		err.Reason,
		err.UserID,
		err.RepoID)
}
