// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

// PullRequest contains information to make a pull request
type PullRequest struct {
	BaseRepo       *repo_model.Repository
	Allowed        bool
	SameRepo       bool
	HeadInfoSubURL string // [<user>:]<branch> url segment
}

// Repository contains information to operate a repository
type Repository struct {
	access_model.Permission
	IsWatching   bool
	IsViewBranch bool
	IsViewTag    bool
	IsViewCommit bool
	Repository   *repo_model.Repository
	Owner        *user_model.User
	Commit       *git.Commit
	Tag          *git.Tag
	GitRepo      *git.Repository
	RefName      string
	BranchName   string
	TagName      string
	TreePath     string
	CommitID     string
	RepoLink     string
	CloneLink    repo_model.CloneLink
	CommitsCount int64

	PullRequest *PullRequest
}

// CanWriteToBranch checks if the branch is writable by the user
func (r *Repository) CanWriteToBranch(ctx context.Context, user *user_model.User, branch string) bool {
	return issues_model.CanMaintainerWriteToBranch(ctx, r.Permission, branch, user)
}

// CanEnableEditor returns true if repository is editable and user has proper access level.
func (r *Repository) CanEnableEditor(ctx context.Context, user *user_model.User) bool {
	return r.IsViewBranch && r.CanWriteToBranch(ctx, user, r.BranchName) && r.Repository.CanEnableEditor() && !r.Repository.IsArchived
}

// CanCreateBranch returns true if repository is editable and user has proper access level.
func (r *Repository) CanCreateBranch() bool {
	return r.Permission.CanWrite(unit_model.TypeCode) && r.Repository.CanCreateBranch()
}

func (r *Repository) GetObjectFormat() git.ObjectFormat {
	return git.ObjectFormatFromName(r.Repository.ObjectFormatName)
}

// RepoMustNotBeArchived checks if a repo is archived
func RepoMustNotBeArchived() func(ctx *Context) {
	return func(ctx *Context) {
		if ctx.Repo.Repository.IsArchived {
			ctx.NotFound("IsArchived", errors.New(ctx.Locale.TrString("repo.archive.title")))
		}
	}
}

// CanCommitToBranchResults represents the results of CanCommitToBranch
type CanCommitToBranchResults struct {
	CanCommitToBranch bool
	EditorEnabled     bool
	UserCanPush       bool
	RequireSigned     bool
	WillSign          bool
	SigningKey        string
	WontSignReason    string
}

// CanCommitToBranch returns true if repository is editable and user has proper access level
//
// and branch is not protected for push
func (r *Repository) CanCommitToBranch(ctx context.Context, doer *user_model.User) (CanCommitToBranchResults, error) {
	protectedBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, r.Repository.ID, r.BranchName)
	if err != nil {
		return CanCommitToBranchResults{}, err
	}
	userCanPush := true
	requireSigned := false
	if protectedBranch != nil {
		protectedBranch.Repo = r.Repository
		userCanPush = protectedBranch.CanUserPush(ctx, doer)
		requireSigned = protectedBranch.RequireSignedCommits
	}

	sign, keyID, _, err := asymkey_service.SignCRUDAction(ctx, r.Repository.RepoPath(), doer, r.Repository.RepoPath(), git.BranchPrefix+r.BranchName)

	canCommit := r.CanEnableEditor(ctx, doer) && userCanPush
	if requireSigned {
		canCommit = canCommit && sign
	}
	wontSignReason := ""
	if err != nil {
		if asymkey_service.IsErrWontSign(err) {
			wontSignReason = string(err.(*asymkey_service.ErrWontSign).Reason)
			err = nil
		} else {
			wontSignReason = "error"
		}
	}

	return CanCommitToBranchResults{
		CanCommitToBranch: canCommit,
		EditorEnabled:     r.CanEnableEditor(ctx, doer),
		UserCanPush:       userCanPush,
		RequireSigned:     requireSigned,
		WillSign:          sign,
		SigningKey:        keyID,
		WontSignReason:    wontSignReason,
	}, err
}

// CanUseTimetracker returns whether or not a user can use the timetracker.
func (r *Repository) CanUseTimetracker(ctx context.Context, issue *issues_model.Issue, user *user_model.User) bool {
	// Checking for following:
	// 1. Is timetracker enabled
	// 2. Is the user a contributor, admin, poster or assignee and do the repository policies require this?
	isAssigned, _ := issues_model.IsUserAssignedToIssue(ctx, issue, user)
	return r.Repository.IsTimetrackerEnabled(ctx) && (!r.Repository.AllowOnlyContributorsToTrackTime(ctx) ||
		r.Permission.CanWriteIssuesOrPulls(issue.IsPull) || issue.IsPoster(user.ID) || isAssigned)
}

