// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	editorconfig "gopkg.in/editorconfig/editorconfig-core-go.v1"
	macaron "gopkg.in/macaron.v1"
)

// PullRequest contains informations to make a pull request
type PullRequest struct {
	BaseRepo *models.Repository
	Allowed  bool
	SameRepo bool
	HeadInfo string // [<user>:]<branch>
}

// Repository contains information to operate a repository
type Repository struct {
	AccessMode   models.AccessMode
	IsWatching   bool
	IsViewBranch bool
	IsViewTag    bool
	IsViewCommit bool
	Repository   *models.Repository
	Owner        *models.User
	Commit       *git.Commit
	Tag          *git.Tag
	GitRepo      *git.Repository
	BranchName   string
	TagName      string
	TreePath     string
	CommitID     string
	RepoLink     string
	CloneLink    models.CloneLink
	CommitsCount int64
	Mirror       *models.Mirror

	PullRequest *PullRequest
}

// IsOwner returns true if current user is the owner of repository.
func (r *Repository) IsOwner() bool {
	return r.AccessMode >= models.AccessModeOwner
}

// IsAdmin returns true if current user has admin or higher access of repository.
func (r *Repository) IsAdmin() bool {
	return r.AccessMode >= models.AccessModeAdmin
}

// IsWriter returns true if current user has write or higher access of repository.
func (r *Repository) IsWriter() bool {
	return r.AccessMode >= models.AccessModeWrite
}

// HasAccess returns true if the current user has at least read access for this repository
func (r *Repository) HasAccess() bool {
	return r.AccessMode >= models.AccessModeRead
}

// CanEnableEditor returns true if repository is editable and user has proper access level.
func (r *Repository) CanEnableEditor() bool {
	return r.Repository.CanEnableEditor() && r.IsViewBranch && r.IsWriter()
}

// CanCommitToBranch returns true if repository is editable and user has proper access level
//   and branch is not protected
func (r *Repository) CanCommitToBranch() (bool, error) {
	protectedBranch, err := r.Repository.IsProtectedBranch(r.BranchName)
	if err != nil {
		return false, err
	}
	return r.CanEnableEditor() && !protectedBranch, nil
}

// GetEditorconfig returns the .editorconfig definition if found in the
// HEAD of the default repo branch.
func (r *Repository) GetEditorconfig() (*editorconfig.Editorconfig, error) {
	commit, err := r.GitRepo.GetBranchCommit(r.Repository.DefaultBranch)
	if err != nil {
		return nil, err
	}
	treeEntry, err := commit.GetTreeEntryByPath(".editorconfig")
	if err != nil {
		return nil, err
	}
	reader, err := treeEntry.Blob().Data()
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return editorconfig.ParseBytes(data)
}

// RetrieveBaseRepo retrieves base repository
func RetrieveBaseRepo(ctx *Context, repo *models.Repository) {
	// Non-fork repository will not return error in this method.
	if err := repo.GetBaseRepo(); err != nil {
		if models.IsErrRepoNotExist(err) {
			repo.IsFork = false
			repo.ForkID = 0
			return
		}
		ctx.Handle(500, "GetBaseRepo", err)
		return
	} else if err = repo.BaseRepo.GetOwner(); err != nil {
		ctx.Handle(500, "BaseRepo.GetOwner", err)
		return
	}
}

// composeGoGetImport returns go-get-import meta content.
func composeGoGetImport(owner, repo string) string {
	return path.Join(setting.Domain, setting.AppSubURL, owner, repo)
}

// earlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func earlyResponseForGoGetMeta(ctx *Context) {
	ctx.PlainText(200, []byte(com.Expand(`<meta name="go-import" content="{GoGetImport} git {CloneLink}">`,
		map[string]string{
			"GoGetImport": composeGoGetImport(ctx.Params(":username"), strings.TrimSuffix(ctx.Params(":reponame"), ".git")),
			"CloneLink":   models.ComposeHTTPSCloneURL(ctx.Params(":username"), ctx.Params(":reponame")),
		})))
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Context, redirectRepoID int64) {
	ownerName := ctx.Params(":username")
	previousRepoName := ctx.Params(":reponame")

	repo, err := models.GetRepositoryByID(redirectRepoID)
	if err != nil {
		ctx.Handle(500, "GetRepositoryByID", err)
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.Path,
		fmt.Sprintf("%s/%s", ownerName, previousRepoName),
		fmt.Sprintf("%s/%s", ownerName, repo.Name),
		1,
	)
	ctx.Redirect(redirectPath)
}

