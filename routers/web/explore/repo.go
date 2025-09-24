// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sitemap"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/services/context"
)

const (
	// tplExploreRepos explore repositories page template
	tplExploreRepos        templates.TplName = "explore/repos"
	relevantReposOnlyParam string            = "only_show_relevant"
)

// RepoSearchOptions when calling search repositories
type RepoSearchOptions struct {
	OwnerID          int64
	Private          bool
	Restricted       bool
	PageSize         int
	OnlyShowRelevant bool
	TplName          templates.TplName
}

// RenderRepoSearch render repositories search page
// This function is also used to render the Admin Repository Management page.
func RenderRepoSearch(ctx *context.Context, opts *RepoSearchOptions) {
	// Sitemap index for sitemap paths
	page := int(ctx.PathParamInt64("idx"))
	isSitemap := ctx.PathParam("idx") != ""
	if page <= 1 {
		page = ctx.FormInt("page")
	}

	if page <= 0 {
		page = 1
	}

	if isSitemap {
		opts.PageSize = setting.UI.SitemapPagingNum
	}

	var (
		repos   []*repo_model.Repository
		count   int64
		err     error
		orderBy db.SearchOrderBy
	)

	sortOrder := ctx.FormString("sort")
	if sortOrder == "" {
		sortOrder = setting.UI.ExploreDefaultSort
	}

	if order, ok := repo_model.OrderByFlatMap[sortOrder]; ok {
		orderBy = order
	} else {
		sortOrder = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}
	ctx.Data["SortType"] = sortOrder

	keyword := ctx.FormTrim("q")

	ctx.Data["OnlyShowRelevant"] = opts.OnlyShowRelevant

	topicOnly := ctx.FormBool("topic")
	ctx.Data["TopicOnly"] = topicOnly

	language := ctx.FormTrim("language")
	ctx.Data["Language"] = language

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	repos, count, err = repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: opts.PageSize,
		},
		Actor:              ctx.Doer,
		OrderBy:            orderBy,
		Private:            opts.Private,
		Keyword:            keyword,
		OwnerID:            opts.OwnerID,
		AllPublic:          true,
		AllLimited:         true,
		TopicOnly:          topicOnly,
		Language:           language,
		IncludeDescription: setting.UI.SearchRepoDescription,
		OnlyShowRelevant:   opts.OnlyShowRelevant,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	if isSitemap {
		m := sitemap.NewSitemap()
		for _, item := range repos {
			m.Add(sitemap.URL{URL: item.HTMLURL(), LastMod: item.UpdatedUnix.AsTimePtr()})
		}
		ctx.Resp.Header().Set("Content-Type", "text/xml")
		if _, err := m.WriteTo(ctx.Resp); err != nil {
			log.Error("Failed writing sitemap: %v", err)
		}
		return
	}

	ctx.Data["Keyword"] = keyword
	ctx.Data["Total"] = count
	ctx.Data["Repos"] = repos
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	pager := context.NewPagination(int(count), opts.PageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, opts.TplName)
}

// Repos render explore repositories page
func Repos(ctx *context.Context) {
	ctx.Data["UsersPageIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["OrganizationsPageIsDisabled"] = setting.Service.Explore.DisableOrganizationsPage
	ctx.Data["CodePageIsDisabled"] = setting.Service.Explore.DisableCodePage
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["ShowRepoOwnerOnList"] = true
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	var ownerID int64
	if ctx.Doer != nil && !ctx.Doer.IsAdmin {
		ownerID = ctx.Doer.ID
	}

	onlyShowRelevant := setting.UI.OnlyShowRelevantRepos

	_ = ctx.Req.ParseForm() // parse the form first, to prepare the ctx.Req.Form field
	if len(ctx.Req.Form[relevantReposOnlyParam]) != 0 {
		onlyShowRelevant = ctx.FormBool(relevantReposOnlyParam)
	}

	RenderRepoSearch(ctx, &RepoSearchOptions{
		PageSize:         setting.UI.ExplorePagingNum,
		OwnerID:          ownerID,
		Private:          ctx.Doer != nil,
		TplName:          tplExploreRepos,
		OnlyShowRelevant: onlyShowRelevant,
	})
}

// RepoHistory renders repository history page - an alternative interface to repo home
func RepoHistory(ctx *context.Context) {
	// Set page metadata
	ctx.Data["Title"] = ctx.Repo.Repository.FullName() + " - History View"
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["PageIsRepoHistory"] = true
	ctx.Data["IsRepoHistoryView"] = true

	// Call the main repository home logic
	// This duplicates the functionality of repo.Home but in the explore context
	renderRepositoryHistory(ctx)
}

// renderRepositoryHistory duplicates repo.Home functionality for the history view
func renderRepositoryHistory(ctx *context.Context) {
	// Handle feed requests
	if handleRepoHistoryFeed(ctx) {
		return
	}

	// Check repository viewability
	if !ctx.Repo.Repository.UnitEnabled(ctx, unit.TypeCode) {
		ctx.NotFound(errors.New("code unit disabled for repository"))
		return
	}

	// Set up basic repository data
	title := ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name + " (History)"
	if ctx.Repo.Repository.Description != "" {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["RepositoryUploadEnabled"] = false // Disable uploads in history view

	// Handle empty or broken repositories
	if ctx.Repo.Repository.IsEmpty || ctx.Repo.Repository.IsBroken() {
		ctx.Data["IsRepoEmpty"] = true
		ctx.HTML(http.StatusOK, "repo/empty")
		return
	}

	// Initialize git repository
	gitRepo, err := gitrepo.OpenRepository(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer gitRepo.Close()

	// Get default branch
	defaultBranch := ctx.Repo.Repository.DefaultBranch

	// Get commit for default branch
	commit, err := gitRepo.GetBranchCommit(defaultBranch)
	if err != nil {
		ctx.ServerError("GetBranchCommit", err)
		return
	}

	// Set up repository context
	ctx.Repo.GitRepo = gitRepo
	ctx.Repo.BranchName = defaultBranch
	ctx.Repo.Commit = commit
	ctx.Repo.CommitID = commit.ID.String()
	ctx.Repo.TreePath = ""

	// Get repository tree entries
	entries, err := commit.ListEntries()
	if err != nil {
		ctx.ServerError("Commit.ListEntries", err)
		return
	}

	// Set up template data
	ctx.Data["BranchName"] = defaultBranch
	ctx.Data["CommitID"] = commit.ID.String()
	ctx.Data["TreePath"] = ""
	ctx.Data["Files"] = entries
	ctx.Data["LastCommit"] = commit
	ctx.Data["LastCommitUser"] = commit.Committer

	// Repository metadata
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()
	ctx.Data["CloneButtonOriginLink"] = ctx.Repo.Repository.CloneLink(ctx, ctx.Doer)

	// Render the history view template
	ctx.HTML(http.StatusOK, "explore/repo_history")
}

// handleRepoHistoryFeed handles RSS/Atom feed requests for repository history
func handleRepoHistoryFeed(ctx *context.Context) bool {
	if !setting.Other.EnableFeed {
		return false
	}

	// Check if this is a feed request
	repoName := ctx.PathParam("reponame")
	if strings.HasSuffix(repoName, ".rss") || strings.HasSuffix(repoName, ".atom") {
		// Handle feed logic here if needed
		return true
	}
	return false
}
