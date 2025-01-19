// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"
)

func checkOutdatedBranch(ctx *context.Context) {
	if !(ctx.Repo.IsAdmin() || ctx.Repo.IsOwner()) {
		return
	}

	// get the head commit of the branch since ctx.Repo.CommitID is not always the head commit of `ctx.Repo.BranchName`
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.BranchName)
	if err != nil {
		log.Error("GetBranchCommitID: %v", err)
		// Don't return an error page, as it can be rechecked the next time the user opens the page.
		return
	}

	dbBranch, err := git_model.GetBranch(ctx, ctx.Repo.Repository.ID, ctx.Repo.BranchName)
	if err != nil {
		log.Error("GetBranch: %v", err)
		// Don't return an error page, as it can be rechecked the next time the user opens the page.
		return
	}

	if dbBranch.CommitID != commit.ID.String() {
		ctx.Flash.Warning(ctx.Tr("repo.error.broken_git_hook", "https://docs.gitea.com/help/faq#push-hook--webhook--actions-arent-running"), true)
	}
}

func prepareHomeSidebarRepoTopics(ctx *context.Context) {
	topics, err := db.Find[repo_model.Topic](ctx, &repo_model.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("models.FindTopics", err)
		return
	}
	ctx.Data["Topics"] = topics
}

func prepareOpenWithEditorApps(ctx *context.Context) {
	var tmplApps []map[string]any
	apps := setting.Config().Repository.OpenWithEditorApps.Value(ctx)
	if len(apps) == 0 {
		apps = setting.DefaultOpenWithEditorApps()
	}
	for _, app := range apps {
		schema, _, _ := strings.Cut(app.OpenURL, ":")
		var iconHTML template.HTML
		if schema == "vscode" || schema == "vscodium" || schema == "jetbrains" {
			iconHTML = svg.RenderHTML(fmt.Sprintf("gitea-%s", schema), 16)
		} else {
			iconHTML = svg.RenderHTML("gitea-git", 16) // TODO: it could support user's customized icon in the future
		}
		tmplApps = append(tmplApps, map[string]any{
			"DisplayName": app.DisplayName,
			"OpenURL":     app.OpenURL,
			"IconHTML":    iconHTML,
		})
	}
	ctx.Data["OpenWithEditorApps"] = tmplApps
}

func prepareHomeSidebarCitationFile(entry *git.TreeEntry) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if entry.Name() != "" {
			return
		}
		tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
		if err != nil {
			HandleGitError(ctx, "Repo.Commit.SubTree", err)
			return
		}
		allEntries, err := tree.ListEntries()
		if err != nil {
			ctx.ServerError("ListEntries", err)
			return
		}
		for _, entry := range allEntries {
			if entry.Name() == "CITATION.cff" || entry.Name() == "CITATION.bib" {
				// Read Citation file contents
				if content, err := entry.Blob().GetBlobContent(setting.UI.MaxDisplayFileSize); err != nil {
					log.Error("checkCitationFile: GetBlobContent: %v", err)
				} else {
					ctx.Data["CitiationExist"] = true
					ctx.PageData["citationFileContent"] = content
					break
				}
			}
		}
	}
}

func prepareHomeSidebarLicenses(ctx *context.Context) {
	repoLicenses, err := repo_model.GetRepoLicenses(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoLicenses", err)
		return
	}
	ctx.Data["DetectedRepoLicenses"] = repoLicenses.StringList()
	ctx.Data["LicenseFileName"] = repo_service.LicenseFileName
}

func prepareToRenderDirectory(ctx *context.Context) {
	entries := renderDirectoryFiles(ctx, 1*time.Second)
	if ctx.Written() {
		return
	}

	if ctx.Repo.TreePath != "" {
		ctx.Data["HideRepoInfo"] = true
		ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)
	}

	subfolder, readmeFile, err := findReadmeFileInEntries(ctx, entries, true)
	if err != nil {
		ctx.ServerError("findReadmeFileInEntries", err)
		return
	}

	prepareToRenderReadmeFile(ctx, subfolder, readmeFile)
}

