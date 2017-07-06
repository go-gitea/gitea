// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
)

// ErrNameReserved represents a "reserved name" error.
type ErrNameReserved struct {
	Name string
}

// IsErrNameReserved checks if an error is a ErrNameReserved.
func IsErrNameReserved(err error) bool {
	_, ok := err.(ErrNameReserved)
	return ok
}

func (err ErrNameReserved) Error() string {
	return fmt.Sprintf("name is reserved [name: %s]", err.Name)
}

// ErrNamePatternNotAllowed represents a "pattern not allowed" error.
type ErrNamePatternNotAllowed struct {
	Pattern string
}

// IsErrNamePatternNotAllowed checks if an error is an
// ErrNamePatternNotAllowed.
func IsErrNamePatternNotAllowed(err error) bool {
	_, ok := err.(ErrNamePatternNotAllowed)
	return ok
}

func (err ErrNamePatternNotAllowed) Error() string {
	return fmt.Sprintf("name pattern is not allowed [pattern: %s]", err.Pattern)
}

//  ____ ___
// |    |   \______ ___________
// |    |   /  ___// __ \_  __ \
// |    |  /\___ \\  ___/|  | \/
// |______//____  >\___  >__|
//              \/     \/

// ErrUserAlreadyExist represents a "user already exists" error.
type ErrUserAlreadyExist struct {
	Name string
}

// IsErrUserAlreadyExist checks if an error is a ErrUserAlreadyExists.
func IsErrUserAlreadyExist(err error) bool {
	_, ok := err.(ErrUserAlreadyExist)
	return ok
}

func (err ErrUserAlreadyExist) Error() string {
	return fmt.Sprintf("user already exists [name: %s]", err.Name)
}

// ErrUserNotExist represents a "UserNotExist" kind of error.
type ErrUserNotExist struct {
	UID   int64
	Name  string
	KeyID int64
}

// IsErrUserNotExist checks if an error is a ErrUserNotExist.
func IsErrUserNotExist(err error) bool {
	_, ok := err.(ErrUserNotExist)
	return ok
}

func (err ErrUserNotExist) Error() string {
	return fmt.Sprintf("user does not exist [uid: %d, name: %s, keyid: %d]", err.UID, err.Name, err.KeyID)
}

// ErrEmailAlreadyUsed represents a "EmailAlreadyUsed" kind of error.
type ErrEmailAlreadyUsed struct {
	Email string
}

// IsErrEmailAlreadyUsed checks if an error is a ErrEmailAlreadyUsed.
func IsErrEmailAlreadyUsed(err error) bool {
	_, ok := err.(ErrEmailAlreadyUsed)
	return ok
}

func (err ErrEmailAlreadyUsed) Error() string {
	return fmt.Sprintf("e-mail has been used [email: %s]", err.Email)
}

// ErrOpenIDAlreadyUsed represents a "OpenIDAlreadyUsed" kind of error.
type ErrOpenIDAlreadyUsed struct {
	OpenID string
}

// IsErrOpenIDAlreadyUsed checks if an error is a ErrOpenIDAlreadyUsed.
func IsErrOpenIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrOpenIDAlreadyUsed)
	return ok
}

