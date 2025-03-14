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
	"code.gitea.io/gitea/modules/httplib"
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
	BaseRepo *repo_model.Repository
	Allowed  bool // it only used by the web tmpl: "PullRequestCtx.Allowed"
	SameRepo bool // it only used by the web tmpl: "PullRequestCtx.SameRepo"
}

// Repository contains information to operate a repository
type Repository struct {
	access_model.Permission

	Repository *repo_model.Repository
	Owner      *user_model.User

	RepoLink string
	GitRepo  *git.Repository

	// RefFullName is the full ref name that the user is viewing
	RefFullName git.RefName
	BranchName  string // it is the RefFullName's short name if its type is "branch"
	TreePath    string

	// Commit it is always set to the commit for the branch or tag, or just the commit that the user is viewing
	Commit       *git.Commit
	CommitID     string
	CommitsCount int64

	PullRequest *PullRequest
}

// CanWriteToBranch checks if the branch is writable by the user
func (r *Repository) CanWriteToBranch(ctx context.Context, user *user_model.User, branch string) bool {
	return issues_model.CanMaintainerWriteToBranch(ctx, r.Permission, branch, user)
}

// CanEnableEditor returns true if repository is editable and user has proper access level.
func (r *Repository) CanEnableEditor(ctx context.Context, user *user_model.User) bool {
	return r.RefFullName.IsBranch() && r.CanWriteToBranch(ctx, user, r.BranchName) && r.Repository.CanEnableEditor() && !r.Repository.IsArchived
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
			ctx.NotFound(errors.New(ctx.Locale.TrString("repo.archive.title")))
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

// CanUseTimetracker returns whether a user can use the timetracker.
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
	contextName := r.RefFullName.ShortName()
	isRef := r.RefFullName.IsBranch() || r.RefFullName.IsTag()
	return cache.GetInt64(r.Repository.GetCommitsCountCacheKey(contextName, isRef), func() (int64, error) {
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

// RefTypeNameSubURL makes a sub-url for the current ref (branch/tag/commit) field, for example:
// * "branch/master"
// * "tag/v1.0.0"
// * "commit/123456"
// It is usually used to construct a link like ".../src/{{RefTypeNameSubURL}}/{{PathEscapeSegments TreePath}}"
func (r *Repository) RefTypeNameSubURL() string {
	return r.RefFullName.RefWebLinkPath()
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
func ComposeGoGetImport(ctx context.Context, owner, repo string) string {
	curAppURL, _ := url.Parse(httplib.GuessCurrentAppURL(ctx))
	return path.Join(curAppURL.Host, setting.AppSubURL, url.PathEscape(owner), url.PathEscape(repo))
}

// EarlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func EarlyResponseForGoGetMeta(ctx *Context) {
	username := ctx.PathParam("username")
	reponame := strings.TrimSuffix(ctx.PathParam("reponame"), ".git")
	if username == "" || reponame == "" {
		ctx.PlainText(http.StatusBadRequest, "invalid repository path")
		return
	}

	var cloneURL string
	if setting.Repository.GoGetCloneURLProtocol == "ssh" {
		cloneURL = repo_model.ComposeSSHCloneURL(ctx.Doer, username, reponame)
	} else {
		cloneURL = repo_model.ComposeHTTPSCloneURL(ctx, username, reponame)
	}
	goImportContent := fmt.Sprintf("%s git %s", ComposeGoGetImport(ctx, username, reponame), cloneURL)
	htmlMeta := fmt.Sprintf(`<meta name="go-import" content="%s">`, html.EscapeString(goImportContent))
	ctx.PlainText(http.StatusOK, htmlMeta)
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Base, redirectRepoID int64) {
	ownerName := ctx.PathParam("username")
	previousRepoName := ctx.PathParam("reponame")

	repo, err := repo_model.GetRepositoryByID(ctx, redirectRepoID)
	if err != nil {
		log.Error("GetRepositoryByID: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "GetRepositoryByID")
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

	if !ctx.Repo.Permission.HasAnyUnitAccessOrEveryoneAccess() && !canWriteAsMaintainer(ctx) {
		if ctx.FormString("go-get") == "1" {
			EarlyResponseForGoGetMeta(ctx)
			return
		}
		ctx.NotFound(nil)
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

	ctx.Repo.Repository = repo
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["IsEmptyRepo"] = ctx.Repo.Repository.IsEmpty
}

// RepoAssignment returns a middleware to handle repository assignment
func RepoAssignment(ctx *Context) {
	if ctx.Data["Repository"] != nil {
		setting.PanicInDevOrTesting("RepoAssignment should not be executed twice")
	}

	var err error
	userName := ctx.PathParam("username")
	repoName := ctx.PathParam("reponame")
	repoName = strings.TrimSuffix(repoName, ".git")
	if setting.Other.EnableFeed {
		ctx.Data["EnableFeed"] = true
		repoName = strings.TrimSuffix(repoName, ".rss")
		repoName = strings.TrimSuffix(repoName, ".atom")
	}

	// Check if the user is the same as the repository owner
	if ctx.IsSigned && ctx.Doer.LowerName == strings.ToLower(userName) {
		ctx.Repo.Owner = ctx.Doer
	} else {
		ctx.Repo.Owner, err = user_model.GetUserByName(ctx, userName)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				// go-get does not support redirects
				// https://github.com/golang/go/issues/19760
				if ctx.FormString("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return
				}

				if redirectUserID, err := user_model.LookupUserRedirect(ctx, userName); err == nil {
					RedirectToUser(ctx.Base, userName, redirectUserID)
				} else if user_model.IsErrUserRedirectNotExist(err) {
					ctx.NotFound(nil)
				} else {
					ctx.ServerError("LookupUserRedirect", err)
				}
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return
		}
	}
	ctx.ContextUser = ctx.Repo.Owner
	ctx.Data["ContextUser"] = ctx.ContextUser

	// redirect link to wiki
	if strings.HasSuffix(repoName, ".wiki") {
		// ctx.Req.URL.Path does not have the preceding appSubURL - any redirect must have this added
		// Now we happen to know that all of our paths are: /:username/:reponame/whatever_else
		originalRepoName := ctx.PathParam("reponame")
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
		return
	}

	// Get repository.
	repo, err := repo_model.GetRepositoryByName(ctx, ctx.Repo.Owner.ID, repoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			redirectRepoID, err := repo_model.LookupRedirect(ctx, ctx.Repo.Owner.ID, repoName)
			if err == nil {
				RedirectToRepo(ctx.Base, redirectRepoID)
			} else if repo_model.IsErrRedirectNotExist(err) {
				if ctx.FormString("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return
				}
				ctx.NotFound(nil)
			} else {
				ctx.ServerError("LookupRepoRedirect", err)
			}
		} else {
			ctx.ServerError("GetRepositoryByName", err)
		}
		return
	}
	repo.Owner = ctx.Repo.Owner

	repoAssignment(ctx, repo)
	if ctx.Written() {
		return
	}

	ctx.Repo.RepoLink = repo.Link()
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["FeedURL"] = ctx.Repo.RepoLink

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
		return
	}
	ctx.Data["NumReleases"], err = db.Count[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		IncludeDrafts: ctx.Repo.CanWrite(unit_model.TypeReleases),
		RepoID:        ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}

	ctx.Data["Title"] = repo.Owner.Name + "/" + repo.Name
	ctx.Data["Repository"] = repo
	ctx.Data["Owner"] = ctx.Repo.Repository.Owner
	ctx.Data["CanWriteCode"] = ctx.Repo.CanWrite(unit_model.TypeCode)
	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWrite(unit_model.TypeIssues)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWrite(unit_model.TypePullRequests)
	ctx.Data["CanWriteActions"] = ctx.Repo.CanWrite(unit_model.TypeActions)

	canSignedUserFork, err := repo_module.CanUserForkRepo(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("CanUserForkRepo", err)
		return
	}
	ctx.Data["CanSignedUserFork"] = canSignedUserFork

	userAndOrgForks, err := repo_model.GetForksByUserAndOrgs(ctx, ctx.Doer, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetForksByUserAndOrgs", err)
		return
	}
	ctx.Data["UserAndOrgForks"] = userAndOrgForks

	// canSignedUserFork is true if the current user doesn't have a fork of this repo yet or
	// if he owns an org that doesn't have a fork of this repo yet
	// If multiple forks are available or if the user can fork to another account, but there is already a fork: open selection dialog
	ctx.Data["ShowForkModal"] = len(userAndOrgForks) > 1 || (canSignedUserFork && len(userAndOrgForks) > 0)

	ctx.Data["RepoCloneLink"] = repo.CloneLink(ctx, ctx.Doer)

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
			return
		}
	}

	if repo.IsGenerated() {
		RetrieveTemplateRepo(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	isHomeOrSettings := ctx.Link == ctx.Repo.RepoLink ||
		ctx.Link == ctx.Repo.RepoLink+"/settings" ||
		strings.HasPrefix(ctx.Link, ctx.Repo.RepoLink+"/settings/") ||
		ctx.Link == ctx.Repo.RepoLink+"/-/migrate/status"

	// Disable everything when the repo is being created
	if ctx.Repo.Repository.IsBeingCreated() || ctx.Repo.Repository.IsBroken() {
		if !isHomeOrSettings {
			ctx.Redirect(ctx.Repo.RepoLink)
		}
		return
	}

	if ctx.Repo.GitRepo != nil {
		setting.PanicInDevOrTesting("RepoAssignment: GitRepo should be nil")
		_ = ctx.Repo.GitRepo.Close()
		ctx.Repo.GitRepo = nil
	}

	ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		if strings.Contains(err.Error(), "repository does not exist") || strings.Contains(err.Error(), "no such file or directory") {
			log.Error("Repository %-v has a broken repository on the file system: %s Error: %v", ctx.Repo.Repository, ctx.Repo.Repository.RepoPath(), err)
			ctx.Repo.Repository.MarkAsBrokenEmpty()
			// Only allow access to base of repo or settings
			if !isHomeOrSettings {
				ctx.Redirect(ctx.Repo.RepoLink)
			}
			return
		}
		ctx.ServerError("RepoAssignment Invalid repo "+repo.FullName(), err)
		return
	}

	// Stop at this point when the repo is empty.
	if ctx.Repo.Repository.IsEmpty {
		return
	}

	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
		ListOptions:     db.ListOptionsAll,
	}
	branchesTotal, err := db.Count[git_model.Branch](ctx, branchOpts)
	if err != nil {
		ctx.ServerError("CountBranches", err)
		return
	}

	// non-empty repo should have at least 1 branch, so this repository's branches haven't been synced yet
	if branchesTotal == 0 { // fallback to do a sync immediately
		branchesTotal, err = repo_module.SyncRepoBranches(ctx, ctx.Repo.Repository.ID, 0)
		if err != nil {
			ctx.ServerError("SyncRepoBranches", err)
			return
		}
	}

	ctx.Data["BranchesCount"] = branchesTotal

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
	} else if repo.AllowsPulls(ctx) {
		// Or, this is repository accepts pull requests between branches.
		canCompare = true
		ctx.Data["BaseRepo"] = repo
		ctx.Repo.PullRequest.BaseRepo = repo
		ctx.Repo.PullRequest.Allowed = canPush
		ctx.Repo.PullRequest.SameRepo = true
	}
	ctx.Data["CanCompareOrPull"] = canCompare
	ctx.Data["PullRequestCtx"] = ctx.Repo.PullRequest

	if ctx.Repo.Repository.Status == repo_model.RepositoryPendingTransfer {
		repoTransfer, err := repo_model.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.ServerError("GetPendingRepositoryTransfer", err)
			return
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadRecipient", err)
			return
		}

		ctx.Data["RepoTransfer"] = repoTransfer
		if ctx.Doer != nil {
			ctx.Data["CanUserAcceptOrRejectTransfer"] = repoTransfer.CanUserAcceptOrRejectTransfer(ctx, ctx.Doer)
		}
	}

	if ctx.FormString("go-get") == "1" {
		ctx.Data["GoGetImport"] = ComposeGoGetImport(ctx, repo.Owner.Name, repo.Name)
		fullURLPrefix := repo.HTMLURL() + "/src/branch/" + util.PathEscapeSegments(ctx.Repo.BranchName)
		ctx.Data["GoDocDirectory"] = fullURLPrefix + "{/dir}"
		ctx.Data["GoDocFile"] = fullURLPrefix + "{/dir}/{file}#L{line}"
	}
}

