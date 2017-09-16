// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Unknwon/com"

	"code.gitea.io/git"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplCreate  base.TplName = "repo/create"
	tplMigrate base.TplName = "repo/migrate"
)

// MustBeNotBare render when a repo is a bare git dir
func MustBeNotBare(ctx *context.Context) {
	if ctx.Repo.Repository.IsBare {
		ctx.Handle(404, "MustBeNotBare", nil)
	}
}

func checkContextUser(ctx *context.Context, uid int64) *models.User {
	orgs, err := models.GetOwnedOrgsByUserIDDesc(ctx.User.ID, "updated_unix")
	if err != nil {
		ctx.Handle(500, "GetOwnedOrgsByUserIDDesc", err)
		return nil
	}
	ctx.Data["Orgs"] = orgs

	// Not equal means current user is an organization.
	if uid == ctx.User.ID || uid == 0 {
		return ctx.User
	}

	org, err := models.GetUserByID(uid)
	if models.IsErrUserNotExist(err) {
		return ctx.User
	}

	if err != nil {
		ctx.Handle(500, "GetUserByID", fmt.Errorf("[%d]: %v", uid, err))
		return nil
	}

	// Check ownership of organization.
	if !org.IsOrganization() || !(ctx.User.IsAdmin || org.IsOwnedBy(ctx.User.ID)) {
		ctx.Error(403)
		return nil
	}
	return org
}

// Create render creating repository page
func Create(ctx *context.Context) {
	if !ctx.User.CanCreateRepo() {
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", ctx.User.MaxCreationLimit()), tplCreate, nil)
	}

	ctx.Data["Title"] = ctx.Tr("new_repo")

	// Give default value for template to render.
	ctx.Data["Gitignores"] = models.Gitignores
	ctx.Data["Licenses"] = models.Licenses
	ctx.Data["Readmes"] = models.Readmes
	ctx.Data["readme"] = "Default"
	ctx.Data["private"] = ctx.User.LastRepoVisibility
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(200, tplCreate)
}

func handleCreateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form interface{}) {
	switch {
	case models.IsErrReachLimitOfRepo(err):
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", owner.MaxCreationLimit()), tpl, form)
	case models.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case models.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		ctx.Handle(500, name, err)
	}
}

// CreatePost response for creating repository
func CreatePost(ctx *context.Context, form auth.CreateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_repo")

	ctx.Data["Gitignores"] = models.Gitignores
	ctx.Data["Licenses"] = models.Licenses
	ctx.Data["Readmes"] = models.Readmes

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplCreate)
		return
	}

	repo, err := models.CreateRepository(ctx.User, ctxUser, models.CreateRepoOptions{
		Name:        form.RepoName,
		Description: form.Description,
		Gitignores:  form.Gitignores,
		License:     form.License,
		Readme:      form.Readme,
		IsPrivate:   form.Private || setting.Repository.ForcePrivate,
		AutoInit:    form.AutoInit,
	})
	if err == nil {
		log.Trace("Repository created [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
		ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
		return
	}

	if repo != nil {
		if errDelete := models.DeleteRepository(ctx.User, ctxUser.ID, repo.ID); errDelete != nil {
			log.Error(4, "DeleteRepository: %v", errDelete)
		}
	}

	handleCreateError(ctx, ctxUser, err, "CreatePost", tplCreate, &form)
}

// Migrate render migration of repository page
func Migrate(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")
	ctx.Data["private"] = ctx.User.LastRepoVisibility
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["mirror"] = ctx.Query("mirror") == "1"
	ctx.Data["LFSActive"] = setting.LFS.StartServer

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(200, tplMigrate)
}