// CanCreateIssueDependencies returns whether or not a user can create dependencies.
func (r *Repository) CanCreateIssueDependencies(ctx context.Context, user *user_model.User, isPull bool) bool {
	return r.Repository.IsDependenciesEnabled(ctx) && r.Permission.CanWriteIssuesOrPulls(isPull)
}

// GetCommitsCount returns cached commit count for current view
func (r *Repository) GetCommitsCount() (int64, error) {
	if r.Commit == nil {
		return 0, nil
	}
	var contextName string
	if r.IsViewBranch {
		contextName = r.BranchName
	} else if r.IsViewTag {
		contextName = r.TagName
	} else {
		contextName = r.CommitID
	}
	return cache.GetInt64(r.Repository.GetCommitsCountCacheKey(contextName, r.IsViewBranch || r.IsViewTag), func() (int64, error) {
		return r.Commit.CommitsCount()
	})
}

// GetCommitGraphsCount returns cached commit count for current view
func (r *Repository) GetCommitGraphsCount(ctx context.Context, hidePRRefs bool, branches, files []string) (int64, error) {
	cacheKey := fmt.Sprintf("commits-count-%d-graph-%t-%s-%s", r.Repository.ID, hidePRRefs, branches, files)

	return cache.GetInt64(cacheKey, func() (int64, error) {
		if len(branches) == 0 {
			return git.AllCommitsCount(ctx, r.Repository.RepoPath(), hidePRRefs, files...)
		}
		return git.CommitsCount(ctx,
			git.CommitsCountOptions{
				RepoPath: r.Repository.RepoPath(),
				Revision: branches,
				RelPath:  files,
			})
	})
}

// BranchNameSubURL sub-URL for the BranchName field
func (r *Repository) BranchNameSubURL() string {
	switch {
	case r.IsViewBranch:
		return "branch/" + util.PathEscapeSegments(r.BranchName)
	case r.IsViewTag:
		return "tag/" + util.PathEscapeSegments(r.TagName)
	case r.IsViewCommit:
		return "commit/" + util.PathEscapeSegments(r.CommitID)
	}
	log.Error("Unknown view type for repo: %v", r)
	return ""
}

// FileExists returns true if a file exists in the given repo branch
func (r *Repository) FileExists(path, branch string) (bool, error) {
	if branch == "" {
		branch = r.Repository.DefaultBranch
	}
	commit, err := r.GitRepo.GetBranchCommit(branch)
	if err != nil {
		return false, err
	}
	if _, err := commit.GetTreeEntryByPath(path); err != nil {
		return false, err
	}
	return true, nil
}

// GetEditorconfig returns the .editorconfig definition if found in the
// HEAD of the default repo branch.
func (r *Repository) GetEditorconfig(optCommit ...*git.Commit) (cfg *editorconfig.Editorconfig, warning, err error) {
	if r.GitRepo == nil {
		return nil, nil, nil
	}

	var commit *git.Commit

	if len(optCommit) != 0 {
		commit = optCommit[0]
	} else {
		commit, err = r.GitRepo.GetBranchCommit(r.Repository.DefaultBranch)
		if err != nil {
			return nil, nil, err
		}
	}
	treeEntry, err := commit.GetTreeEntryByPath(".editorconfig")
	if err != nil {
		return nil, nil, err
	}
	if treeEntry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
		return nil, nil, git.ErrNotExist{ID: "", RelPath: ".editorconfig"}
	}
	reader, err := treeEntry.Blob().DataAsync()
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	return editorconfig.ParseGraceful(reader)
}

// RetrieveBaseRepo retrieves base repository
func RetrieveBaseRepo(ctx *Context, repo *repo_model.Repository) {
	// Non-fork repository will not return error in this method.
	if err := repo.GetBaseRepo(ctx); err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			repo.IsFork = false
			repo.ForkID = 0
			return
		}
		ctx.ServerError("GetBaseRepo", err)
		return
	} else if err = repo.BaseRepo.LoadOwner(ctx); err != nil {
		ctx.ServerError("BaseRepo.LoadOwner", err)
		return
	}
}