func (err ErrOpenIDAlreadyUsed) Error() string {
	return fmt.Sprintf("OpenID has been used [oid: %s]", err.OpenID)
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

// ErrUserNotAllowedCreateOrg represents a "UserNotAllowedCreateOrg" kind of error.
type ErrUserNotAllowedCreateOrg struct {
}

// IsErrUserNotAllowedCreateOrg checks if an error is an ErrUserNotAllowedCreateOrg.
func IsErrUserNotAllowedCreateOrg(err error) bool {
	_, ok := err.(ErrUserNotAllowedCreateOrg)
	return ok
}

func (err ErrUserNotAllowedCreateOrg) Error() string {
	return fmt.Sprintf("user is not allowed to create organizations")
}

// ErrReachLimitOfRepo represents a "ReachLimitOfRepo" kind of error.
type ErrReachLimitOfRepo struct {
	Limit int
}

// IsErrReachLimitOfRepo checks if an error is a ErrReachLimitOfRepo.
func IsErrReachLimitOfRepo(err error) bool {
	_, ok := err.(ErrReachLimitOfRepo)
	return ok
}

func (err ErrReachLimitOfRepo) Error() string {
	return fmt.Sprintf("user has reached maximum limit of repositories [limit: %d]", err.Limit)
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

// IsErrWikiAlreadyExist checks if an error is a ErrWikiAlreadyExist.
func IsErrWikiAlreadyExist(err error) bool {
	_, ok := err.(ErrWikiAlreadyExist)
	return ok
}

func (err ErrWikiAlreadyExist) Error() string {
	return fmt.Sprintf("wiki page already exists [title: %s]", err.Title)
}

// __________     ___.   .__  .__          ____  __.
// \______   \__ _\_ |__ |  | |__| ____   |    |/ _|____ ___.__.
//  |     ___/  |  \ __ \|  | |  |/ ___\  |      <_/ __ <   |  |
//  |    |   |  |  / \_\ \  |_|  \  \___  |    |  \  ___/\___  |
//  |____|   |____/|___  /____/__|\___  > |____|__ \___  > ____|
//                     \/             \/          \/   \/\/

// ErrKeyUnableVerify represents a "KeyUnableVerify" kind of error.
type ErrKeyUnableVerify struct {
	Result string
}

// IsErrKeyUnableVerify checks if an error is a ErrKeyUnableVerify.
func IsErrKeyUnableVerify(err error) bool {
	_, ok := err.(ErrKeyUnableVerify)
	return ok
}

func (err ErrKeyUnableVerify) Error() string {
	return fmt.Sprintf("Unable to verify key content [result: %s]", err.Result)
}

// ErrKeyNotExist represents a "KeyNotExist" kind of error.
type ErrKeyNotExist struct {
	ID int64
}

// IsErrKeyNotExist checks if an error is a ErrKeyNotExist.
func IsErrKeyNotExist(err error) bool {
	_, ok := err.(ErrKeyNotExist)
	return ok
}

func (err ErrKeyNotExist) Error() string {
	return fmt.Sprintf("public key does not exist [id: %d]", err.ID)
}

// ErrKeyAlreadyExist represents a "KeyAlreadyExist" kind of error.
type ErrKeyAlreadyExist struct {
	OwnerID     int64
	Fingerprint string
	Content     string
}

// IsErrKeyAlreadyExist checks if an error is a ErrKeyAlreadyExist.
func IsErrKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrKeyAlreadyExist)
	return ok
}

func (err ErrKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, finter_print: %s, content: %s]",
		err.OwnerID, err.Fingerprint, err.Content)
}

// ErrKeyNameAlreadyUsed represents a "KeyNameAlreadyUsed" kind of error.
type ErrKeyNameAlreadyUsed struct {
	OwnerID int64
	Name    string
}

// IsErrKeyNameAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrKeyNameAlreadyUsed)
	return ok
}

func (err ErrKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [owner_id: %d, name: %s]", err.OwnerID, err.Name)
}

// ErrGPGEmailNotFound represents a "ErrGPGEmailNotFound" kind of error.
type ErrGPGEmailNotFound struct {
	Email string
}

// IsErrGPGEmailNotFound checks if an error is a ErrGPGEmailNotFound.
func IsErrGPGEmailNotFound(err error) bool {
	_, ok := err.(ErrGPGEmailNotFound)
	return ok
}

func (err ErrGPGEmailNotFound) Error() string {
	return fmt.Sprintf("failed to found email or is not confirmed : %s", err.Email)
}

// ErrGPGKeyParsing represents a "ErrGPGKeyParsing" kind of error.
type ErrGPGKeyParsing struct {
	ParseError error
}

// IsErrGPGKeyParsing checks if an error is a ErrGPGKeyParsing.
func IsErrGPGKeyParsing(err error) bool {
	_, ok := err.(ErrGPGKeyParsing)
	return ok
}

func (err ErrGPGKeyParsing) Error() string {
	return fmt.Sprintf("failed to parse gpg key %s", err.ParseError.Error())
}

// ErrGPGKeyNotExist represents a "GPGKeyNotExist" kind of error.
type ErrGPGKeyNotExist struct {
	ID int64
}