// MigratePost response for migrating from external git repository
func MigratePost(ctx *context.Context, form auth.MigrateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplMigrate)
		return
	}

	remoteAddr, err := form.ParseRemoteAddr(ctx.User)
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			ctx.Data["Err_CloneAddr"] = true
			addrErr := err.(models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsURLError:
				ctx.RenderWithErr(ctx.Tr("form.url_error"), tplMigrate, &form)
			case addrErr.IsPermissionDenied:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tplMigrate, &form)
			case addrErr.IsInvalidPath:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tplMigrate, &form)
			default:
				ctx.Handle(500, "Unknown error", err)
			}
		} else {
			ctx.Handle(500, "ParseRemoteAddr", err)
		}
		return
	}

	repo, err := models.MigrateRepository(ctx.User, ctxUser, models.MigrateRepoOptions{
		Name:        form.RepoName,
		Description: form.Description,
		IsPrivate:   form.Private || setting.Repository.ForcePrivate,
		IsMirror:    form.Mirror,
		RemoteAddr:  remoteAddr,
	})
	if err == nil {
		log.Trace("Repository migrated [%d]: %s/%s", repo.ID, ctxUser.Name, form.RepoName)
		ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + form.RepoName)
		return
	}

	if repo != nil {
		if errDelete := models.DeleteRepository(ctx.User, ctxUser.ID, repo.ID); errDelete != nil {
			log.Error(4, "DeleteRepository: %v", errDelete)
		}
	}

	if strings.Contains(err.Error(), "Authentication failed") ||
		strings.Contains(err.Error(), "could not read Username") {
		ctx.Data["Err_Auth"] = true
		ctx.RenderWithErr(ctx.Tr("form.auth_failed", models.HandleCloneUserCredentials(err.Error(), true)), tplMigrate, &form)
		return
	} else if strings.Contains(err.Error(), "fatal:") {
		ctx.Data["Err_CloneAddr"] = true
		ctx.RenderWithErr(ctx.Tr("repo.migrate.failed", models.HandleCloneUserCredentials(err.Error(), true)), tplMigrate, &form)
		return
	}

	handleCreateError(ctx, ctxUser, err, "MigratePost", tplMigrate, &form)
}

// Action response for actions to a repository
func Action(ctx *context.Context) {
	var err error
	switch ctx.Params(":action") {
	case "watch":
		err = models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	case "unwatch":
		err = models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	case "star":
		err = models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	case "unstar":
		err = models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	case "desc": // FIXME: this is not used
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}

		ctx.Repo.Repository.Description = ctx.Query("desc")
		ctx.Repo.Repository.Website = ctx.Query("site")
		err = models.UpdateRepository(ctx.Repo.Repository, false)
	}

	if err != nil {
		ctx.Handle(500, fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
		return
	}

	redirectTo := ctx.Query("redirect_to")
	if len(redirectTo) == 0 {
		redirectTo = ctx.Repo.RepoLink
	}
	ctx.Redirect(redirectTo)
}

// Download download an archive of a repository
func Download(ctx *context.Context) {
	var (
		uri         = ctx.Params("*")
		refName     string
		ext         string
		archivePath string
		archiveType git.ArchiveType
	)

	switch {
	case strings.HasSuffix(uri, ".zip"):
		ext = ".zip"
		archivePath = path.Join(ctx.Repo.GitRepo.Path, "archives/zip")
		archiveType = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		ext = ".tar.gz"
		archivePath = path.Join(ctx.Repo.GitRepo.Path, "archives/targz")
		archiveType = git.TARGZ
	default:
		log.Trace("Unknown format: %s", uri)
		ctx.Error(404)
		return
	}
	refName = strings.TrimSuffix(uri, ext)

	if !com.IsDir(archivePath) {
		if err := os.MkdirAll(archivePath, os.ModePerm); err != nil {
			ctx.Handle(500, "Download -> os.MkdirAll(archivePath)", err)
			return
		}
	}

	// Get corresponding commit.
	var (
		commit *git.Commit
		err    error
	)
	gitRepo := ctx.Repo.GitRepo
	if gitRepo.IsBranchExist(refName) {
		commit, err = gitRepo.GetBranchCommit(refName)
		if err != nil {
			ctx.Handle(500, "GetBranchCommit", err)
			return
		}
	} else if gitRepo.IsTagExist(refName) {
		commit, err = gitRepo.GetTagCommit(refName)
		if err != nil {
			ctx.Handle(500, "GetTagCommit", err)
			return
		}
	} else if len(refName) >= 4 && len(refName) <= 40 {
		commit, err = gitRepo.GetCommit(refName)
		if err != nil {
			ctx.Handle(404, "GetCommit", nil)
			return
		}
	} else {
		ctx.Handle(404, "Download", nil)
		return
	}

	archivePath = path.Join(archivePath, base.ShortSha(commit.ID.String())+ext)
	if !com.IsFile(archivePath) {
		if err := commit.CreateArchive(archivePath, archiveType); err != nil {
			ctx.Handle(500, "Download -> CreateArchive "+archivePath, err)
			return
		}
	}

	ctx.ServeFile(archivePath, ctx.Repo.Repository.Name+"-"+refName+ext)
}
