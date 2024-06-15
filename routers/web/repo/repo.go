// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
	commitstatus_service "code.gitea.io/gitea/services/repository/commitstatus"
)

const (
	tplCreate       base.TplName = "repo/create"
	tplAlertDetails base.TplName = "base/alert_details"
)

// MustBeNotEmpty render when a repo is a empty git dir
func MustBeNotEmpty(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound("MustBeNotEmpty", nil)
	}
}

// MustBeEditable check that repo can be edited
func MustBeEditable(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
		ctx.NotFound("", nil)
		return
	}
}

// MustBeAbleToUpload check that repo can be uploaded to
func MustBeAbleToUpload(ctx *context.Context) {
	if !setting.Repository.Upload.Enabled {
		ctx.NotFound("", nil)
	}
}

func CommitInfoCache(ctx *context.Context) {
	var err error
	ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}
	ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}
	ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
	ctx.Repo.GitRepo.LastCommitCache = git.NewLastCommitCache(ctx.Repo.CommitsCount, ctx.Repo.Repository.FullName(), ctx.Repo.GitRepo, cache.GetCache())
}

func checkContextUser(ctx *context.Context, uid int64) *user_model.User {
	orgs, err := organization.GetOrgsCanCreateRepoByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return nil
	}

	if !ctx.Doer.IsAdmin {
		orgsAvailable := []*organization.Organization{}
		for i := 0; i < len(orgs); i++ {
			if orgs[i].CanCreateRepo() {
				orgsAvailable = append(orgsAvailable, orgs[i])
			}
		}
		ctx.Data["Orgs"] = orgsAvailable
	} else {
		ctx.Data["Orgs"] = orgs
	}

	// Not equal means current user is an organization.
	if uid == ctx.Doer.ID || uid == 0 {
		return ctx.Doer
	}

	org, err := user_model.GetUserByID(ctx, uid)
	if user_model.IsErrUserNotExist(err) {
		return ctx.Doer
	}

	if err != nil {
		ctx.ServerError("GetUserByID", fmt.Errorf("[%d]: %w", uid, err))
		return nil
	}

	// Check ownership of organization.
	if !org.IsOrganization() {
		ctx.Error(http.StatusForbidden)
		return nil
	}
	if !ctx.Doer.IsAdmin {
		canCreate, err := organization.OrgFromUser(org).CanCreateOrgRepo(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return nil
		} else if !canCreate {
			ctx.Error(http.StatusForbidden)
			return nil
		}
	} else {
		ctx.Data["Orgs"] = orgs
	}
	return org
}

func getRepoPrivate(ctx *context.Context) bool {
	switch strings.ToLower(setting.Repository.DefaultPrivate) {
	case setting.RepoCreatingLastUserVisibility:
		return ctx.Doer.LastRepoVisibility
	case setting.RepoCreatingPrivate:
		return true
	case setting.RepoCreatingPublic:
		return false
	default:
		return ctx.Doer.LastRepoVisibility
	}
}

// Create render creating repository page
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_repo")

	// Give default value for template to render.
	ctx.Data["Gitignores"] = repo_module.Gitignores
	ctx.Data["LabelTemplateFiles"] = repo_module.LabelTemplateFiles
	ctx.Data["Licenses"] = repo_module.Licenses
	ctx.Data["Readmes"] = repo_module.Readmes
	ctx.Data["readme"] = "Default"
	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["default_branch"] = setting.Repository.DefaultBranch

	ctxUser := checkContextUser(ctx, ctx.FormInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.Data["repo_template_name"] = ctx.Tr("repo.template_select")
	templateID := ctx.FormInt64("template_id")
	if templateID > 0 {
		templateRepo, err := repo_model.GetRepositoryByID(ctx, templateID)
		if err == nil && access_model.CheckRepoUnitUser(ctx, templateRepo, ctxUser, unit.TypeCode) {
			ctx.Data["repo_template"] = templateID
			ctx.Data["repo_template_name"] = templateRepo.Name
		}
	}

	ctx.Data["CanCreateRepo"] = ctx.Doer.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.Doer.MaxCreationLimit()
	ctx.Data["SupportedObjectFormats"] = git.DefaultFeatures().SupportedObjectFormats
	ctx.Data["DefaultObjectFormat"] = git.Sha1ObjectFormat

	ctx.HTML(http.StatusOK, tplCreate)
}