func prepareHomeSidebarLanguageStats(ctx *context.Context) {
	langs, err := repo_model.GetTopLanguageStats(ctx, ctx.Repo.Repository, 5)
	if err != nil {
		ctx.ServerError("Repo.GetTopLanguageStats", err)
		return
	}

	ctx.Data["LanguageStats"] = langs
}

func prepareHomeSidebarLatestRelease(ctx *context.Context) {
	if !ctx.Repo.Repository.UnitEnabled(ctx, unit_model.TypeReleases) {
		return
	}

	release, err := repo_model.GetLatestReleaseByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.ServerError("GetLatestReleaseByRepoID", err)
		return
	}

	if release != nil {
		if err = release.LoadAttributes(ctx); err != nil {
			ctx.ServerError("release.LoadAttributes", err)
			return
		}
		ctx.Data["LatestRelease"] = release
	}
}

func prepareUpstreamDivergingInfo(ctx *context.Context) {
	if !ctx.Repo.Repository.IsFork || !ctx.Repo.IsViewBranch || ctx.Repo.TreePath != "" {
		return
	}
	upstreamDivergingInfo, err := repo_service.GetUpstreamDivergingInfo(ctx, ctx.Repo.Repository, ctx.Repo.BranchName)
	if err != nil {
		if !errors.Is(err, util.ErrNotExist) && !errors.Is(err, util.ErrInvalidArgument) {
			log.Error("GetUpstreamDivergingInfo: %v", err)
		}
		return
	}
	ctx.Data["UpstreamDivergingInfo"] = upstreamDivergingInfo
}

func prepareRecentlyPushedNewBranches(ctx *context.Context) {
	if ctx.Doer != nil {
		if err := ctx.Repo.Repository.GetBaseRepo(ctx); err != nil {
			ctx.ServerError("GetBaseRepo", err)
			return
		}

		opts := &git_model.FindRecentlyPushedNewBranchesOptions{
			Repo:     ctx.Repo.Repository,
			BaseRepo: ctx.Repo.Repository,
		}
		if ctx.Repo.Repository.IsFork {
			opts.BaseRepo = ctx.Repo.Repository.BaseRepo
		}

		baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, opts.BaseRepo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return
		}

		if !opts.Repo.IsMirror && !opts.BaseRepo.IsMirror &&
			opts.BaseRepo.UnitEnabled(ctx, unit_model.TypePullRequests) &&
			baseRepoPerm.CanRead(unit_model.TypePullRequests) {
			var finalBranches []*git_model.RecentlyPushedNewBranch
			branches, err := git_model.FindRecentlyPushedNewBranches(ctx, ctx.Doer, opts)
			if err != nil {
				log.Error("FindRecentlyPushedNewBranches failed: %v", err)
			}

			for _, branch := range branches {
				divergingInfo, err := repo_service.GetBranchDivergingInfo(ctx,
					branch.BranchRepo, branch.BranchName, // "base" repo for diverging info
					opts.BaseRepo, opts.BaseRepo.DefaultBranch, // "head" repo for diverging info
				)
				if err != nil {
					log.Error("GetBranchDivergingInfo failed: %v", err)
					continue
				}
				branchRepoHasNewCommits := divergingInfo.BaseHasNewCommits
				baseRepoCommitsBehind := divergingInfo.HeadCommitsBehind
				if branchRepoHasNewCommits || baseRepoCommitsBehind > 0 {
					finalBranches = append(finalBranches, branch)
				}
			}
			ctx.Data["RecentlyPushedNewBranches"] = finalBranches
		}
	}
}

func updateContextRepoEmptyAndStatus(ctx *context.Context, empty bool, status repo_model.RepositoryStatus) {
	if ctx.Repo.Repository.IsEmpty == empty && ctx.Repo.Repository.Status == status {
		return
	}
	ctx.Repo.Repository.IsEmpty = empty
	if ctx.Repo.Repository.Status == repo_model.RepositoryReady || ctx.Repo.Repository.Status == repo_model.RepositoryBroken {
		ctx.Repo.Repository.Status = status // only handle ready and broken status, leave other status as-is
	}
	if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, ctx.Repo.Repository, "is_empty", "status"); err != nil {
		ctx.ServerError("updateContextRepoEmptyAndStatus: UpdateRepositoryCols", err)
		return
	}
}