// RepoAssignment returns a macaron to handle repository assignment
func RepoAssignment() macaron.Handler {
	return func(ctx *Context) {
		var (
			owner *models.User
			err   error
		)

		userName := ctx.Params(":username")
		repoName := ctx.Params(":reponame")

		// Check if the user is the same as the repository owner
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
			owner = ctx.User
		} else {
			owner, err = models.GetUserByName(userName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					if ctx.Query("go-get") == "1" {
						earlyResponseForGoGetMeta(ctx)
						return
					}
					ctx.Handle(404, "GetUserByName", err)
				} else {
					ctx.Handle(500, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner
		ctx.Data["Username"] = ctx.Repo.Owner.Name

		// Get repository.
		repo, err := models.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				redirectRepoID, err := models.LookupRepoRedirect(owner.ID, repoName)
				if err == nil {
					RedirectToRepo(ctx, redirectRepoID)
				} else if models.IsErrRepoRedirectNotExist(err) {
					if ctx.Query("go-get") == "1" {
						earlyResponseForGoGetMeta(ctx)
						return
					}
					ctx.Handle(404, "GetRepositoryByName", err)
				} else {
					ctx.Handle(500, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Handle(500, "GetRepositoryByName", err)
			}
			return
		}
		repo.Owner = owner

		// Admin has super access.
		if ctx.IsSigned && ctx.User.IsAdmin {
			ctx.Repo.AccessMode = models.AccessModeOwner
		} else {
			var userID int64
			if ctx.User != nil {
				userID = ctx.User.ID
			}
			mode, err := models.AccessLevel(userID, repo)
			if err != nil {
				ctx.Handle(500, "AccessLevel", err)
				return
			}
			ctx.Repo.AccessMode = mode
		}

		// Check access.
		if ctx.Repo.AccessMode == models.AccessModeNone {
			if ctx.Query("go-get") == "1" {
				earlyResponseForGoGetMeta(ctx)
				return
			}
			ctx.Handle(404, "no access right", err)
			return
		}
		ctx.Data["HasAccess"] = true

		if repo.IsMirror {
			ctx.Repo.Mirror, err = models.GetMirrorByRepoID(repo.ID)
			if err != nil {
				ctx.Handle(500, "GetMirror", err)
				return
			}
			ctx.Data["MirrorEnablePrune"] = ctx.Repo.Mirror.EnablePrune
			ctx.Data["MirrorInterval"] = ctx.Repo.Mirror.Interval
			ctx.Data["Mirror"] = ctx.Repo.Mirror
		}

		ctx.Repo.Repository = repo
		ctx.Data["RepoName"] = ctx.Repo.Repository.Name
		ctx.Data["IsBareRepo"] = ctx.Repo.Repository.IsBare

		gitRepo, err := git.OpenRepository(models.RepoPath(userName, repoName))
		if err != nil {
			ctx.Handle(500, "RepoAssignment Invalid repo "+models.RepoPath(userName, repoName), err)
			return
		}
		ctx.Repo.GitRepo = gitRepo
		ctx.Repo.RepoLink = repo.Link()
		ctx.Data["RepoLink"] = ctx.Repo.RepoLink
		ctx.Data["RepoRelPath"] = ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name

		tags, err := ctx.Repo.GitRepo.GetTags()
		if err != nil {
			ctx.Handle(500, "GetTags", err)
			return
		}
		ctx.Data["Tags"] = tags
		ctx.Repo.Repository.NumTags = len(tags)

		ctx.Data["Title"] = owner.Name + "/" + repo.Name
		ctx.Data["Repository"] = repo
		ctx.Data["Owner"] = ctx.Repo.Repository.Owner
		ctx.Data["IsRepositoryOwner"] = ctx.Repo.IsOwner()
		ctx.Data["IsRepositoryAdmin"] = ctx.Repo.IsAdmin()
		ctx.Data["IsRepositoryWriter"] = ctx.Repo.IsWriter()

		ctx.Data["DisableSSH"] = setting.SSH.Disabled
		ctx.Data["ExposeAnonSSH"] = setting.SSH.ExposeAnonymous
		ctx.Data["DisableHTTP"] = setting.Repository.DisableHTTPGit
		ctx.Data["CloneLink"] = repo.CloneLink()
		ctx.Data["WikiCloneLink"] = repo.WikiCloneLink()

		if ctx.IsSigned {
			ctx.Data["IsWatchingRepo"] = models.IsWatching(ctx.User.ID, repo.ID)
			ctx.Data["IsStaringRepo"] = models.IsStaring(ctx.User.ID, repo.ID)
		}

		// repo is bare and display enable
		if ctx.Repo.Repository.IsBare {
			ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
			return
		}

		ctx.Data["TagName"] = ctx.Repo.TagName
		brs, err := ctx.Repo.GitRepo.GetBranches()
		if err != nil {
			ctx.Handle(500, "GetBranches", err)
			return
		}
		ctx.Data["Branches"] = brs
		ctx.Data["BrancheCount"] = len(brs)

		// If not branch selected, try default one.
		// If default branch doesn't exists, fall back to some other branch.
		if len(ctx.Repo.BranchName) == 0 {
			if len(ctx.Repo.Repository.DefaultBranch) > 0 && gitRepo.IsBranchExist(ctx.Repo.Repository.DefaultBranch) {
				ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
			} else if len(brs) > 0 {
				ctx.Repo.BranchName = brs[0]
			}
		}
		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["CommitID"] = ctx.Repo.CommitID

		if repo.IsFork {
			RetrieveBaseRepo(ctx, repo)
			if ctx.Written() {
				return
			}
		}

		// People who have push access or have forked repository can propose a new pull request.
		if ctx.Repo.IsWriter() || (ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID)) {
			// Pull request is allowed if this is a fork repository
			// and base repository accepts pull requests.
			if repo.BaseRepo != nil && repo.BaseRepo.AllowsPulls() {
				ctx.Data["BaseRepo"] = repo.BaseRepo
				ctx.Repo.PullRequest.BaseRepo = repo.BaseRepo
				ctx.Repo.PullRequest.Allowed = true
				ctx.Repo.PullRequest.HeadInfo = ctx.Repo.Owner.Name + ":" + ctx.Repo.BranchName
			} else {
				// Or, this is repository accepts pull requests between branches.
				if repo.AllowsPulls() {
					ctx.Data["BaseRepo"] = repo
					ctx.Repo.PullRequest.BaseRepo = repo
					ctx.Repo.PullRequest.Allowed = true
					ctx.Repo.PullRequest.SameRepo = true
					ctx.Repo.PullRequest.HeadInfo = ctx.Repo.BranchName
				}
			}

			// Reset repo units as otherwise user specific units wont be loaded later
			ctx.Repo.Repository.Units = nil
		}
		ctx.Data["PullRequestCtx"] = ctx.Repo.PullRequest

		if ctx.Query("go-get") == "1" {
			ctx.Data["GoGetImport"] = composeGoGetImport(owner.Name, repo.Name)
			prefix := setting.AppURL + path.Join(owner.Name, repo.Name, "src", ctx.Repo.BranchName)
			ctx.Data["GoDocDirectory"] = prefix + "{/dir}"
			ctx.Data["GoDocFile"] = prefix + "{/dir}/{file}#L{line}"
		}
	}
}

// RepoRef handles repository reference name including those contain `/`.
func RepoRef() macaron.Handler {
	return func(ctx *Context) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsBare {
			return
		}

		var (
			refName string
			err     error
		)

		// For API calls.
		if ctx.Repo.GitRepo == nil {
			repoPath := models.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
			ctx.Repo.GitRepo, err = git.OpenRepository(repoPath)
			if err != nil {
				ctx.Handle(500, "RepoRef Invalid repo "+repoPath, err)
				return
			}
		}

		// Get default branch.
		if len(ctx.Params("*")) == 0 {
			refName = ctx.Repo.Repository.DefaultBranch
			if !ctx.Repo.GitRepo.IsBranchExist(refName) {
				brs, err := ctx.Repo.GitRepo.GetBranches()
				if err != nil {
					ctx.Handle(500, "GetBranches", err)
					return
				} else if len(brs) == 0 {
					err = fmt.Errorf("No branches in non-bare repository %s",
						ctx.Repo.GitRepo.Path)
					ctx.Handle(500, "GetBranches", err)
					return
				}
				refName = brs[0]
			}
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
			if err != nil {
				ctx.Handle(500, "GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			ctx.Repo.IsViewBranch = true

		} else {
			hasMatched := false
			parts := strings.Split(ctx.Params("*"), "/")
			for i, part := range parts {
				refName = strings.TrimPrefix(refName+"/"+part, "/")

				if ctx.Repo.GitRepo.IsBranchExist(refName) ||
					ctx.Repo.GitRepo.IsTagExist(refName) {
					if i < len(parts)-1 {
						ctx.Repo.TreePath = strings.Join(parts[i+1:], "/")
					}
					hasMatched = true
					break
				}
			}
			if !hasMatched && len(parts[0]) == 40 {
				refName = parts[0]
				ctx.Repo.TreePath = strings.Join(parts[1:], "/")
			}

			if ctx.Repo.GitRepo.IsBranchExist(refName) {
				ctx.Repo.IsViewBranch = true

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
				if err != nil {
					ctx.Handle(500, "GetBranchCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()

			} else if ctx.Repo.GitRepo.IsTagExist(refName) {
				ctx.Repo.IsViewTag = true
				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refName)
				if err != nil {
					ctx.Handle(500, "GetTagCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if len(refName) == 40 {
				ctx.Repo.IsViewCommit = true
				ctx.Repo.CommitID = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refName)
				if err != nil {
					ctx.Handle(404, "GetCommit", nil)
					return
				}
			} else {
				ctx.Handle(404, "RepoRef invalid repo", fmt.Errorf("branch or tag not exist: %s", refName))
				return
			}
		}

		ctx.Repo.BranchName = refName
		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["CommitID"] = ctx.Repo.CommitID
		ctx.Data["TreePath"] = ctx.Repo.TreePath
		ctx.Data["IsViewBranch"] = ctx.Repo.IsViewBranch
		ctx.Data["IsViewTag"] = ctx.Repo.IsViewTag
		ctx.Data["IsViewCommit"] = ctx.Repo.IsViewCommit

		ctx.Repo.CommitsCount, err = ctx.Repo.Commit.CommitsCount()
		if err != nil {
			ctx.Handle(500, "CommitsCount", err)
			return
		}
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
	}
}

// RequireRepoAdmin returns a macaron middleware for requiring repository admin permission
func RequireRepoAdmin() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || (!ctx.Repo.IsAdmin() && !ctx.User.IsAdmin) {
			ctx.Handle(404, ctx.Req.RequestURI, nil)
			return
		}
	}
}

// RequireRepoWriter returns a macaron middleware for requiring repository write permission
func RequireRepoWriter() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.IsSigned || (!ctx.Repo.IsWriter() && !ctx.User.IsAdmin) {
			ctx.Handle(404, ctx.Req.RequestURI, nil)
			return
		}
	}
}