func handleCreateError(ctx *context.Context, owner *user_model.User, err error, name string, tpl base.TplName, form any) {
	switch {
	case repo_model.IsErrReachLimitOfRepo(err):
		maxCreationLimit := owner.MaxCreationLimit()
		msg := ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit)
		ctx.RenderWithErr(msg, tpl, form)
	case repo_model.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case repo_model.IsErrRepoFilesAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		switch {
		case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"), tpl, form)
		case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt"), tpl, form)
		case setting.Repository.AllowDeleteOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.delete"), tpl, form)
		default:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist"), tpl, form)
		}
	case db.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tpl, form)
	case db.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		ctx.ServerError(name, err)
	}
}

// CreatePost response for creating repository
func CreatePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateRepoForm)
	ctx.Data["Title"] = ctx.Tr("new_repo")

	ctx.Data["Gitignores"] = repo_module.Gitignores
	ctx.Data["LabelTemplateFiles"] = repo_module.LabelTemplateFiles
	ctx.Data["Licenses"] = repo_module.Licenses
	ctx.Data["Readmes"] = repo_module.Readmes

	ctx.Data["CanCreateRepo"] = ctx.Doer.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.Doer.MaxCreationLimit()

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplCreate)
		return
	}

	var repo *repo_model.Repository
	var err error
	if form.RepoTemplate > 0 {
		opts := repo_service.GenerateRepoOptions{
			Name:            form.RepoName,
			Description:     form.Description,
			Private:         form.Private || setting.Repository.ForcePrivate,
			GitContent:      form.GitContent,
			Topics:          form.Topics,
			GitHooks:        form.GitHooks,
			Webhooks:        form.Webhooks,
			Avatar:          form.Avatar,
			IssueLabels:     form.Labels,
			ProtectedBranch: form.ProtectedBranch,
		}

		if !opts.IsValid() {
			ctx.RenderWithErr(ctx.Tr("repo.template.one_item"), tplCreate, form)
			return
		}

		templateRepo := getRepository(ctx, form.RepoTemplate)
		if ctx.Written() {
			return
		}

		if !templateRepo.IsTemplate {
			ctx.RenderWithErr(ctx.Tr("repo.template.invalid"), tplCreate, form)
			return
		}

		repo, err = repo_service.GenerateRepository(ctx, ctx.Doer, ctxUser, templateRepo, opts)
		if err == nil {
			log.Trace("Repository generated [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(repo.Link())
			return
		}
	} else {
		repo, err = repo_service.CreateRepository(ctx, ctx.Doer, ctxUser, repo_service.CreateRepoOptions{
			Name:             form.RepoName,
			Description:      form.Description,
			Gitignores:       form.Gitignores,
			IssueLabels:      form.IssueLabels,
			License:          form.License,
			Readme:           form.Readme,
			IsPrivate:        form.Private || setting.Repository.ForcePrivate,
			DefaultBranch:    form.DefaultBranch,
			AutoInit:         form.AutoInit,
			IsTemplate:       form.Template,
			TrustModel:       repo_model.DefaultTrustModel,
			ObjectFormatName: form.ObjectFormatName,
		})
		if err == nil {
			log.Trace("Repository created [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(repo.Link())
			return
		}
	}

	handleCreateError(ctx, ctxUser, err, "CreatePost", tplCreate, &form)
}

const (
	tplWatchUnwatch base.TplName = "repo/watch_unwatch"
	tplStarUnstar   base.TplName = "repo/star_unstar"
)

// Action response for actions to a repository
func Action(ctx *context.Context) {
	var err error
	switch ctx.Params(":action") {
	case "watch":
		err = repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, true)
	case "unwatch":
		err = repo_model.WatchRepo(ctx, ctx.Doer, ctx.Repo.Repository, false)
	case "star":
		err = repo_model.StarRepo(ctx, ctx.Doer, ctx.Repo.Repository, true)
	case "unstar":
		err = repo_model.StarRepo(ctx, ctx.Doer, ctx.Repo.Repository, false)
	case "accept_transfer":
		err = acceptOrRejectRepoTransfer(ctx, true)
	case "reject_transfer":
		err = acceptOrRejectRepoTransfer(ctx, false)
	case "desc": // FIXME: this is not used
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}

		ctx.Repo.Repository.Description = ctx.FormString("desc")
		ctx.Repo.Repository.Website = ctx.FormString("site")
		err = repo_service.UpdateRepository(ctx, ctx.Repo.Repository, false)
	}

	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.Flash.Error(ctx.Tr("repo.action.blocked_user"))
		} else {
			ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
			return
		}
	}

	switch ctx.Params(":action") {
	case "watch", "unwatch":
		ctx.Data["IsWatchingRepo"] = repo_model.IsWatching(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID)
	case "star", "unstar":
		ctx.Data["IsStaringRepo"] = repo_model.IsStaring(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID)
	}

	switch ctx.Params(":action") {
	case "watch", "unwatch", "star", "unstar":
		// we have to reload the repository because NumStars or NumWatching (used in the templates) has just changed
		ctx.Data["Repository"], err = repo_model.GetRepositoryByName(ctx, ctx.Repo.Repository.OwnerID, ctx.Repo.Repository.Name)
		if err != nil {
			ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
			return
		}
	}

	switch ctx.Params(":action") {
	case "watch", "unwatch":
		ctx.HTML(http.StatusOK, tplWatchUnwatch)
		return
	case "star", "unstar":
		ctx.HTML(http.StatusOK, tplStarUnstar)
		return
	}

	ctx.RedirectToCurrentSite(ctx.FormString("redirect_to"), ctx.Repo.RepoLink)
}