// IsErrGPGKeyNotExist checks if an error is a ErrGPGKeyNotExist.
func IsErrGPGKeyNotExist(err error) bool {
	_, ok := err.(ErrGPGKeyNotExist)
	return ok
}

func (err ErrGPGKeyNotExist) Error() string {
	return fmt.Sprintf("public gpg key does not exist [id: %d]", err.ID)
}

// ErrGPGKeyIDAlreadyUsed represents a "GPGKeyIDAlreadyUsed" kind of error.
type ErrGPGKeyIDAlreadyUsed struct {
	KeyID string
}

// IsErrGPGKeyIDAlreadyUsed checks if an error is a ErrKeyNameAlreadyUsed.
func IsErrGPGKeyIDAlreadyUsed(err error) bool {
	_, ok := err.(ErrGPGKeyIDAlreadyUsed)
	return ok
}

func (err ErrGPGKeyIDAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %s]", err.KeyID)
}

// ErrGPGKeyAccessDenied represents a "GPGKeyAccessDenied" kind of Error.
type ErrGPGKeyAccessDenied struct {
	UserID int64
	KeyID  int64
}

// IsErrGPGKeyAccessDenied checks if an error is a ErrGPGKeyAccessDenied.
func IsErrGPGKeyAccessDenied(err error) bool {
	_, ok := err.(ErrGPGKeyAccessDenied)
	return ok
}

// Error pretty-prints an error of type ErrGPGKeyAccessDenied.
func (err ErrGPGKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, key_id: %d]",
		err.UserID, err.KeyID)
}

// ErrKeyAccessDenied represents a "KeyAccessDenied" kind of error.
type ErrKeyAccessDenied struct {
	UserID int64
	KeyID  int64
	Note   string
}

// IsErrKeyAccessDenied checks if an error is a ErrKeyAccessDenied.
func IsErrKeyAccessDenied(err error) bool {
	_, ok := err.(ErrKeyAccessDenied)
	return ok
}

func (err ErrKeyAccessDenied) Error() string {
	return fmt.Sprintf("user does not have access to the key [user_id: %d, key_id: %d, note: %s]",
		err.UserID, err.KeyID, err.Note)
}

