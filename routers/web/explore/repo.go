// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package explore

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sitemap"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

const (
	// tplExploreRepos explore repositories page template
	tplExploreRepos templates.TplName = "explore/repos"
	// tplExploreSubjects explore subjects page template
	tplExploreSubjects     templates.TplName = "explore/subjects"
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

// Subjects render explore subjects page (articles list)
func Subjects(ctx *context.Context) {
	ctx.Data["UsersPageIsDisabled"] = setting.Service.Explore.DisableUsersPage
	ctx.Data["OrganizationsPageIsDisabled"] = setting.Service.Explore.DisableOrganizationsPage
	ctx.Data["CodePageIsDisabled"] = setting.Service.Explore.DisableCodePage
	ctx.Data["Title"] = ctx.Tr("explore")
	ctx.Data["PageIsExplore"] = true
	ctx.Data["PageIsExploreSubjects"] = true

	// Get page number
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	// Get sort order
	sortOrder := ctx.FormString("sort")
	if sortOrder == "" {
		sortOrder = string(repo_model.SubjectSortRecentUpdate)
	}

	// Map sort order to database ORDER BY clause
	orderBy := repo_model.SubjectOrderByMap[repo_model.SubjectSortType(sortOrder)]
	if orderBy == "" {
		sortOrder = string(repo_model.SubjectSortRecentUpdate)
		orderBy = repo_model.SubjectOrderByMap[repo_model.SubjectSortRecentUpdate]
	}
	ctx.Data["SortType"] = sortOrder

	// Get search keyword
	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	// Search subjects
	subjects, count, err := repo_model.FindSubjects(ctx, repo_model.FindSubjectsOptions{
		ListOptions: db.ListOptions{
			Page:     page,
			PageSize: setting.UI.ExplorePagingNum,
		},
		Keyword: keyword,
		OrderBy: orderBy,
	})
	if err != nil {
		ctx.ServerError("FindSubjects", err)
		return
	}

	// For each subject, get the repository count
	type SubjectWithCount struct {
		*repo_model.Subject
		RepoCount     int64
		RootRepoCount int64
	}

	subjectsWithCount := make([]*SubjectWithCount, 0, len(subjects))
	for _, subject := range subjects {
		repoCount, err := repo_model.CountRepositoriesBySubject(ctx, subject.ID)
		if err != nil {
			ctx.ServerError("CountRepositoriesBySubject", err)
			return
		}

		rootRepoCount, err := repo_model.CountRootRepositoriesBySubject(ctx, subject.ID)
		if err != nil {
			ctx.ServerError("CountRootRepositoriesBySubject", err)
			return
		}

		subjectsWithCount = append(subjectsWithCount, &SubjectWithCount{
			Subject:       subject,
			RepoCount:     repoCount,
			RootRepoCount: rootRepoCount,
		})
	}

	ctx.Data["Total"] = count
	ctx.Data["Subjects"] = subjectsWithCount

	pager := context.NewPagination(int(count), setting.UI.ExplorePagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplExploreSubjects)
}

// RepoHistory renders repository history page - an alternative interface to repo home
func RepoHistory(ctx *context.Context) {
	// Set page metadata
	ctx.Data["Title"] = ctx.Repo.Repository.FullName() + " - History View"
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["PageIsRepoHistory"] = true
	ctx.Data["IsRepoHistoryView"] = true

	// Determine which sub-view to render (bubble | table | article)
	view := ctx.FormString("view")
	if view == "" {
		// Default to bubble view per UX requirement
		view = "bubble"
	}
	ctx.Data["HistoryView"] = view
	ctx.Data["IsBubbleView"] = view == "bubble"
	ctx.Data["IsTableView"] = view == "table"
	ctx.Data["IsArticleView"] = view == "article"

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

	// For Article view, handle mode parameter and load README content
	if ctx.Data["IsArticleView"] == true {
		prepareArticleView(ctx, gitRepo, commit, entries, defaultBranch)
		if ctx.Written() {
			return
		}
	}

	// Render the history view template
	ctx.HTML(http.StatusOK, "explore/repo_history")
}

// RepoArticle renders the dedicated article page (Read/Edit/History tabs)
// It reuses the repository history renderer but forces the view to "article".
func RepoArticle(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Repo.Repository.FullName() + " - Article"
	ctx.Data["PageIsExploreRepositories"] = true
	ctx.Data["PageIsRepoHistory"] = true
	ctx.Data["IsRepoHistoryView"] = true
	ctx.Data["IsSubjectContext"] = true
	ctx.Data["IsRepoHistoryView"] = true
	ctx.Data["IsSubjectContext"] = false

	// Force article view (set flags directly)
	ctx.Data["HistoryView"] = "article"
	ctx.Data["IsBubbleView"] = false
	ctx.Data["IsTableView"] = false
	ctx.Data["IsArticleView"] = true

	renderRepositoryHistory(ctx)
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

// prepareArticleView prepares data for the article view (README display with read/edit/history modes)
func prepareArticleView(ctx *context.Context, gitRepo *git.Repository, commit *git.Commit, entries []*git.TreeEntry, defaultBranch string) {
	// Determine mode (read/edit/history)
	mode := ctx.FormString("mode")
	if mode == "" {
		mode = "read"
	}
	ctx.Data["ArticleMode"] = mode
	ctx.Data["IsArticleModeRead"] = mode == "read"
	ctx.Data["IsArticleModeEdit"] = mode == "edit"
	ctx.Data["IsArticleModeHistory"] = mode == "history"

	// Find README.md file
	readmeFile := findReadmeInEntries(entries)
	if readmeFile == nil {
		ctx.Data["ReadmeError"] = "No README.md file found in repository"
		return
	}

	readmeTreePath := readmeFile.Name()
	ctx.Data["ReadmeTreePath"] = readmeTreePath

	// Get the blob for the README
	blob := readmeFile.Blob()
	if blob == nil {
		ctx.ServerError("Blob is nil", errors.New("readme blob is nil"))
		return
	}

	// Get contributor count for the readme file
	contributorCount, err := getFileContributorCount(gitRepo, defaultBranch, readmeTreePath)
	if err != nil {
		log.Warn("Failed to get contributor count: %v", err)
		contributorCount = 0
	}
	ctx.Data["ReadmeContributorCount"] = contributorCount

	// Get last commit for the readme file
	lastCommit, err := gitRepo.GetCommitByPath(readmeTreePath)
	if err != nil {
		log.Warn("Failed to get last commit: %v", err)
	} else {
		ctx.Data["ReadmeLastCommit"] = lastCommit
	}

	// For read mode, render the README content
	if mode == "read" {
		buf, dataRc, err := getReadmeContent(gitRepo, blob)
		if err != nil {
			ctx.ServerError("getReadmeContent", err)
			return
		}
		defer dataRc.Close()

		// Check file size
		fileSize := blob.Size()
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			return
		}

		// Detect if this is markup
		if markupType := markup.DetectMarkupTypeByFileName(readmeFile.Name()); markupType != "" {
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType

			rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
				CurrentRefPath:  path.Join("branch", util.PathEscapeSegments(defaultBranch)),
				CurrentTreePath: "",
			}).
				WithMarkupType(markupType).
				WithRelativePath(readmeTreePath)

			rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
			var escapeStatus *charset.EscapeStatus
			escapeStatus, ctx.Data["FileContent"], err = markupRender(ctx, rctx, rd)
			if err != nil {
				log.Error("Render failed for %s in %-v: %v", readmeFile.Name(), ctx.Repo.Repository, err)
				ctx.Data["IsMarkup"] = false
			}
			ctx.Data["EscapeStatus"] = escapeStatus
		}

		if ctx.Data["IsMarkup"] != true {
			ctx.Data["IsPlainText"] = true
			rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
			content, err := io.ReadAll(rd)
			if err != nil {
				log.Error("Read readme content failed: %v", err)
			}
			contentEscaped := template.HTMLEscapeString(util.UnsafeBytesToString(content))
			ctx.Data["EscapeStatus"], ctx.Data["FileContent"] = charset.EscapeControlHTML(template.HTML(contentEscaped), ctx.Locale)
		}

		ctx.Data["FileSize"] = fileSize
		ctx.Data["CanEditReadmeFile"] = ctx.Repo.Repository.CanEnableEditor()
	} else if mode == "edit" {
		// For edit mode, load raw content
		buf, dataRc, err := getReadmeContent(gitRepo, blob)
		if err != nil {
			ctx.ServerError("getReadmeContent", err)
			return
		}
		defer dataRc.Close()

		fileSize := blob.Size()
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["NotEditableReason"] = ctx.Tr("repo.editor.cannot_edit_too_large_file")
		} else {
			allContent, err := io.ReadAll(io.MultiReader(bytes.NewReader(buf), dataRc))
			if err != nil {
				ctx.ServerError("ReadAll", err)
				return
			}
			if content, err := charset.ToUTF8(allContent, charset.ConvertOpts{KeepBOM: true}); err != nil {
				ctx.Data["FileContent"] = string(allContent)
			} else {
				ctx.Data["FileContent"] = content
			}
		}
		ctx.Data["FileSize"] = fileSize
	} else if mode == "history" {
		// For history mode, get file commit history
		commitsCount, err := gitRepo.FileCommitsCount(defaultBranch, readmeTreePath)
		if err != nil {
			ctx.ServerError("FileCommitsCount", err)
			return
		}

		page := ctx.FormInt("page")
		if page <= 0 {
			page = 1
		}

		commits, err := gitRepo.CommitsByFileAndRange(
			git.CommitsByFileAndRangeOptions{
				Revision: defaultBranch,
				File:     readmeTreePath,
				Page:     page,
			})
		if err != nil {
			ctx.ServerError("CommitsByFileAndRange", err)
			return
		}

		// Process commits to attach user information
		processedCommits, err := processGitCommits(ctx, commits)
		if err != nil {
			ctx.ServerError("processGitCommits", err)
			return
		}

		ctx.Data["Commits"] = processedCommits
		ctx.Data["CommitCount"] = commitsCount
		ctx.Data["FileTreePath"] = readmeTreePath

		pager := context.NewPagination(int(commitsCount), setting.Git.CommitsRangeSize, page, 5)
		pager.AddParamFromRequest(ctx.Req)
		ctx.Data["Page"] = pager
	}
}