func acceptOrRejectRepoTransfer(ctx *context.Context, accept bool) error {
	repoTransfer, err := models.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
	if err != nil {
		return err
	}

	if err := repoTransfer.LoadAttributes(ctx); err != nil {
		return err
	}

	if !repoTransfer.CanUserAcceptTransfer(ctx, ctx.Doer) {
		return errors.New("user does not have enough permissions")
	}

	if accept {
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
			ctx.Repo.GitRepo = nil
		}

		if err := repo_service.TransferOwnership(ctx, repoTransfer.Doer, repoTransfer.Recipient, ctx.Repo.Repository, repoTransfer.Teams); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.success"))
	} else {
		if err := repo_service.CancelRepositoryTransfer(ctx, ctx.Repo.Repository); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.rejected"))
	}

	ctx.Redirect(ctx.Repo.Repository.Link())
	return nil
}

// RedirectDownload return a file based on the following infos:
func RedirectDownload(ctx *context.Context) {
	var (
		vTag     = ctx.Params("vTag")
		fileName = ctx.Params("fileName")
	)
	tagNames := []string{vTag}
	curRepo := ctx.Repo.Repository
	releases, err := db.Find[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeDrafts: ctx.Repo.CanWrite(unit.TypeReleases),
		RepoID:        curRepo.ID,
		TagNames:      tagNames,
	})
	if err != nil {
		ctx.ServerError("RedirectDownload", err)
		return
	}
	if len(releases) == 1 {
		release := releases[0]
		att, err := repo_model.GetAttachmentByReleaseIDFileName(ctx, release.ID, fileName)
		if err != nil {
			ctx.Error(http.StatusNotFound)
			return
		}
		if att != nil {
			ServeAttachment(ctx, att.UUID)
			return
		}
	} else if len(releases) == 0 && vTag == "latest" {
		// GitHub supports the alias "latest" for the latest release
		// We only fetch the latest release if the tag is "latest" and no release with the tag "latest" exists
		release, err := repo_model.GetLatestReleaseByRepoID(ctx, ctx.Repo.Repository.ID)
		if err != nil {
			ctx.Error(http.StatusNotFound)
			return
		}
		att, err := repo_model.GetAttachmentByReleaseIDFileName(ctx, release.ID, fileName)
		if err != nil {
			ctx.Error(http.StatusNotFound)
			return
		}
		if att != nil {
			ServeAttachment(ctx, att.UUID)
			return
		}
	}
	ctx.Error(http.StatusNotFound)
}