// RetrieveTemplateRepo retrieves template repository used to generate this repository
func RetrieveTemplateRepo(ctx *Context, repo *repo_model.Repository) {
	// Non-generated repository will not return error in this method.
	templateRepo, err := repo_model.GetTemplateRepo(ctx, repo)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			repo.TemplateID = 0
			return
		}
		ctx.ServerError("GetTemplateRepo", err)
		return
	} else if err = templateRepo.LoadOwner(ctx); err != nil {
		ctx.ServerError("TemplateRepo.LoadOwner", err)
		return
	}

	perm, err := access_model.GetUserRepoPermission(ctx, templateRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return
	}

	if !perm.CanRead(unit_model.TypeCode) {
		repo.TemplateID = 0
	}
}

// ComposeGoGetImport returns go-get-import meta content.
func ComposeGoGetImport(owner, repo string) string {
	/// setting.AppUrl is guaranteed to be parse as url
	appURL, _ := url.Parse(setting.AppURL)

	return path.Join(appURL.Host, setting.AppSubURL, url.PathEscape(owner), url.PathEscape(repo))
}

// EarlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func EarlyResponseForGoGetMeta(ctx *Context) {
	username := ctx.Params(":username")
	reponame := strings.TrimSuffix(ctx.Params(":reponame"), ".git")
	if username == "" || reponame == "" {
		ctx.PlainText(http.StatusBadRequest, "invalid repository path")
		return
	}

	var cloneURL string
	if setting.Repository.GoGetCloneURLProtocol == "ssh" {
		cloneURL = repo_model.ComposeSSHCloneURL(username, reponame)
	} else {
		cloneURL = repo_model.ComposeHTTPSCloneURL(username, reponame)
	}
	goImportContent := fmt.Sprintf("%s git %s", ComposeGoGetImport(username, reponame), cloneURL)
	htmlMeta := fmt.Sprintf(`<meta name="go-import" content="%s">`, html.EscapeString(goImportContent))
	ctx.PlainText(http.StatusOK, htmlMeta)
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Base, redirectRepoID int64) {
	ownerName := ctx.Params(":username")
	previousRepoName := ctx.Params(":reponame")

	repo, err := repo_model.GetRepositoryByID(ctx, redirectRepoID)
	if err != nil {
		log.Error("GetRepositoryByID: %v", err)
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByID")
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.EscapedPath(),
		url.PathEscape(ownerName)+"/"+url.PathEscape(previousRepoName),
		url.PathEscape(repo.OwnerName)+"/"+url.PathEscape(repo.Name),
		1,
	)
	if ctx.Req.URL.RawQuery != "" {
		redirectPath += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(path.Join(setting.AppSubURL, redirectPath), http.StatusTemporaryRedirect)
}

func repoAssignment(ctx *Context, repo *repo_model.Repository) {
	var err error
	if err = repo.LoadOwner(ctx); err != nil {
		ctx.ServerError("LoadOwner", err)
		return
	}

	ctx.Repo.Permission, err = access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return
	}

	if !ctx.Repo.Permission.HasAnyUnitAccessOrEveryoneAccess() {
		if ctx.FormString("go-get") == "1" {
			EarlyResponseForGoGetMeta(ctx)
			return
		}
		ctx.NotFound("no access right", nil)
		return
	}
	ctx.Data["Permission"] = &ctx.Repo.Permission

	if repo.IsMirror {
		pullMirror, err := repo_model.GetMirrorByRepoID(ctx, repo.ID)
		if err == nil {
			ctx.Data["PullMirror"] = pullMirror
		} else if err != repo_model.ErrMirrorNotExist {
			ctx.ServerError("GetMirrorByRepoID", err)
			return
		}
	}

	pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(ctx, repo.ID, db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetPushMirrorsByRepoID", err)
		return
	}

	ctx.Repo.Repository = repo
	ctx.Data["PushMirrors"] = pushMirrors
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["IsEmptyRepo"] = ctx.Repo.Repository.IsEmpty
}

