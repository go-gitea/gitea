package repo

import (
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	tplRepoViewHome  templates.TplName = "repo/view/home"
	tplRepoViewEmpty templates.TplName = "repo/view/empty"
)

// View render repository view page
func View(ctx *context.Context) {
	// Check whether the repo is viewable: not in migration, and the code unit should be enabled
	checkHomeCodeViewable(ctx)
	if ctx.Written() {
		return
	}

	title := ctx.Repo.Repository.Owner.DisplayName() + " " + ctx.Repo.Repository.Name
	if len(ctx.Repo.Repository.Description) > 0 {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title
	// ctx.Data["PageIsViewCode"] = false
	// ctx.Data["RepositoryUploadEnabled"] = false

	if ctx.Repo.Commit == nil || ctx.Repo.Repository.IsEmpty || ctx.Repo.Repository.IsBroken() {
		// empty or broken repositories need to be handled differently
		handleRepoEmptyOrBrokenView(ctx)
		return
	}

	// get the current git entry which doer user is currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "Repo.Commit.GetTreeEntryByPath", err)
		return
	}

	// prepare the tree path
	var treeNames, paths []string
	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink
	if ctx.Repo.TreePath != "" {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
		treeNames = strings.Split(ctx.Repo.TreePath, "/")
		for i := range treeNames {
			paths = append(paths, strings.Join(treeNames[:i+1], "/"))
		}
		ctx.Data["HasParentPath"] = true
		if len(paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + paths[len(paths)-2]
		}
	}
	ctx.Data["Paths"] = paths
	ctx.Data["TreeLink"] = treeLink
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["BranchLink"] = branchLink

	// some UI components are only shown when the tree path is root
	isTreePathRoot := ctx.Repo.TreePath == ""

	prepareFuncs := []func(*context.Context){
		prepareOpenWithEditorApps,
		prepareHomeSidebarRepoTopics,
		checkOutdatedBranch,
		prepareToRenderDirOrFile(entry),
		prepareRecentlyPushedNewBranches,
	}

	if isTreePathRoot {
		prepareFuncs = append(prepareFuncs,
			prepareUpstreamDivergingInfo,
			prepareHomeSidebarLicenses,
			prepareHomeSidebarCitationFile(entry),
			prepareHomeSidebarLanguageStats,
			prepareHomeSidebarLatestRelease,
		)
	}

	for _, prepare := range prepareFuncs {
		prepare(ctx)
		if ctx.Written() {
			return
		}
	}

	ctx.HTML(http.StatusOK, tplRepoViewHome)
}

func handleRepoEmptyOrBrokenView(ctx *context.Context) {
	showEmpty := true
	var err error
	if ctx.Repo.GitRepo != nil {
		showEmpty, err = ctx.Repo.GitRepo.IsEmpty()
		if err != nil {
			log.Error("GitRepo.IsEmpty: %v", err)
			ctx.Repo.Repository.Status = repo_model.RepositoryBroken
			showEmpty = true
			ctx.Flash.Error(ctx.Tr("error.occurred"), true)
		}
	}
	if showEmpty {
		ctx.HTML(http.StatusOK, tplRepoViewEmpty)
		return
	}

	// the repo is not really empty, so we should update the modal in database
	// such problem may be caused by:
	// 1) an error occurs during pushing/receiving.  2) the user replaces an empty git repo manually
	// and even more: the IsEmpty flag is deeply broken and should be removed with the UI changed to manage to cope with empty repos.
	// it's possible for a repository to be non-empty by that flag but still 500
	// because there are no branches - only tags -or the default branch is non-extant as it has been 0-pushed.
	ctx.Repo.Repository.IsEmpty = false
	if err = repo_model.UpdateRepositoryCols(ctx, ctx.Repo.Repository, "is_empty"); err != nil {
		ctx.ServerError("UpdateRepositoryCols", err)
		return
	}
	if err = repo_module.UpdateRepoSize(ctx, ctx.Repo.Repository); err != nil {
		ctx.ServerError("UpdateRepoSize", err)
		return
	}

	// TODO: do I need this?
	// the repo's IsEmpty has been updated, redirect to this page to make sure middlewares can get the correct values
	link := ctx.Link
	if ctx.Req.URL.RawQuery != "" {
		link += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(link)
}