// Download an archive of a repository
func Download(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		if errors.Is(err, archiver_service.ErrUnknownArchiveFormat{}) {
			ctx.Error(http.StatusBadRequest, err.Error())
		} else if errors.Is(err, archiver_service.RepoRefNotFoundError{}) {
			ctx.Error(http.StatusNotFound, err.Error())
		} else {
			ctx.ServerError("archiver_service.NewRequest", err)
		}
		return
	}

	archiver, err := aReq.Await(ctx)
	if err != nil {
		ctx.ServerError("archiver.Await", err)
		return
	}

	download(ctx, aReq.GetArchiveName(), archiver)
}

func download(ctx *context.Context, archiveName string, archiver *repo_model.RepoArchiver) {
	downloadName := ctx.Repo.Repository.Name + "-" + archiveName

	// Add nix format link header so tarballs lock correctly:
	// https://github.com/nixos/nix/blob/56763ff918eb308db23080e560ed2ea3e00c80a7/doc/manual/src/protocols/tarball-fetcher.md
	ctx.Resp.Header().Add("Link", fmt.Sprintf(`<%s/archive/%s.tar.gz?rev=%s>; rel="immutable"`,
		ctx.Repo.Repository.APIURL(),
		archiver.CommitID, archiver.CommitID))

	rPath := archiver.RelativePath()
	if setting.RepoArchive.Storage.ServeDirect() {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.RepoArchives.URL(rPath, downloadName)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	// If we have matched and access to release or issue
	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	ctx.ServeContent(fr, &context.ServeHeaderOptions{
		Filename:     downloadName,
		LastModified: archiver.CreatedUnix.AsLocalTime(),
	})
}

// InitiateDownload will enqueue an archival request, as needed.  It may submit
// a request that's already in-progress, but the archiver service will just
// kind of drop it on the floor if this is the case.
func InitiateDownload(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		ctx.ServerError("archiver_service.NewRequest", err)
		return
	}
	if aReq == nil {
		ctx.Error(http.StatusNotFound)
		return
	}

	archiver, err := repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		ctx.ServerError("archiver_service.StartArchive", err)
		return
	}
	if archiver == nil || archiver.Status != repo_model.ArchiverReady {
		if err := archiver_service.StartArchive(aReq); err != nil {
			ctx.ServerError("archiver_service.StartArchive", err)
			return
		}
	}

	var completed bool
	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		completed = true
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"complete": completed,
	})
}