// ErrDeployKeyNotExist represents a "DeployKeyNotExist" kind of error.
type ErrDeployKeyNotExist struct {
	ID     int64
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyNotExist checks if an error is a ErrDeployKeyNotExist.
func IsErrDeployKeyNotExist(err error) bool {
	_, ok := err.(ErrDeployKeyNotExist)
	return ok
}

func (err ErrDeployKeyNotExist) Error() string {
	return fmt.Sprintf("Deploy key does not exist [id: %d, key_id: %d, repo_id: %d]", err.ID, err.KeyID, err.RepoID)
}

// ErrDeployKeyAlreadyExist represents a "DeployKeyAlreadyExist" kind of error.
type ErrDeployKeyAlreadyExist struct {
	KeyID  int64
	RepoID int64
}

// IsErrDeployKeyAlreadyExist checks if an error is a ErrDeployKeyAlreadyExist.
func IsErrDeployKeyAlreadyExist(err error) bool {
	_, ok := err.(ErrDeployKeyAlreadyExist)
	return ok
}

func (err ErrDeployKeyAlreadyExist) Error() string {
	return fmt.Sprintf("public key already exists [key_id: %d, repo_id: %d]", err.KeyID, err.RepoID)
}

// ErrDeployKeyNameAlreadyUsed represents a "DeployKeyNameAlreadyUsed" kind of error.
type ErrDeployKeyNameAlreadyUsed struct {
	RepoID int64
	Name   string
}

// IsErrDeployKeyNameAlreadyUsed checks if an error is a ErrDeployKeyNameAlreadyUsed.
func IsErrDeployKeyNameAlreadyUsed(err error) bool {
	_, ok := err.(ErrDeployKeyNameAlreadyUsed)
	return ok
}

func (err ErrDeployKeyNameAlreadyUsed) Error() string {
	return fmt.Sprintf("public key already exists [repo_id: %d, name: %s]", err.RepoID, err.Name)
}

//    _____                                   ___________     __
//   /  _  \   ____  ____  ____   ______ _____\__    ___/___ |  | __ ____   ____
//  /  /_\  \_/ ___\/ ___\/ __ \ /  ___//  ___/ |    | /  _ \|  |/ // __ \ /    \
// /    |    \  \__\  \__\  ___/ \___ \ \___ \  |    |(  <_> )    <\  ___/|   |  \
// \____|__  /\___  >___  >___  >____  >____  > |____| \____/|__|_ \\___  >___|  /
//         \/     \/    \/    \/     \/     \/                    \/    \/     \/

// ErrAccessTokenNotExist represents a "AccessTokenNotExist" kind of error.
type ErrAccessTokenNotExist struct {
	SHA string
}

// IsErrAccessTokenNotExist checks if an error is a ErrAccessTokenNotExist.
func IsErrAccessTokenNotExist(err error) bool {
	_, ok := err.(ErrAccessTokenNotExist)
	return ok
}

func (err ErrAccessTokenNotExist) Error() string {
	return fmt.Sprintf("access token does not exist [sha: %s]", err.SHA)
}

// ErrAccessTokenEmpty represents a "AccessTokenEmpty" kind of error.
type ErrAccessTokenEmpty struct {
}

// IsErrAccessTokenEmpty checks if an error is a ErrAccessTokenEmpty.
func IsErrAccessTokenEmpty(err error) bool {
	_, ok := err.(ErrAccessTokenEmpty)
	return ok
}

func (err ErrAccessTokenEmpty) Error() string {
	return fmt.Sprintf("access token is empty")
}

// ________                            .__                __  .__
// \_____  \_______  _________    ____ |__|____________ _/  |_|__| ____   ____
//  /   |   \_  __ \/ ___\__  \  /    \|  \___   /\__  \\   __\  |/  _ \ /    \
// /    |    \  | \/ /_/  > __ \|   |  \  |/    /  / __ \|  | |  (  <_> )   |  \
// \_______  /__|  \___  (____  /___|  /__/_____ \(____  /__| |__|\____/|___|  /
//         \/     /_____/     \/     \/         \/     \/                    \/

// ErrOrgNotExist represents a "OrgNotExist" kind of error.
type ErrOrgNotExist struct {
	ID   int64
	Name string
}

// IsErrOrgNotExist checks if an error is a ErrOrgNotExist.
func IsErrOrgNotExist(err error) bool {
	_, ok := err.(ErrOrgNotExist)
	return ok
}

func (err ErrOrgNotExist) Error() string {
	return fmt.Sprintf("org does not exist [id: %d, name: %s]", err.ID, err.Name)
}

// ErrLastOrgOwner represents a "LastOrgOwner" kind of error.
type ErrLastOrgOwner struct {
	UID int64
}

// IsErrLastOrgOwner checks if an error is a ErrLastOrgOwner.
func IsErrLastOrgOwner(err error) bool {
	_, ok := err.(ErrLastOrgOwner)
	return ok
}

func (err ErrLastOrgOwner) Error() string {
	return fmt.Sprintf("user is the last member of owner team [uid: %d]", err.UID)
}

// __________                           .__  __
// \______   \ ____ ______   ____  _____|__|/  |_  ___________ ___.__.
//  |       _// __ \\____ \ /  _ \/  ___/  \   __\/  _ \_  __ <   |  |
//  |    |   \  ___/|  |_> >  <_> )___ \|  ||  | (  <_> )  | \/\___  |
//  |____|_  /\___  >   __/ \____/____  >__||__|  \____/|__|   / ____|
//         \/     \/|__|              \/                       \/

// ErrRepoNotExist represents a "RepoNotExist" kind of error.
type ErrRepoNotExist struct {
	ID   int64
	UID  int64
	Name string
}

// IsErrRepoNotExist checks if an error is a ErrRepoNotExist.
func IsErrRepoNotExist(err error) bool {
	_, ok := err.(ErrRepoNotExist)
	return ok
}

func (err ErrRepoNotExist) Error() string {
	return fmt.Sprintf("repository does not exist [id: %d, uid: %d, name: %s]", err.ID, err.UID, err.Name)
}

// ErrRepoAlreadyExist represents a "RepoAlreadyExist" kind of error.
type ErrRepoAlreadyExist struct {
	Uname string
	Name  string
}

// IsErrRepoAlreadyExist checks if an error is a ErrRepoAlreadyExist.
func IsErrRepoAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoAlreadyExist)
	return ok
}