// LoadRepoUnits loads repsitory's units, it should be called after repository and user loaded
func LoadRepoUnits() macaron.Handler {
	return func(ctx *Context) {
		var isAdmin bool
		if ctx.User != nil && ctx.User.IsAdmin {
			isAdmin = true
		}

		var userID int64
		if ctx.User != nil {
			userID = ctx.User.ID
		}
		err := ctx.Repo.Repository.LoadUnitsByUserID(userID, isAdmin)
		if err != nil {
			ctx.Handle(500, "LoadUnitsByUserID", err)
			return
		}
	}
}

// CheckUnit will check whether
func CheckUnit(unitType models.UnitType) macaron.Handler {
	return func(ctx *Context) {
		var find bool
		for _, unit := range ctx.Repo.Repository.Units {
			if unit.Type == unitType {
				find = true
				break
			}
		}
		if !find {
			ctx.Handle(404, "CheckUnit", fmt.Errorf("%s: %v", ctx.Tr("units.error.unit_not_allowed"), unitType))
		}
	}
}

// GitHookService checks if repository Git hooks service has been enabled.
func GitHookService() macaron.Handler {
	return func(ctx *Context) {
		if !ctx.User.CanEditGitHook() {
			ctx.Handle(404, "GitHookService", nil)
			return
		}
	}
}

// UnitTypes returns a macaron middleware to set unit types to context variables.
func UnitTypes() macaron.Handler {
	return func(ctx *Context) {
		ctx.Data["UnitTypeCode"] = models.UnitTypeCode
		ctx.Data["UnitTypeIssues"] = models.UnitTypeIssues
		ctx.Data["UnitTypePullRequests"] = models.UnitTypePullRequests
		ctx.Data["UnitTypeReleases"] = models.UnitTypeReleases
		ctx.Data["UnitTypeWiki"] = models.UnitTypeWiki
		ctx.Data["UnitTypeExternalWiki"] = models.UnitTypeExternalWiki
		ctx.Data["UnitTypeExternalTracker"] = models.UnitTypeExternalTracker
	}
}