const headRefName = "HEAD"

func RepoRef() func(*Context) {
	// old code does: return RepoRefByType(git.RefTypeBranch)
	// in most cases, it is an abuse, so we just disable it completely and fix the abuses one by one (if there is anything wrong)
	return nil
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

func getRefNameLegacy(ctx *Base, repo *Repository, reqPath, extraRef string) (refName string, refType git.RefType, fallbackDefaultBranch bool) {
	reqRefPath := path.Join(extraRef, reqPath)
	reqRefPathParts := strings.Split(reqRefPath, "/")
	if refName := getRefName(ctx, repo, reqRefPath, git.RefTypeBranch); refName != "" {
		return refName, git.RefTypeBranch, false
	}
	if refName := getRefName(ctx, repo, reqRefPath, git.RefTypeTag); refName != "" {
		return refName, git.RefTypeTag, false
	}
	if git.IsStringLikelyCommitID(git.ObjectFormatFromName(repo.Repository.ObjectFormatName), reqRefPathParts[0]) {
		// FIXME: this logic is different from other types. Ideally, it should also try to GetCommit to check if it exists
		repo.TreePath = strings.Join(reqRefPathParts[1:], "/")
		return reqRefPathParts[0], git.RefTypeCommit, false
	}
	// FIXME: the old code falls back to default branch if "ref" doesn't exist, there could be an edge case:
	// "README?ref=no-such" would read the README file from the default branch, but the user might expect a 404
	repo.TreePath = reqPath
	return repo.Repository.DefaultBranch, git.RefTypeBranch, true
}

func getRefName(ctx *Base, repo *Repository, path string, refType git.RefType) string {
	switch refType {
	case git.RefTypeBranch:
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
	case git.RefTypeTag:
		return getRefNameFromPath(repo, path, repo.GitRepo.IsTagExist)
	case git.RefTypeCommit:
		parts := strings.Split(path, "/")
		if git.IsStringLikelyCommitID(repo.GetObjectFormat(), parts[0], 7) {
			// FIXME: this logic is different from other types. Ideally, it should also try to GetCommit to check if it exists
			repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}

		if parts[0] == headRefName {
			// HEAD ref points to last default branch commit
			commit, err := repo.GitRepo.GetBranchCommit(repo.Repository.DefaultBranch)
			if err != nil {
				return ""
			}
			repo.TreePath = strings.Join(parts[1:], "/")
			return commit.ID.String()
		}
	default:
		panic(fmt.Sprintf("Unrecognized ref type: %v", refType))
	}
	return ""
}

func repoRefFullName(typ git.RefType, shortName string) git.RefName {
	switch typ {
	case git.RefTypeBranch:
		return git.RefNameFromBranch(shortName)
	case git.RefTypeTag:
		return git.RefNameFromTag(shortName)
	case git.RefTypeCommit:
		return git.RefNameFromCommit(shortName)
	default:
		setting.PanicInDevOrTesting("Unknown RepoRefType: %v", typ)
		return git.RefNameFromBranch("main") // just a dummy result, it shouldn't happen
	}
}

func RepoRefByDefaultBranch() func(*Context) {
	return func(ctx *Context) {
		ctx.Repo.RefFullName = git.RefNameFromBranch(ctx.Repo.Repository.DefaultBranch)
		ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
		ctx.Repo.Commit, _ = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.BranchName)
		ctx.Repo.CommitsCount, _ = ctx.Repo.GetCommitsCount()
		ctx.Data["RefFullName"] = ctx.Repo.RefFullName
		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
	}
}