// RepoAssignment returns a middleware to handle repository assignment
func RepoAssignment(ctx *Context) context.CancelFunc {
	if _, repoAssignmentOnce := ctx.Data["repoAssignmentExecuted"]; repoAssignmentOnce {
		log.Trace("RepoAssignment was exec already, skipping second call ...")
		return nil
	}
	ctx.Data["repoAssignmentExecuted"] = true

	var (
		owner *user_model.User
		err   error
	)

	userName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")
	repoName = strings.TrimSuffix(repoName, ".git")
	if setting.Other.EnableFeed {
		repoName = strings.TrimSuffix(repoName, ".rss")
		repoName = strings.TrimSuffix(repoName, ".atom")
	}

	// Check if the user is the same as the repository owner
	if ctx.IsSigned && ctx.Doer.LowerName == strings.ToLower(userName) {
		owner = ctx.Doer
	} else {
		owner, err = user_model.GetUserByName(ctx, userName)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				// go-get does not support redirects
				// https://github.com/golang/go/issues/19760
				if ctx.FormString("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return nil
				}

				if redirectUserID, err := user_model.LookupUserRedirect(ctx, userName); err == nil {
					RedirectToUser(ctx.Base, userName, redirectUserID)
				} else if user_model.IsErrUserRedirectNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("LookupUserRedirect", err)
				}
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil
		}
	}
	ctx.Repo.Owner = owner
	ctx.ContextUser = owner
	ctx.Data["ContextUser"] = ctx.ContextUser
	ctx.Data["Username"] = ctx.Repo.Owner.Name

	// redirect link to wiki
	if strings.HasSuffix(repoName, ".wiki") {
		// ctx.Req.URL.Path does not have the preceding appSubURL - any redirect must have this added
		// Now we happen to know that all of our paths are: /:username/:reponame/whatever_else
		originalRepoName := ctx.Params(":reponame")
		redirectRepoName := strings.TrimSuffix(repoName, ".wiki")
		redirectRepoName += originalRepoName[len(redirectRepoName)+5:]
		redirectPath := strings.Replace(
			ctx.Req.URL.EscapedPath(),
			url.PathEscape(userName)+"/"+url.PathEscape(originalRepoName),
			url.PathEscape(userName)+"/"+url.PathEscape(redirectRepoName)+"/wiki",
			1,
		)
		if ctx.Req.URL.RawQuery != "" {
			redirectPath += "?" + ctx.Req.URL.RawQuery
		}
		ctx.Redirect(path.Join(setting.AppSubURL, redirectPath))
		return nil
	}

	// Get repository.
	repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			redirectRepoID, err := repo_model.LookupRedirect(ctx, owner.ID, repoName)
			if err == nil {
				RedirectToRepo(ctx.Base, redirectRepoID)
			} else if repo_model.IsErrRedirectNotExist(err) {
				if ctx.FormString("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return nil
				}
				ctx.NotFound("GetRepositoryByName", nil)
			} else {
				ctx.ServerError("LookupRepoRedirect", err)
			}
		} else {
			ctx.ServerError("GetRepositoryByName", err)
		}
		return nil
	}
	repo.Owner = owner

	repoAssignment(ctx, repo)
	if ctx.Written() {
		return nil
	}

	ctx.Repo.RepoLink = repo.Link()
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["RepoRelPath"] = ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name

	if setting.Other.EnableFeed {
		ctx.Data["EnableFeed"] = true
		ctx.Data["FeedURL"] = ctx.Repo.RepoLink
	}

	unit, err := ctx.Repo.Repository.GetUnit(ctx, unit_model.TypeExternalTracker)
	if err == nil {
		ctx.Data["RepoExternalIssuesLink"] = unit.ExternalTrackerConfig().ExternalTrackerURL
	}

	ctx.Data["NumTags"], err = db.Count[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		HasSha1:       optional.Some(true), // only draft releases which are created with existing tags
		RepoID:        ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return nil
	}
	ctx.Data["NumReleases"], err = db.Count[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		IncludeDrafts: ctx.Repo.CanWrite(unit_model.TypeReleases),
		RepoID:        ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return nil
	}

	ctx.Data["Title"] = owner.Name + "/" + repo.Name
	ctx.Data["Repository"] = repo
	ctx.Data["Owner"] = ctx.Repo.Repository.Owner
	ctx.Data["IsRepositoryOwner"] = ctx.Repo.IsOwner()
	ctx.Data["IsRepositoryAdmin"] = ctx.Repo.IsAdmin()
	ctx.Data["RepoOwnerIsOrganization"] = repo.Owner.IsOrganization()
	ctx.Data["CanWriteCode"] = ctx.Repo.CanWrite(unit_model.TypeCode)
	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWrite(unit_model.TypeIssues)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWrite(unit_model.TypePullRequests)
	ctx.Data["CanWriteActions"] = ctx.Repo.CanWrite(unit_model.TypeActions)

	canSignedUserFork, err := repo_module.CanUserForkRepo(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("CanUserForkRepo", err)
		return nil
	}
	ctx.Data["CanSignedUserFork"] = canSignedUserFork

	userAndOrgForks, err := repo_model.GetForksByUserAndOrgs(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetForksByUserAndOrgs", err)
		return nil
	}
	ctx.Data["UserAndOrgForks"] = userAndOrgForks

	// canSignedUserFork is true if the current user doesn't have a fork of this repo yet or
	// if he owns an org that doesn't have a fork of this repo yet
	// If multiple forks are available or if the user can fork to another account, but there is already a fork: open selection dialog
	ctx.Data["ShowForkModal"] = len(userAndOrgForks) > 1 || (canSignedUserFork && len(userAndOrgForks) > 0)

	ctx.Data["RepoCloneLink"] = repo.CloneLink()

	cloneButtonShowHTTPS := !setting.Repository.DisableHTTPGit
	cloneButtonShowSSH := !setting.SSH.Disabled && (ctx.IsSigned || setting.SSH.ExposeAnonymous)
	if !cloneButtonShowHTTPS && !cloneButtonShowSSH {
		// We have to show at least one link, so we just show the HTTPS
		cloneButtonShowHTTPS = true
	}
	ctx.Data["CloneButtonShowHTTPS"] = cloneButtonShowHTTPS
	ctx.Data["CloneButtonShowSSH"] = cloneButtonShowSSH
	ctx.Data["CloneButtonOriginLink"] = ctx.Data["RepoCloneLink"] // it may be rewritten to the WikiCloneLink by the router middleware

	ctx.Data["RepoSearchEnabled"] = setting.Indexer.RepoIndexerEnabled
	if setting.Indexer.RepoIndexerEnabled {
		ctx.Data["CodeIndexerUnavailable"] = !code_indexer.IsAvailable(ctx)
	}

	if ctx.IsSigned {
		ctx.Data["IsWatchingRepo"] = repo_model.IsWatching(ctx, ctx.Doer.ID, repo.ID)
		ctx.Data["IsStaringRepo"] = repo_model.IsStaring(ctx, ctx.Doer.ID, repo.ID)
	}

	if repo.IsFork {
		RetrieveBaseRepo(ctx, repo)
		if ctx.Written() {
			return nil
		}
	}

	if repo.IsGenerated() {
		RetrieveTemplateRepo(ctx, repo)
		if ctx.Written() {
			return nil
		}
	}

	isHomeOrSettings := ctx.Link == ctx.Repo.RepoLink || ctx.Link == ctx.Repo.RepoLink+"/settings" || strings.HasPrefix(ctx.Link, ctx.Repo.RepoLink+"/settings/")

	// Disable everything when the repo is being created
	if ctx.Repo.Repository.IsBeingCreated() || ctx.Repo.Repository.IsBroken() {
		ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
		if !isHomeOrSettings {
			ctx.Redirect(ctx.Repo.RepoLink)
		}
		return nil
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		if strings.Contains(err.Error(), "repository does not exist") || strings.Contains(err.Error(), "no such file or directory") {
			log.Error("Repository %-v has a broken repository on the file system: %s Error: %v", ctx.Repo.Repository, ctx.Repo.Repository.RepoPath(), err)
			ctx.Repo.Repository.MarkAsBrokenEmpty()
			ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
			// Only allow access to base of repo or settings
			if !isHomeOrSettings {
				ctx.Redirect(ctx.Repo.RepoLink)
			}
			return nil
		}
		ctx.ServerError("RepoAssignment Invalid repo "+repo.FullName(), err)
		return nil
	}
	if ctx.Repo.GitRepo != nil {
		ctx.Repo.GitRepo.Close()
	}
	ctx.Repo.GitRepo = gitRepo

	// We opened it, we should close it
	cancel := func() {
		// If it's been set to nil then assume someone else has closed it.
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
		}
	}

	// Stop at this point when the repo is empty.
	if ctx.Repo.Repository.IsEmpty {
		ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
		return cancel
	}

	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
		ListOptions:     db.ListOptionsAll,
	}
	branchesTotal, err := db.Count[git_model.Branch](ctx, branchOpts)
	if err != nil {
		ctx.ServerError("CountBranches", err)
		return cancel
	}

	// non-empty repo should have at least 1 branch, so this repository's branches haven't been synced yet
	if branchesTotal == 0 { // fallback to do a sync immediately
		branchesTotal, err = repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0)
		if err != nil {
			ctx.ServerError("SyncRepoBranches", err)
			return cancel
		}
	}

	ctx.Data["BranchesCount"] = branchesTotal

	// If no branch is set in the request URL, try to guess a default one.
	if len(ctx.Repo.BranchName) == 0 {
		if len(ctx.Repo.Repository.DefaultBranch) > 0 && gitRepo.IsBranchExist(ctx.Repo.Repository.DefaultBranch) {
			ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
		} else {
			ctx.Repo.BranchName, _ = gitrepo.GetDefaultBranch(ctx, ctx.Repo.Repository)
			if ctx.Repo.BranchName == "" {
				// If it still can't get a default branch, fall back to default branch from setting.
				// Something might be wrong. Either site admin should fix the repo sync or Gitea should fix a potential bug.
				ctx.Repo.BranchName = setting.Repository.DefaultBranch
			}
		}
		ctx.Repo.RefName = ctx.Repo.BranchName
	}
	ctx.Data["BranchName"] = ctx.Repo.BranchName

	// People who have push access or have forked repository can propose a new pull request.
	canPush := ctx.Repo.CanWrite(unit_model.TypeCode) ||
		(ctx.IsSigned && repo_model.HasForkedRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID))
	canCompare := false

	// Pull request is allowed if this is a fork repository
	// and base repository accepts pull requests.
	if repo.BaseRepo != nil && repo.BaseRepo.AllowsPulls(ctx) {
		canCompare = true
		ctx.Data["BaseRepo"] = repo.BaseRepo
		ctx.Repo.PullRequest.BaseRepo = repo.BaseRepo
		ctx.Repo.PullRequest.Allowed = canPush
		ctx.Repo.PullRequest.HeadInfoSubURL = url.PathEscape(ctx.Repo.Owner.Name) + ":" + util.PathEscapeSegments(ctx.Repo.BranchName)
	} else if repo.AllowsPulls(ctx) {
		// Or, this is repository accepts pull requests between branches.
		canCompare = true
		ctx.Data["BaseRepo"] = repo
		ctx.Repo.PullRequest.BaseRepo = repo
		ctx.Repo.PullRequest.Allowed = canPush
		ctx.Repo.PullRequest.SameRepo = true
		ctx.Repo.PullRequest.HeadInfoSubURL = util.PathEscapeSegments(ctx.Repo.BranchName)
	}
	ctx.Data["CanCompareOrPull"] = canCompare
	ctx.Data["PullRequestCtx"] = ctx.Repo.PullRequest

	if ctx.Repo.Repository.Status == repo_model.RepositoryPendingTransfer {
		repoTransfer, err := models.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.ServerError("GetPendingRepositoryTransfer", err)
			return cancel
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadRecipient", err)
			return cancel
		}

		ctx.Data["RepoTransfer"] = repoTransfer
		if ctx.Doer != nil {
			ctx.Data["CanUserAcceptTransfer"] = repoTransfer.CanUserAcceptTransfer(ctx, ctx.Doer)
		}
	}

	if ctx.FormString("go-get") == "1" {
		ctx.Data["GoGetImport"] = ComposeGoGetImport(owner.Name, repo.Name)
		fullURLPrefix := repo.HTMLURL() + "/src/branch/" + util.PathEscapeSegments(ctx.Repo.BranchName)
		ctx.Data["GoDocDirectory"] = fullURLPrefix + "{/dir}"
		ctx.Data["GoDocFile"] = fullURLPrefix + "{/dir}/{file}#L{line}"
	}
	return cancel
}