// SearchRepo repositories via options
func SearchRepo(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}
	opts := &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		Actor:              ctx.Doer,
		Keyword:            ctx.FormTrim("q"),
		OwnerID:            ctx.FormInt64("uid"),
		PriorityOwnerID:    ctx.FormInt64("priority_owner_id"),
		TeamID:             ctx.FormInt64("team_id"),
		TopicOnly:          ctx.FormBool("topic"),
		Collaborate:        optional.None[bool](),
		Private:            ctx.IsSigned && (ctx.FormString("private") == "" || ctx.FormBool("private")),
		Template:           optional.None[bool](),
		StarredByID:        ctx.FormInt64("starredBy"),
		IncludeDescription: ctx.FormBool("includeDesc"),
	}

	if ctx.FormString("template") != "" {
		opts.Template = optional.Some(ctx.FormBool("template"))
	}

	if ctx.FormBool("exclusive") {
		opts.Collaborate = optional.Some(false)
	}

	mode := ctx.FormString("mode")
	switch mode {
	case "source":
		opts.Fork = optional.Some(false)
		opts.Mirror = optional.Some(false)
	case "fork":
		opts.Fork = optional.Some(true)
	case "mirror":
		opts.Mirror = optional.Some(true)
	case "collaborative":
		opts.Mirror = optional.Some(false)
		opts.Collaborate = optional.Some(true)
	case "":
	default:
		ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid search mode: \"%s\"", mode))
		return
	}

	if ctx.FormString("archived") != "" {
		opts.Archived = optional.Some(ctx.FormBool("archived"))
	}

	if ctx.FormString("is_private") != "" {
		opts.IsPrivate = optional.Some(ctx.FormBool("is_private"))
	}

	sortMode := ctx.FormString("sort")
	if len(sortMode) > 0 {
		sortOrder := ctx.FormString("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := repo_model.OrderByMap[sortOrder]; ok {
			if orderBy, ok := searchModeMap[sortMode]; ok {
				opts.OrderBy = orderBy
			} else {
				ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid sort mode: \"%s\"", sortMode))
				return
			}
		} else {
			ctx.Error(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid sort order: \"%s\"", sortOrder))
			return
		}
	}

	// To improve performance when only the count is requested
	if ctx.FormBool("count_only") {
		if count, err := repo_model.CountRepository(ctx, opts); err != nil {
			log.Error("CountRepository: %v", err)
			ctx.JSON(http.StatusInternalServerError, nil) // frontend JS doesn't handle error response (same as below)
		} else {
			ctx.SetTotalCountHeader(count)
			ctx.JSONOK()
		}
		return
	}

	repos, count, err := repo_model.SearchRepository(ctx, opts)
	if err != nil {
		log.Error("SearchRepository: %v", err)
		ctx.JSON(http.StatusInternalServerError, nil)
		return
	}

	ctx.SetTotalCountHeader(count)

	latestCommitStatuses, err := commitstatus_service.FindReposLastestCommitStatuses(ctx, repos)
	if err != nil {
		log.Error("FindReposLastestCommitStatuses: %v", err)
		ctx.JSON(http.StatusInternalServerError, nil)
		return
	}

	results := make([]*repo_service.WebSearchRepository, len(repos))
	for i, repo := range repos {
		results[i] = &repo_service.WebSearchRepository{
			Repository: &api.Repository{
				ID:       repo.ID,
				FullName: repo.FullName(),
				Fork:     repo.IsFork,
				Private:  repo.IsPrivate,
				Template: repo.IsTemplate,
				Mirror:   repo.IsMirror,
				Stars:    repo.NumStars,
				HTMLURL:  repo.HTMLURL(),
				Link:     repo.Link(),
				Internal: !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
			},
		}

		if latestCommitStatuses[i] != nil {
			results[i].LatestCommitStatus = latestCommitStatuses[i]
			results[i].LocaleLatestCommitStatus = latestCommitStatuses[i].LocaleString(ctx.Locale)
		}
	}

	ctx.JSON(http.StatusOK, repo_service.WebSearchResults{
		OK:   true,
		Data: results,
	})
}

type branchTagSearchResponse struct {
	Results []string `json:"results"`
}

// GetBranchesList get branches for current repo'
func GetBranchesList(ctx *context.Context) {
	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
		ListOptions:     db.ListOptionsAll,
	}
	branches, err := git_model.FindBranchNames(ctx, branchOpts)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}
	resp := &branchTagSearchResponse{}
	// always put default branch on the top if it exists
	if slices.Contains(branches, ctx.Repo.Repository.DefaultBranch) {
		branches = util.SliceRemoveAll(branches, ctx.Repo.Repository.DefaultBranch)
		branches = append([]string{ctx.Repo.Repository.DefaultBranch}, branches...)
	}
	resp.Results = branches
	ctx.JSON(http.StatusOK, resp)
}

// GetTagList get tag list for current repo
func GetTagList(ctx *context.Context) {
	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}
	resp := &branchTagSearchResponse{}
	resp.Results = tags
	ctx.JSON(http.StatusOK, resp)
}

func PrepareBranchList(ctx *context.Context) {
	branchOpts := git_model.FindBranchOptions{
		RepoID:          ctx.Repo.Repository.ID,
		IsDeletedBranch: optional.Some(false),
		ListOptions:     db.ListOptionsAll,
	}
	brs, err := git_model.FindBranchNames(ctx, branchOpts)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	// always put default branch on the top if it exists
	if slices.Contains(brs, ctx.Repo.Repository.DefaultBranch) {
		brs = util.SliceRemoveAll(brs, ctx.Repo.Repository.DefaultBranch)
		brs = append([]string{ctx.Repo.Repository.DefaultBranch}, brs...)
	}
	ctx.Data["Branches"] = brs
}