// RepoRefByType handles repository reference name for a specific type
// of repository reference
func RepoRefByType(detectRefType git.RefType) func(*Context) {
	return func(ctx *Context) {
		var err error
		refType := detectRefType
		if ctx.Repo.Repository.IsBeingCreated() {
			return // no git repo, so do nothing, users will see a "migrating" UI provided by "migrate/migrating.tmpl"
		}
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsEmpty {
			// assume the user is viewing the (non-existent) default branch
			ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
			ctx.Repo.RefFullName = git.RefNameFromBranch(ctx.Repo.BranchName)
			// these variables are used by the template to "add/upload" new files
			ctx.Data["BranchName"] = ctx.Repo.BranchName
			ctx.Data["TreePath"] = ""
			return
		}

		// Get default branch.
		var refShortName string
		reqPath := ctx.PathParam("*")
		if reqPath == "" {
			refShortName = ctx.Repo.Repository.DefaultBranch
			if !ctx.Repo.GitRepo.IsBranchExist(refShortName) {
				brs, _, err := ctx.Repo.GitRepo.GetBranches(0, 1)
				if err == nil && len(brs) != 0 {
					refShortName = brs[0].Name
				} else if len(brs) == 0 {
					log.Error("No branches in non-empty repository %s", ctx.Repo.GitRepo.Path)
				} else {
					log.Error("GetBranches error: %v", err)
				}
			}
			ctx.Repo.RefFullName = git.RefNameFromBranch(refShortName)
			ctx.Repo.BranchName = refShortName
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refShortName)
			if err == nil {
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if strings.Contains(err.Error(), "fatal: not a git repository") || strings.Contains(err.Error(), "object does not exist") {
				// if the repository is broken, we can continue to the handler code, to show "Settings -> Delete Repository" for end users
				log.Error("GetBranchCommit: %v", err)
			} else {
				ctx.ServerError("GetBranchCommit", err)
				return
			}
		} else { // there is a path in request
			guessLegacyPath := refType == ""
			fallbackDefaultBranch := false
			if guessLegacyPath {
				refShortName, refType, fallbackDefaultBranch = getRefNameLegacy(ctx.Base, ctx.Repo, reqPath, "")
			} else {
				refShortName = getRefName(ctx.Base, ctx.Repo, reqPath, refType)
			}
			ctx.Repo.RefFullName = repoRefFullName(refType, refShortName)
			isRenamedBranch, has := ctx.Data["IsRenamedBranch"].(bool)
			if isRenamedBranch && has {
				renamedBranchName := ctx.Data["RenamedBranchName"].(string)
				ctx.Flash.Info(ctx.Tr("repo.branch.renamed", refShortName, renamedBranchName))
				link := setting.AppSubURL + strings.Replace(ctx.Req.URL.EscapedPath(), util.PathEscapeSegments(refShortName), util.PathEscapeSegments(renamedBranchName), 1)
				ctx.Redirect(link)
				return
			}

			if refType == git.RefTypeBranch && ctx.Repo.GitRepo.IsBranchExist(refShortName) {
				ctx.Repo.BranchName = refShortName
				ctx.Repo.RefFullName = git.RefNameFromBranch(refShortName)

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refShortName)
				if err != nil {
					ctx.ServerError("GetBranchCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if refType == git.RefTypeTag && ctx.Repo.GitRepo.IsTagExist(refShortName) {
				ctx.Repo.RefFullName = git.RefNameFromTag(refShortName)

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refShortName)
				if err != nil {
					if git.IsErrNotExist(err) {
						ctx.NotFound(err)
						return
					}
					ctx.ServerError("GetTagCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if git.IsStringLikelyCommitID(ctx.Repo.GetObjectFormat(), refShortName, 7) {
				ctx.Repo.RefFullName = git.RefNameFromCommit(refShortName)
				ctx.Repo.CommitID = refShortName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refShortName)
				if err != nil {
					ctx.NotFound(err)
					return
				}
				// If short commit ID add canonical link header
				if len(refShortName) < ctx.Repo.GetObjectFormat().FullLength() {
					canonicalURL := util.URLJoin(httplib.GuessCurrentAppURL(ctx), strings.Replace(ctx.Req.URL.RequestURI(), util.PathEscapeSegments(refShortName), url.PathEscape(ctx.Repo.Commit.ID.String()), 1))
					ctx.RespHeader().Set("Link", fmt.Sprintf(`<%s>; rel="canonical"`, canonicalURL))
				}
			} else {
				ctx.NotFound(fmt.Errorf("branch or tag not exist: %s", refShortName))
				return
			}

			if guessLegacyPath {
				// redirect from old URL scheme to new URL scheme
				// * /user2/repo1/commits/master => /user2/repo1/commits/branch/master
				// * /user2/repo1/src/master => /user2/repo1/src/branch/master
				// * /user2/repo1/src/README.md => /user2/repo1/src/branch/master/README.md (fallback to default branch)
				var redirect string
				refSubPath := "src"
				// remove the "/subpath/owner/repo/" prefix, the names are case-insensitive
				remainingLowerPath, cut := strings.CutPrefix(setting.AppSubURL+strings.ToLower(ctx.Req.URL.Path), strings.ToLower(ctx.Repo.RepoLink)+"/")
				if cut {
					refSubPath, _, _ = strings.Cut(remainingLowerPath, "/") // it could be "src" or "commits"
				}
				if fallbackDefaultBranch {
					redirect = fmt.Sprintf("%s/%s/%s/%s/%s", ctx.Repo.RepoLink, refSubPath, refType, util.PathEscapeSegments(refShortName), ctx.PathParamRaw("*"))
				} else {
					redirect = fmt.Sprintf("%s/%s/%s/%s", ctx.Repo.RepoLink, refSubPath, refType, ctx.PathParamRaw("*"))
				}
				if ctx.Req.URL.RawQuery != "" {
					redirect += "?" + ctx.Req.URL.RawQuery
				}
				ctx.Redirect(redirect)
				return
			}
		}

		ctx.Data["RefFullName"] = ctx.Repo.RefFullName
		ctx.Data["RefTypeNameSubURL"] = ctx.Repo.RefTypeNameSubURL()
		ctx.Data["TreePath"] = ctx.Repo.TreePath

		ctx.Data["BranchName"] = ctx.Repo.BranchName

		ctx.Data["CommitID"] = ctx.Repo.CommitID

		ctx.Data["CanCreateBranch"] = ctx.Repo.CanCreateBranch() // only used by the branch selector dropdown: AllowCreateNewRef

		ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
		if err != nil {
			ctx.ServerError("GetCommitsCount", err)
			return
		}
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		ctx.Repo.GitRepo.LastCommitCache = git.NewLastCommitCache(ctx.Repo.CommitsCount, ctx.Repo.Repository.FullName(), ctx.Repo.GitRepo, cache.GetCache())
	}
}

// GitHookService checks if repository Git hooks service has been enabled.
func GitHookService() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.Doer.CanEditGitHook() {
			ctx.NotFound(nil)
			return
		}
	}
}

// canWriteAsMaintainer check if the doer can write to a branch as a maintainer
func canWriteAsMaintainer(ctx *Context) bool {
	branchName := getRefNameFromPath(ctx.Repo, ctx.PathParam("*"), func(branchName string) bool {
		return issues_model.CanMaintainerWriteToBranch(ctx, ctx.Repo.Permission, branchName, ctx.Doer)
	})
	return len(branchName) > 0
}