// RepoRefType type of repo reference
type RepoRefType int

const (
	// RepoRefLegacy unknown type, make educated guess and redirect.
	// for backward compatibility with previous URL scheme
	RepoRefLegacy RepoRefType = iota
	// RepoRefAny is for usage where educated guess is needed
	// but redirect can not be made
	RepoRefAny
	// RepoRefBranch branch
	RepoRefBranch
	// RepoRefTag tag
	RepoRefTag
	// RepoRefCommit commit
	RepoRefCommit
	// RepoRefBlob blob
	RepoRefBlob
)

const headRefName = "HEAD"

// RepoRef handles repository reference names when the ref name is not
// explicitly given
func RepoRef() func(*Context) context.CancelFunc {
	// since no ref name is explicitly specified, ok to just use branch
	return RepoRefByType(RepoRefBranch)
}

// RefTypeIncludesBranches returns true if ref type can be a branch
func (rt RepoRefType) RefTypeIncludesBranches() bool {
	if rt == RepoRefLegacy || rt == RepoRefAny || rt == RepoRefBranch {
		return true
	}
	return false
}

// RefTypeIncludesTags returns true if ref type can be a tag
func (rt RepoRefType) RefTypeIncludesTags() bool {
	if rt == RepoRefLegacy || rt == RepoRefAny || rt == RepoRefTag {
		return true
	}
	return false
}