// findReadmeInEntries finds a README file in the given entries
func findReadmeInEntries(entries []*git.TreeEntry) *git.TreeEntry {
	// Look for readme.md (case insensitive)
	for _, entry := range entries {
		if entry.IsRegular() || entry.IsExecutable() {
			name := strings.ToLower(entry.Name())
			if name == "readme.md" || name == "readme" || name == "readme.txt" {
				return entry
			}
		}
	}
	return nil
}

// getReadmeContent reads content from a blob
func getReadmeContent(gitRepo *git.Repository, blob *git.Blob) ([]byte, io.ReadCloser, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return nil, nil, err
	}

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	return buf, dataRc, nil
}

// markupRender renders markup content
func markupRender(ctx *context.Context, rctx *markup.RenderContext, rd io.Reader) (*charset.EscapeStatus, template.HTML, error) {
	var buf bytes.Buffer
	err := markup.Render(rctx, rd, &buf)
	if err != nil {
		return nil, "", err
	}
	escapeStatus, content := charset.EscapeControlHTML(template.HTML(buf.String()), ctx.Locale)
	return escapeStatus, content, nil
}

// processGitCommits processes git commits to attach user information
func processGitCommits(ctx *context.Context, commits []*git.Commit) ([]*user_model.UserCommit, error) {
	// Validate commits with emails to attach user information
	userCommits, err := user_model.ValidateCommitsWithEmails(ctx, commits)
	if err != nil {
		return nil, err
	}
	return userCommits, nil
}

// getFileContributorCount gets the number of unique contributors for a specific file
func getFileContributorCount(gitRepo *git.Repository, branch, filePath string) (int64, error) {
	// Use git shortlog to get unique contributors for the file
	stdout, _, err := gitcmd.NewCommand("shortlog", "-sn").
		AddDynamicArguments(branch, "--", filePath).
		RunStdString(gitRepo.Ctx, &gitcmd.RunOpts{Dir: gitRepo.Path})
	if err != nil {
		return 0, err
	}

	// Count the number of lines (each line represents a unique contributor)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0, nil // No contributors
	}

	return int64(len(lines)), nil
}