func handleRepoEmptyOrBroken(ctx *context.Context) {
	showEmpty := true
	if ctx.Repo.GitRepo != nil {
		reallyEmpty, err := ctx.Repo.GitRepo.IsEmpty()
		if err != nil {
			showEmpty = true // the repo is broken
			updateContextRepoEmptyAndStatus(ctx, true, repo_model.RepositoryBroken)
			log.Error("GitRepo.IsEmpty: %v", err)
			ctx.Flash.Error(ctx.Tr("error.occurred"), true)
		} else if reallyEmpty {
			showEmpty = true // the repo is really empty
			updateContextRepoEmptyAndStatus(ctx, true, repo_model.RepositoryReady)
		} else if branches, _, _ := ctx.Repo.GitRepo.GetBranches(0, 1); len(branches) == 0 {
			showEmpty = true // it is not really empty, but there is no branch
			// at the moment, other repo units like "actions" are not able to handle such case,
			// so we just mark the repo as empty to prevent from displaying these units.
			ctx.Data["RepoHasContentsWithoutBranch"] = true
			updateContextRepoEmptyAndStatus(ctx, true, repo_model.RepositoryReady)
		} else {
			// the repo is actually not empty and has branches, need to update the database later
			showEmpty = false
		}
	}
	if showEmpty {
		ctx.HTML(http.StatusOK, tplRepoEMPTY)
		return
	}

	// The repo is not really empty, so we should update the model in database, such problem may be caused by:
	// 1) an error occurs during pushing/receiving.
	// 2) the user replaces an empty git repo manually.
	updateContextRepoEmptyAndStatus(ctx, false, repo_model.RepositoryReady)
	if err := repo_module.UpdateRepoSize(ctx, ctx.Repo.Repository); err != nil {
		ctx.ServerError("UpdateRepoSize", err)
		return
	}

	// the repo's IsEmpty has been updated, redirect to this page to make sure middlewares can get the correct values
	link := ctx.Link
	if ctx.Req.URL.RawQuery != "" {
		link += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(link)
}

func prepareToRenderDirOrFile(entry *git.TreeEntry) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if entry.IsDir() {
			prepareToRenderDirectory(ctx)
		} else {
			prepareToRenderFile(ctx, entry)
		}
	}
}

func handleRepoHomeFeed(ctx *context.Context) bool {
	if setting.Other.EnableFeed {
		isFeed, _, showFeedType := feed.GetFeedType(ctx.PathParam(":reponame"), ctx.Req)
		if isFeed {
			switch {
			case ctx.Link == fmt.Sprintf("%s.%s", ctx.Repo.RepoLink, showFeedType):
				feed.ShowRepoFeed(ctx, ctx.Repo.Repository, showFeedType)
			case ctx.Repo.TreePath == "":
				feed.ShowBranchFeed(ctx, ctx.Repo.Repository, showFeedType)
			case ctx.Repo.TreePath != "":
				feed.ShowFileFeed(ctx, ctx.Repo.Repository, showFeedType)
			}
			return true
		}
	}
	return false
}

// Home render repository home page
func Home(ctx *context.Context) {
	if handleRepoHomeFeed(ctx) {
		return
	}

	// Check whether the repo is viewable: not in migration, and the code unit should be enabled
	// Ideally the "feed" logic should be after this, but old code did so, so keep it as-is.
	checkHomeCodeViewable(ctx)
	if ctx.Written() {
		return
	}

	title := ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	if len(ctx.Repo.Repository.Description) > 0 {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["RepositoryUploadEnabled"] = setting.Repository.Upload.Enabled // show New File / Upload File buttons

	if ctx.Repo.Commit == nil || ctx.Repo.Repository.IsEmpty || ctx.Repo.Repository.IsBroken() {
		// empty or broken repositories need to be handled differently
		handleRepoEmptyOrBroken(ctx)
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

	ctx.HTML(http.StatusOK, tplRepoHome)
}