func (err ErrRepoAlreadyExist) Error() string {
	return fmt.Sprintf("repository already exists [uname: %s, name: %s]", err.Uname, err.Name)
}

// ErrRepoRedirectNotExist represents a "RepoRedirectNotExist" kind of error.
type ErrRepoRedirectNotExist struct {
	OwnerID  int64
	RepoName string
}

// IsErrRepoRedirectNotExist check if an error is an ErrRepoRedirectNotExist
func IsErrRepoRedirectNotExist(err error) bool {
	_, ok := err.(ErrRepoRedirectNotExist)
	return ok
}

func (err ErrRepoRedirectNotExist) Error() string {
	return fmt.Sprintf("repository redirect does not exist [uid: %d, name: %s]", err.OwnerID, err.RepoName)
}

// ErrInvalidCloneAddr represents a "InvalidCloneAddr" kind of error.
type ErrInvalidCloneAddr struct {
	IsURLError         bool
	IsInvalidPath      bool
	IsPermissionDenied bool
}

// IsErrInvalidCloneAddr checks if an error is a ErrInvalidCloneAddr.
func IsErrInvalidCloneAddr(err error) bool {
	_, ok := err.(ErrInvalidCloneAddr)
	return ok
}

func (err ErrInvalidCloneAddr) Error() string {
	return fmt.Sprintf("invalid clone address [is_url_error: %v, is_invalid_path: %v, is_permission_denied: %v]",
		err.IsURLError, err.IsInvalidPath, err.IsPermissionDenied)
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

// ErrRepoFileAlreadyExist represents a "RepoFileAlreadyExist" kind of error.
type ErrRepoFileAlreadyExist struct {
	FileName string
}

// IsErrRepoFileAlreadyExist checks if an error is a ErrRepoFileAlreadyExist.
func IsErrRepoFileAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoFileAlreadyExist)
	return ok
}

func (err ErrRepoFileAlreadyExist) Error() string {
	return fmt.Sprintf("repository file already exists [file_name: %s]", err.FileName)
}

// __________                             .__
// \______   \____________    ____   ____ |  |__
//  |    |  _/\_  __ \__  \  /    \_/ ___\|  |  \
//  |    |   \ |  | \// __ \|   |  \  \___|   Y  \
//  |______  / |__|  (____  /___|  /\___  >___|  /
//         \/             \/     \/     \/     \/

// ErrBranchNotExist represents a "BranchNotExist" kind of error.
type ErrBranchNotExist struct {
	Name string
}

// IsErrBranchNotExist checks if an error is a ErrBranchNotExist.
func IsErrBranchNotExist(err error) bool {
	_, ok := err.(ErrBranchNotExist)
	return ok
}

func (err ErrBranchNotExist) Error() string {
	return fmt.Sprintf("branch does not exist [name: %s]", err.Name)
}

//  __      __      ___.   .__                   __
// /  \    /  \ ____\_ |__ |  |__   ____   ____ |  | __
// \   \/\/   // __ \| __ \|  |  \ /  _ \ /  _ \|  |/ /
//  \        /\  ___/| \_\ \   Y  (  <_> |  <_> )    <
//   \__/\  /  \___  >___  /___|  /\____/ \____/|__|_ \
//        \/       \/    \/     \/                   \/

// ErrWebhookNotExist represents a "WebhookNotExist" kind of error.
type ErrWebhookNotExist struct {
	ID int64
}

// IsErrWebhookNotExist checks if an error is a ErrWebhookNotExist.
func IsErrWebhookNotExist(err error) bool {
	_, ok := err.(ErrWebhookNotExist)
	return ok
}