func getRefNameFromPath(repo *Repository, path string, isExist func(string) bool) string {
	refName := ""
	parts := strings.Split(path, "/")
	for i, part := range parts {
		refName = strings.TrimPrefix(refName+"/"+part, "/")
		if isExist(refName) {
			repo.TreePath = strings.Join(parts[i+1:], "/")
			return refName
		}
	}
	return ""
}

func getRefName(ctx *Base, repo *Repository, pathType RepoRefType) string {
	path := ctx.Params("*")
	switch pathType {
	case RepoRefLegacy, RepoRefAny:
		if refName := getRefName(ctx, repo, RepoRefBranch); len(refName) > 0 {
			return refName
		}
		if refName := getRefName(ctx, repo, RepoRefTag); len(refName) > 0 {
			return refName
		}
		// For legacy and API support only full commit sha
		parts := strings.Split(path, "/")

		if len(parts) > 0 && len(parts[0]) == git.ObjectFormatFromName(repo.Repository.ObjectFormatName).FullLength() {
			repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}
		if refName := getRefName(ctx, repo, RepoRefBlob); len(refName) > 0 {
			return refName
		}
		repo.TreePath = path
		return repo.Repository.DefaultBranch
	case RepoRefBranch:
		ref := getRefNameFromPath(repo, path, repo.GitRepo.IsBranchExist)
		if len(ref) == 0 {
			// check if ref is HEAD
			parts := strings.Split(path, "/")
			if parts[0] == headRefName {
				repo.TreePath = strings.Join(parts[1:], "/")
				return repo.Repository.DefaultBranch
			}

			// maybe it's a renamed branch
			return getRefNameFromPath(repo, path, func(s string) bool {
				b, exist, err := git_model.FindRenamedBranch(ctx, repo.Repository.ID, s)
				if err != nil {
					log.Error("FindRenamedBranch: %v", err)
					return false
				}

				if !exist {
					return false
				}

				ctx.Data["IsRenamedBranch"] = true
				ctx.Data["RenamedBranchName"] = b.To

				return true
			})
		}

		return ref
	case RepoRefTag:
		return getRefNameFromPath(repo, path, repo.GitRepo.IsTagExist)
	case RepoRefCommit:
		parts := strings.Split(path, "/")

		if len(parts) > 0 && len(parts[0]) >= 7 && len(parts[0]) <= repo.GetObjectFormat().FullLength() {
			repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}

		if len(parts) > 0 && parts[0] == headRefName {
			// HEAD ref points to last default branch commit
			commit, err := repo.GitRepo.GetBranchCommit(repo.Repository.DefaultBranch)
			if err != nil {
				return ""
			}
			repo.TreePath = strings.Join(parts[1:], "/")
			return commit.ID.String()
		}
	case RepoRefBlob:
		_, err := repo.GitRepo.GetBlob(path)
		if err != nil {
			return ""
		}
		return path
	default:
		log.Error("Unrecognized path type: %v", path)
	}
	return ""
}