func (err ErrWebhookNotExist) Error() string {
	return fmt.Sprintf("webhook does not exist [id: %d]", err.ID)
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

// .____          ___.          .__
// |    |   _____ \_ |__   ____ |  |
// |    |   \__  \ | __ \_/ __ \|  |
// |    |___ / __ \| \_\ \  ___/|  |__
// |_______ (____  /___  /\___  >____/
//         \/    \/    \/     \/

// ErrLabelNotExist represents a "LabelNotExist" kind of error.
type ErrLabelNotExist struct {
	LabelID int64
	RepoID  int64
}

// IsErrLabelNotExist checks if an error is a ErrLabelNotExist.
func IsErrLabelNotExist(err error) bool {
	_, ok := err.(ErrLabelNotExist)
	return ok
}

func (err ErrLabelNotExist) Error() string {
	return fmt.Sprintf("label does not exist [label_id: %d, repo_id: %d]", err.LabelID, err.RepoID)
}

//    _____  .__.__                   __
//   /     \ |__|  |   ____   _______/  |_  ____   ____   ____
//  /  \ /  \|  |  | _/ __ \ /  ___/\   __\/  _ \ /    \_/ __ \
// /    Y    \  |  |_\  ___/ \___ \  |  | (  <_> )   |  \  ___/
// \____|__  /__|____/\___  >____  > |__|  \____/|___|  /\___  >
//         \/             \/     \/                   \/     \/

// ErrMilestoneNotExist represents a "MilestoneNotExist" kind of error.
type ErrMilestoneNotExist struct {
	ID     int64
	RepoID int64
}

// IsErrMilestoneNotExist checks if an error is a ErrMilestoneNotExist.
func IsErrMilestoneNotExist(err error) bool {
	_, ok := err.(ErrMilestoneNotExist)
	return ok
}

func (err ErrMilestoneNotExist) Error() string {
	return fmt.Sprintf("milestone does not exist [id: %d, repo_id: %d]", err.ID, err.RepoID)
}

//    _____   __    __                .__                           __
//   /  _  \_/  |__/  |______    ____ |  |__   _____   ____   _____/  |_
//  /  /_\  \   __\   __\__  \ _/ ___\|  |  \ /     \_/ __ \ /    \   __\
// /    |    \  |  |  |  / __ \\  \___|   Y  \  Y Y  \  ___/|   |  \  |
// \____|__  /__|  |__| (____  /\___  >___|  /__|_|  /\___  >___|  /__|
//         \/                \/     \/     \/      \/     \/     \/

// ErrAttachmentNotExist represents a "AttachmentNotExist" kind of error.
type ErrAttachmentNotExist struct {
	ID   int64
	UUID string
}

// IsErrAttachmentNotExist checks if an error is a ErrAttachmentNotExist.
func IsErrAttachmentNotExist(err error) bool {
	_, ok := err.(ErrAttachmentNotExist)
	return ok
}

func (err ErrAttachmentNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

// .____                 .__           _________
// |    |    ____   ____ |__| ____    /   _____/ ____  __ _________   ____  ____
// |    |   /  _ \ / ___\|  |/    \   \_____  \ /  _ \|  |  \_  __ \_/ ___\/ __ \
// |    |__(  <_> ) /_/  >  |   |  \  /        (  <_> )  |  /|  | \/\  \__\  ___/
// |_______ \____/\___  /|__|___|  / /_______  /\____/|____/ |__|    \___  >___  >
//         \/    /_____/         \/          \/                          \/    \/

// ErrLoginSourceNotExist represents a "LoginSourceNotExist" kind of error.
type ErrLoginSourceNotExist struct {
	ID int64
}

// IsErrLoginSourceNotExist checks if an error is a ErrLoginSourceNotExist.
func IsErrLoginSourceNotExist(err error) bool {
	_, ok := err.(ErrLoginSourceNotExist)
	return ok
}

func (err ErrLoginSourceNotExist) Error() string {
	return fmt.Sprintf("login source does not exist [id: %d]", err.ID)
}

// ErrLoginSourceAlreadyExist represents a "LoginSourceAlreadyExist" kind of error.
type ErrLoginSourceAlreadyExist struct {
	Name string
}

// IsErrLoginSourceAlreadyExist checks if an error is a ErrLoginSourceAlreadyExist.
func IsErrLoginSourceAlreadyExist(err error) bool {
	_, ok := err.(ErrLoginSourceAlreadyExist)
	return ok
}

func (err ErrLoginSourceAlreadyExist) Error() string {
	return fmt.Sprintf("login source already exists [name: %s]", err.Name)
}

// ErrLoginSourceInUse represents a "LoginSourceInUse" kind of error.
type ErrLoginSourceInUse struct {
	ID int64
}

// IsErrLoginSourceInUse checks if an error is a ErrLoginSourceInUse.
func IsErrLoginSourceInUse(err error) bool {
	_, ok := err.(ErrLoginSourceInUse)
	return ok
}

func (err ErrLoginSourceInUse) Error() string {
	return fmt.Sprintf("login source is still used by some users [id: %d]", err.ID)
}

// ___________
// \__    ___/___ _____    _____
//   |    |_/ __ \\__  \  /     \
//   |    |\  ___/ / __ \|  Y Y  \
//   |____| \___  >____  /__|_|  /
//              \/     \/      \/

// ErrTeamAlreadyExist represents a "TeamAlreadyExist" kind of error.
type ErrTeamAlreadyExist struct {
	OrgID int64
	Name  string
}

// IsErrTeamAlreadyExist checks if an error is a ErrTeamAlreadyExist.
func IsErrTeamAlreadyExist(err error) bool {
	_, ok := err.(ErrTeamAlreadyExist)
	return ok
}

func (err ErrTeamAlreadyExist) Error() string {
	return fmt.Sprintf("team already exists [org_id: %d, name: %s]", err.OrgID, err.Name)
}

//
// Two-factor authentication
//

// ErrTwoFactorNotEnrolled indicates that a user is not enrolled in two-factor authentication.
type ErrTwoFactorNotEnrolled struct {
	UID int64
}

// IsErrTwoFactorNotEnrolled checks if an error is a ErrTwoFactorNotEnrolled.
func IsErrTwoFactorNotEnrolled(err error) bool {
	_, ok := err.(ErrTwoFactorNotEnrolled)
	return ok
}

func (err ErrTwoFactorNotEnrolled) Error() string {
	return fmt.Sprintf("user not enrolled in 2FA [uid: %d]", err.UID)
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
	_, ok := err.(ErrAttachmentNotExist)
	return ok
}

func (err ErrUploadNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

//  ___________         __                             .__    .____                 .__          ____ ___
//  \_   _____/__  ____/  |_  ___________  ____ _____  |  |   |    |    ____   ____ |__| ____   |    |   \______ ___________
//   |    __)_\  \/  /\   __\/ __ \_  __ \/    \\__  \ |  |   |    |   /  _ \ / ___\|  |/    \  |    |   /  ___// __ \_  __ \
//   |        \>    <  |  | \  ___/|  | \/   |  \/ __ \|  |__ |    |__(  <_> ) /_/  >  |   |  \ |    |  /\___ \\  ___/|  | \/
//  /_______  /__/\_ \ |__|  \___  >__|  |___|  (____  /____/ |_______ \____/\___  /|__|___|  / |______//____  >\___  >__|
//          \/      \/           \/           \/     \/               \/    /_____/         \/               \/     \/

// ErrExternalLoginUserAlreadyExist represents a "ExternalLoginUserAlreadyExist" kind of error.
type ErrExternalLoginUserAlreadyExist struct {
	ExternalID    string
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserAlreadyExist checks if an error is a ExternalLoginUserAlreadyExist.
func IsErrExternalLoginUserAlreadyExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserAlreadyExist)
	return ok
}

func (err ErrExternalLoginUserAlreadyExist) Error() string {
	return fmt.Sprintf("external login user already exists [externalID: %s, userID: %d, loginSourceID: %d]", err.ExternalID, err.UserID, err.LoginSourceID)
}

// ErrExternalLoginUserNotExist represents a "ExternalLoginUserNotExist" kind of error.
type ErrExternalLoginUserNotExist struct {
	UserID        int64
	LoginSourceID int64
}

// IsErrExternalLoginUserNotExist checks if an error is a ExternalLoginUserNotExist.
func IsErrExternalLoginUserNotExist(err error) bool {
	_, ok := err.(ErrExternalLoginUserNotExist)
	return ok
}

func (err ErrExternalLoginUserNotExist) Error() string {
	return fmt.Sprintf("external login user link does not exists [userID: %d, loginSourceID: %d]", err.UserID, err.LoginSourceID)
}