// RepoRefByType handles repository reference name for a specific type
// of repository reference
func RepoRefByType(refType RepoRefType, ignoreNotExistErr ...bool) func(*Context) context.CancelFunc {
	return func(ctx *Context) (cancel context.CancelFunc) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsEmpty {
			// assume the user is viewing the (non-existent) default branch
			ctx.Repo.IsViewBranch = true
			ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
			ctx.Data["TreePath"] = ""
			return nil
		}

		var (
			refName string
			err     error
		)

		if ctx.Repo.GitRepo == nil {
			ctx.Repo.GitRepo, err = gitrepo.OpenRepository(ctx, ctx.Repo.Repository)
			if err != nil {
				ctx.ServerError(fmt.Sprintf("Open Repository %v failed", ctx.Repo.Repository.FullName()), err)
				return nil
			}
			// We opened it, we should close it
			cancel = func() {
				// If it's been set to nil then assume someone else has closed it.
				if ctx.Repo.GitRepo != nil {
					ctx.Repo.GitRepo.Close()
				}
			}
		}

		// Get default branch.
		if len(ctx.Params("*")) == 0 {
			refName = ctx.Repo.Repository.DefaultBranch
			if !ctx.Repo.GitRepo.IsBranchExist(refName) {
				brs, _, err := ctx.Repo.GitRepo.GetBranches(0, 1)
				if err == nil && len(brs) != 0 {
					refName = brs[0].Name
				} else if len(brs) == 0 {
					log.Error("No branches in non-empty repository %s", ctx.Repo.GitRepo.Path)
					ctx.Repo.Repository.MarkAsBrokenEmpty()
				} else {
					log.Error("GetBranches error: %v", err)
					ctx.Repo.Repository.MarkAsBrokenEmpty()
				}
			}
			ctx.Repo.RefName = refName
			ctx.Repo.BranchName = refName
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
			if err == nil {
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if strings.Contains(err.Error(), "fatal: not a git repository") || strings.Contains(err.Error(), "object does not exist") {
				// if the repository is broken, we can continue to the handler code, to show "Settings -> Delete Repository" for end users
				log.Error("GetBranchCommit: %v", err)
				ctx.Repo.Repository.MarkAsBrokenEmpty()
			} else {
				ctx.ServerError("GetBranchCommit", err)
				return cancel
			}
			ctx.Repo.IsViewBranch = true
		} else {
			refName = getRefName(ctx.Base, ctx.Repo, refType)
			ctx.Repo.RefName = refName
			isRenamedBranch, has := ctx.Data["IsRenamedBranch"].(bool)
			if isRenamedBranch && has {
				renamedBranchName := ctx.Data["RenamedBranchName"].(string)
				ctx.Flash.Info(ctx.Tr("repo.branch.renamed", refName, renamedBranchName))
				link := setting.AppSubURL + strings.Replace(ctx.Req.URL.EscapedPath(), util.PathEscapeSegments(refName), util.PathEscapeSegments(renamedBranchName), 1)
				ctx.Redirect(link)
				return cancel
			}

			if refType.RefTypeIncludesBranches() && ctx.Repo.GitRepo.IsBranchExist(refName) {
				ctx.Repo.IsViewBranch = true
				ctx.Repo.BranchName = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
				if err != nil {
					ctx.ServerError("GetBranchCommit", err)
					return cancel
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if refType.RefTypeIncludesTags() && ctx.Repo.GitRepo.IsTagExist(refName) {
				ctx.Repo.IsViewTag = true
				ctx.Repo.TagName = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refName)
				if err != nil {
					if git.IsErrNotExist(err) {
						ctx.NotFound("GetTagCommit", err)
						return cancel
					}
					ctx.ServerError("GetTagCommit", err)
					return cancel
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if len(refName) >= 7 && len(refName) <= ctx.Repo.GetObjectFormat().FullLength() {
				ctx.Repo.IsViewCommit = true
				ctx.Repo.CommitID = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refName)
				if err != nil {
					ctx.NotFound("GetCommit", err)
					return cancel
				}
				// If short commit ID add canonical link header
				if len(refName) < ctx.Repo.GetObjectFormat().FullLength() {
					ctx.RespHeader().Set("Link", fmt.Sprintf("<%s>; rel=\"canonical\"",
						util.URLJoin(setting.AppURL, strings.Replace(ctx.Req.URL.RequestURI(), util.PathEscapeSegments(refName), url.PathEscape(ctx.Repo.Commit.ID.String()), 1))))
				}
			} else {
				if len(ignoreNotExistErr) > 0 && ignoreNotExistErr[0] {
					return cancel
				}
				ctx.NotFound("RepoRef invalid repo", fmt.Errorf("branch or tag not exist: %s", refName))
				return cancel
			}

			if refType == RepoRefLegacy {
				// redirect from old URL scheme to new URL scheme
				prefix := strings.TrimPrefix(setting.AppSubURL+strings.ToLower(strings.TrimSuffix(ctx.Req.URL.Path, ctx.Params("*"))), strings.ToLower(ctx.Repo.RepoLink))

				ctx.Redirect(path.Join(
					ctx.Repo.RepoLink,
					util.PathEscapeSegments(prefix),
					ctx.Repo.BranchNameSubURL(),
					util.PathEscapeSegments(ctx.Repo.TreePath)))
				return cancel
			}
		}

		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["RefName"] = ctx.Repo.RefName
		ctx.Data["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
		ctx.Data["TagName"] = ctx.Repo.TagName
		ctx.Data["CommitID"] = ctx.Repo.CommitID
		ctx.Data["TreePath"] = ctx.Repo.TreePath
		ctx.Data["IsViewBranch"] = ctx.Repo.IsViewBranch
		ctx.Data["IsViewTag"] = ctx.Repo.IsViewTag
		ctx.Data["IsViewCommit"] = ctx.Repo.IsViewCommit
		ctx.Data["CanCreateBranch"] = ctx.Repo.CanCreateBranch()

		ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
		if err != nil {
			ctx.ServerError("GetCommitsCount", err)
			return cancel
		}
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		ctx.Repo.GitRepo.LastCommitCache = git.NewLastCommitCache(ctx.Repo.CommitsCount, ctx.Repo.Repository.FullName(), ctx.Repo.GitRepo, cache.GetCache())

		return cancel
	}
}

// GitHookService checks if repository Git hooks service has been enabled.
func GitHookService() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Doer.CanEditGitHook() {
			ctx.NotFound("GitHookService", nil)
			return
		}
	}
}
