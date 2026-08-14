// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	gocontext "context"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"gitea.dev/models/renderhelper"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/base"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	git_service "gitea.dev/services/git"
	notify_service "gitea.dev/services/notify"
	repo_service "gitea.dev/services/repository"
	wiki_service "gitea.dev/services/wiki"
)

const (
	tplWikiStart    templates.TplName = "repo/wiki/start"
	tplWikiView     templates.TplName = "repo/wiki/view"
	tplWikiRevision templates.TplName = "repo/wiki/revision"
	tplWikiNew      templates.TplName = "repo/wiki/new"
	tplWikiPages    templates.TplName = "repo/wiki/pages"
)

// MustEnableWiki check if wiki is enabled, if external then redirect
func MustEnableWiki(ctx *context.Context) {
	if !ctx.Repo.Permission.CanRead(unit.TypeWiki) &&
		!ctx.Repo.Permission.CanRead(unit.TypeExternalWiki) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User %-v cannot read %-v or %-v of repo %-v\n"+
				"User in repo has Permissions: %-+v",
				ctx.Doer,
				unit.TypeWiki,
				unit.TypeExternalWiki,
				ctx.Repo.Repository,
				ctx.Repo.Permission)
		}
		ctx.NotFound(nil)
		return
	}

	repoUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalWiki)
	if err == nil {
		ctx.Redirect(repoUnit.ExternalWikiConfig().ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meta information
type PageMeta struct {
	Name         string
	IsDir        bool
	SubURL       string
	GitEntryName string
	UpdatedUnix  timeutil.TimeStamp
}

type WikiTreeNode struct {
	Name     string
	IsDir    bool
	Open     bool
	SubURL   string
	Children []*WikiTreeNode
}

func buildWikiTree(ctx gocontext.Context, wikiRepo *git.Repository, tree *git.Tree, basePath string) ([]*WikiTreeNode, error) {
	entries, err := tree.ListEntries(ctx, wikiRepo)
	if err != nil {
		return nil, err
	}
	entries.CustomSort(base.NaturalSortCompare)

	nodes := make([]*WikiTreeNode, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsRegular() && !entry.IsDir() {
			continue
		}

		if entry.IsDir() {
			wikiName, err := wiki_service.GitDirPathToWebPath(entry.Name())
			if err != nil {
				return nil, err
			}
			children, err := buildWikiTree(ctx, wikiRepo, entry.Tree(ctx, wikiRepo), path.Join(basePath, string(wikiName)))
			if err != nil {
				return nil, err
			}
			if len(children) == 0 {
				continue
			}
			_, displayName := wiki_service.WebPathToUserTitle(wikiName)
			nodes = append(nodes, &WikiTreeNode{
				Name:     displayName,
				IsDir:    true,
				SubURL:   wiki_service.WebPathToURLPath(wiki_service.WebPath(path.Join(basePath, string(wikiName)))),
				Children: children,
			})
			continue
		}

		wikiName, err := wiki_service.GitPathToWebPath(entry.Name())
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			return nil, err
		}
		if basePath == "" && (wikiName == "_Sidebar" || wikiName == "_Footer") {
			continue
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		nodes = append(nodes, &WikiTreeNode{
			Name:   displayName,
			SubURL: wiki_service.WebPathToURLPath(wiki_service.WebPath(path.Join(basePath, string(wikiName)))),
		})
	}
	return nodes, nil
}

func openWikiTreePath(nodes []*WikiTreeNode, pagePath string) bool {
	for _, node := range nodes {
		if node.IsDir {
			node.Open = openWikiTreePath(node.Children, pagePath)
			if node.Open {
				return true
			}
			continue
		}
		if node.SubURL == pagePath {
			return true
		}
	}
	return false
}

func treeHasWikiPage(ctx gocontext.Context, wikiRepo *git.Repository, tree *git.Tree) bool {
	if tree == nil {
		return false
	}
	entries, err := tree.ListEntries(ctx, wikiRepo)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsRegular() {
			if _, err := wiki_service.GitPathToWebPath(entry.Name()); err == nil {
				return true
			}
		} else if entry.IsDir() && treeHasWikiPage(ctx, wikiRepo, entry.Tree(ctx, wikiRepo)) {
			return true
		}
	}
	return false
}

// findEntryForFile finds the tree entry for a target filepath.
func findEntryForFile(ctx gocontext.Context, wikiRepo *git.Repository, commit *git.Commit, target string) (*git.TreeEntry, error) {
	entry, err := commit.GetTreeEntryByPath(ctx, wikiRepo, target)
	if err != nil && !git.IsErrNotExist(err) {
		return nil, err
	}
	if entry != nil {
		return entry, nil
	}

	// Then the unescaped, the shortest alternative
	var unescapedTarget string
	if unescapedTarget, err = url.QueryUnescape(target); err != nil {
		return nil, err
	}
	return commit.GetTreeEntryByPath(ctx, wikiRepo, unescapedTarget)
}

func findWikiRepoCommit(ctx *context.Context) (*git.Repository, *git.Commit, error) {
	wikiGitRepo, errGitRepo := git.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository.WikiStorageRepo())
	if errGitRepo != nil {
		ctx.ServerError("OpenRepository", errGitRepo)
		return nil, nil, errGitRepo
	}

	commit, errCommit := wikiGitRepo.GetBranchCommit(ctx, ctx.Repo.Repository.DefaultWikiBranch)
	if git.IsErrNotExist(errCommit) {
		// if the default branch recorded in database is out of sync, then re-sync it
		gitRepoDefaultBranch, errBranch := git.GetDefaultBranch(ctx, ctx.Repo.Repository.WikiStorageRepo())
		if errBranch != nil {
			return wikiGitRepo, nil, errBranch
		}
		// update the default branch in the database
		errDb := repo_model.UpdateRepositoryColsNoAutoTime(ctx, &repo_model.Repository{ID: ctx.Repo.Repository.ID, DefaultWikiBranch: gitRepoDefaultBranch}, "default_wiki_branch")
		if errDb != nil {
			return wikiGitRepo, nil, errDb
		}
		ctx.Repo.Repository.DefaultWikiBranch = gitRepoDefaultBranch
		// retry to get the commit from the correct default branch
		commit, errCommit = wikiGitRepo.GetBranchCommit(ctx, ctx.Repo.Repository.DefaultWikiBranch)
	}
	if errCommit != nil {
		return wikiGitRepo, nil, errCommit
	}
	return wikiGitRepo, commit, nil
}

// wikiContentsByEntry returns the contents of the wiki page referenced by the
// given tree entry. Writes to ctx if an error occurs.
func wikiContentsByEntry(ctx *context.Context, wikiRepo *git.Repository, entry *git.TreeEntry) []byte {
	reader, err := entry.Blob(wikiRepo).DataAsync(ctx)
	if err != nil {
		ctx.ServerError("Blob.Data", err)
		return nil
	}
	defer reader.Close()
	content, err := util.ReadWithLimit(reader, 5*1024*1024) // 5MB should be enough for a wiki page
	if err != nil {
		ctx.ServerError("ReadAll", err)
		return nil
	}
	return content
}

// wikiEntryByName returns the entry of a wiki page, along with a boolean
// indicating whether the entry exists. Writes to ctx if an error occurs.
// The last return value indicates whether the file should be returned as a raw file
func wikiEntryByName(ctx *context.Context, wikiRepo *git.Repository, commit *git.Commit, wikiName wiki_service.WebPath) (*git.TreeEntry, string, bool, bool) {
	isRaw := false
	gitFilename := wiki_service.WebPathToGitPath(wikiName)
	entry, err := findEntryForFile(ctx, wikiRepo, commit, gitFilename)
	if err != nil && !git.IsErrNotExist(err) {
		ctx.ServerError("findEntryForFile", err)
		return nil, "", false, false
	}
	if entry == nil {
		// check if the file without ".md" suffix exists
		gitFilename := strings.TrimSuffix(gitFilename, ".md")
		entry, err = findEntryForFile(ctx, wikiRepo, commit, gitFilename)
		if err != nil && !git.IsErrNotExist(err) {
			ctx.ServerError("findEntryForFile", err)
			return nil, "", false, false
		}
		isRaw = true
	}
	if entry == nil {
		return nil, "", true, false
	}
	return entry, gitFilename, false, isRaw
}

// wikiContentsByName returns the contents of a wiki page, along with a boolean
// indicating whether the page exists. Writes to ctx if an error occurs.
func wikiContentsByName(ctx *context.Context, wikiRepo *git.Repository, commit *git.Commit, wikiName wiki_service.WebPath) ([]byte, *git.TreeEntry, string, bool) {
	entry, gitFilename, noEntry, _ := wikiEntryByName(ctx, wikiRepo, commit, wikiName)
	if entry == nil {
		return nil, nil, "", true
	}
	return wikiContentsByEntry(ctx, wikiRepo, entry), entry, gitFilename, noEntry
}

func renderViewPage(ctx *context.Context) (*git.Repository, *git.TreeEntry) {
	wikiGitRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return nil, nil
	}
	// get the wiki pages list.
	entries, err := commit.Tree().ListEntries(ctx, wikiGitRepo)
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return nil, nil
	}
	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsRegular() && (!entry.IsDir() || !treeHasWikiPage(ctx, wikiGitRepo, entry.Tree(ctx, wikiGitRepo))) {
			continue
		}
		var wikiName wiki_service.WebPath
		if entry.IsDir() {
			wikiName, err = wiki_service.GitDirPathToWebPath(entry.Name())
		} else {
			wikiName, err = wiki_service.GitPathToWebPath(entry.Name())
		}
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.ServerError("WikiFilenameToName", err)
			return nil, nil
		} else if wikiName == "_Sidebar" || wikiName == "_Footer" {
			continue
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, PageMeta{
			Name:         displayName,
			IsDir:        entry.IsDir(),
			SubURL:       wiki_service.WebPathToURLPath(wikiName),
			GitEntryName: entry.Name(),
		})
	}
	ctx.Data["Pages"] = pages

	// get requested page name
	pageName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return nil, nil
	}
	if len(pageName) == 0 {
		pageName = "Home"
	}
	wikiTree, err := buildWikiTree(ctx, wikiGitRepo, commit.Tree(), "")
	if err != nil {
		ctx.ServerError("buildWikiTree", err)
		return nil, nil
	}
	openWikiTreePath(wikiTree, wiki_service.WebPathToURLPath(pageName))
	ctx.Data["WikiTree"] = wikiTree

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	if pageDir := path.Dir(string(pageName)); pageDir != "." {
		ctx.Data["PageDir"] = wiki_service.WebPathToURLPath(wiki_service.WebPath(pageDir))
	}
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	isSideBar := pageName == "_Sidebar"
	isFooter := pageName == "_Footer"

	// lookup filename in wiki - get gitTree entry , real filename
	entry, pageFilename, noEntry, isRaw := wikiEntryByName(ctx, wikiGitRepo, commit, pageName)
	if noEntry {
		dirEntry, err := commit.GetTreeEntryByPath(ctx, wikiGitRepo, wiki_service.WebDirPathToGitPath(pageName))
		if err == nil && dirEntry.IsDir() {
			ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(pageName) + "?action=_pages")
			return nil, nil
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/raw/" + string(pageName))
	}
	if entry == nil || ctx.Written() {
		return nil, nil
	}

	// get page content
	data := wikiContentsByEntry(ctx, wikiGitRepo, entry)
	if ctx.Written() {
		return nil, nil
	}

	pageDir := path.Dir(pageFilename)
	if pageDir == "." {
		pageDir = ""
	}
	rctx := renderhelper.NewRenderContextRepoWiki(ctx, ctx.Repo.Repository, renderhelper.RepoWikiOptions{CurrentTreePath: pageDir})

	renderFn := func(rctx *markup.RenderContext, data []byte) (escaped *charset.EscapeStatus, output template.HTML, err error) {
		buf := &strings.Builder{}
		markupRd, markupWr := io.Pipe()
		defer markupWr.Close()
		done := make(chan struct{})
		go func() {
			escaped, _ = charset.EscapeControlReader(markupRd, buf, ctx.Locale, charset.EscapeOptionsForView())
			output = template.HTML(buf.String())
			buf.Reset()
			close(done)
		}()

		err = markdown.Render(rctx, bytes.NewReader(data), markupWr)
		_ = markupWr.CloseWithError(err)
		<-done
		return escaped, output, err
	}

	ctx.Data["EscapeStatus"], ctx.Data["WikiContentHTML"], err = renderFn(rctx, data)
	if err != nil {
		ctx.ServerError("Render", err)
		return nil, nil
	}

	if rctx.TocShowInSection == markup.TocShowInSidebar && len(rctx.TocHeadingItems) > 0 {
		sb := strings.Builder{}
		markup.RenderTocHeadingItems(rctx, map[string]string{"open": ""}, &sb)
		ctx.Data["WikiSidebarTocHTML"] = template.HTML(sb.String())
	}

	if !isSideBar {
		sidebarContent, _, _, _ := wikiContentsByName(ctx, wikiGitRepo, commit, "_Sidebar")
		if ctx.Written() {
			return nil, nil
		}
		ctx.Data["WikiSidebarEscapeStatus"], ctx.Data["WikiSidebarHTML"], err = renderFn(renderhelper.NewRenderContextRepoWiki(ctx, ctx.Repo.Repository), sidebarContent)
		if err != nil {
			ctx.ServerError("Render", err)
			return nil, nil
		}
	}

	if !isFooter {
		footerContent, _, _, _ := wikiContentsByName(ctx, wikiGitRepo, commit, "_Footer")
		if ctx.Written() {
			return nil, nil
		}
		ctx.Data["WikiFooterEscapeStatus"], ctx.Data["WikiFooterHTML"], err = renderFn(renderhelper.NewRenderContextRepoWiki(ctx, ctx.Repo.Repository), footerContent)
		if err != nil {
			ctx.ServerError("Render", err)
			return nil, nil
		}
	}

	// get commit count - wiki revisions
	commitsCount, _ := git.FileCommitsCount(ctx, ctx.Repo.Repository.WikiStorageRepo(), ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	return wikiGitRepo, entry
}

func renderRevisionPage(ctx *context.Context) (*git.Repository, *git.TreeEntry) {
	wikiGitRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return nil, nil
	}

	// get requested page name
	pageName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return nil, nil
	}
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	if pageDir := path.Dir(string(pageName)); pageDir != "." {
		ctx.Data["PageDir"] = wiki_service.WebPathToURLPath(wiki_service.WebPath(pageDir))
	}
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	// lookup filename in wiki - get page content, gitTree entry , real filename
	_, entry, pageFilename, noEntry := wikiContentsByName(ctx, wikiGitRepo, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if entry == nil || ctx.Written() {
		return nil, nil
	}

	// get commit count - wiki revisions
	commitsCount, _ := git.FileCommitsCount(ctx, ctx.Repo.Repository.WikiStorageRepo(), ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	// get page
	page := max(ctx.FormInt("page"), 1)

	// get Commit Count
	commitsHistory, _, err := wikiGitRepo.CommitsByFileAndRange(ctx,
		git.CommitsByFileAndRangeOptions{
			Revision: ctx.Repo.Repository.DefaultWikiBranch,
			File:     pageFilename,
			Page:     page,
		})
	if err != nil {
		ctx.ServerError("CommitsByFileAndRange", err)
		return nil, nil
	}
	ctx.Data["Commits"], err = git_service.ConvertFromGitCommit(ctx, commitsHistory, ctx.Repo.Repository, "") // no current ref sub path for wiki commit list
	if err != nil {
		ctx.ServerError("ConvertFromGitCommit", err)
		return nil, nil
	}

	pager := context.NewPagination(commitsCount, setting.Git.CommitsRangeSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	return wikiGitRepo, entry
}

func renderEditPage(ctx *context.Context) {
	wikiGitRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	// get requested page name
	pageName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	if pageDir := path.Dir(string(pageName)); pageDir != "." {
		ctx.Data["PageDir"] = wiki_service.WebPathToURLPath(wiki_service.WebPath(pageDir))
	}
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	// lookup filename in wiki -  gitTree entry , real filename
	entry, _, noEntry, isRaw := wikiEntryByName(ctx, wikiGitRepo, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.HTTPError(http.StatusForbidden, "Editing of raw wiki files is not allowed")
	}
	if entry == nil || ctx.Written() {
		return
	}

	// get wiki page content
	data := wikiContentsByEntry(ctx, wikiGitRepo, entry)
	if ctx.Written() {
		return
	}

	ctx.Data["WikiEditContent"] = string(data)
}

// WikiPost renders post of wiki page
func WikiPost(ctx *context.Context) {
	switch ctx.FormString("action") {
	case "_new":
		if !ctx.Repo.Permission.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWikiPost(ctx)
		return
	case "_delete":
		if !ctx.Repo.Permission.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		DeleteWikiPagePost(ctx)
		return
	}

	if !ctx.Repo.Permission.CanWrite(unit.TypeWiki) {
		ctx.NotFound(nil)
		return
	}
	EditWikiPost(ctx)
}

// Wiki renders single wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.Permission.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	switch ctx.FormString("action") {
	case "_pages":
		WikiPages(ctx)
		return
	case "_revision":
		WikiRevision(ctx)
		return
	case "_edit":
		if !ctx.Repo.Permission.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		EditWiki(ctx)
		return
	case "_new":
		if !ctx.Repo.Permission.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWiki(ctx)
		return
	}

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiGitRepo, entry := renderViewPage(ctx)
	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	wikiName = util.IfZero(wikiName, "Home")
	wikiPath := wiki_service.WebPathToGitPath(wikiName)
	detectedRender := markup.DetectRendererTypeByFilename(wikiPath)
	if detectedRender == nil || detectedRender.Name() != markdown.MarkupName {
		ctx.Data["FormatWarning"] = "File extension " + path.Ext(wikiPath) + " is not supported at the moment. Rendered as Markdown."
	}
	// Get last change information.
	lastCommit, err := wikiGitRepo.GetCommitByPath(ctx, wikiPath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}
	ctx.Data["Committer"] = lastCommit.Committer

	ctx.HTML(http.StatusOK, tplWikiView)
}

// WikiRevision renders file revision list of wiki page
func WikiRevision(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.Permission.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiGitRepo, entry := renderRevisionPage(ctx)
	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	// Get last change information.
	wikiName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	wikiName = util.IfZero(wikiName, "Home")
	wikiPath := wiki_service.WebPathToGitPath(wikiName)
	lastCommit, err := wikiGitRepo.GetCommitByPath(ctx, wikiPath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}
	ctx.Data["Committer"] = lastCommit.Committer

	ctx.HTML(http.StatusOK, tplWikiRevision)
}

// WikiPages render wiki pages list page
func WikiPages(ctx *context.Context) {
	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	ctx.Data["Title"] = ctx.Tr("repo.wiki.pages")
	ctx.Data["CanWriteWiki"] = ctx.Repo.Permission.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	wikiGitRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	dirPath, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	treePath := wiki_service.WebDirPathToGitPath(dirPath)
	tree, err := commit.SubTree(ctx, wikiGitRepo, treePath)
	if err != nil {
		ctx.ServerError("SubTree", err)
		return
	}

	allEntries, err := tree.ListEntries(ctx, wikiGitRepo)
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	allEntries.CustomSort(base.NaturalSortCompare)

	entries, _, err := allEntries.GetCommitsInfo(ctx, 0, ctx.Repo.RepoLink, wikiGitRepo, commit, treePath)
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return
	}

	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.Entry.IsRegular() && (!entry.Entry.IsDir() || !treeHasWikiPage(ctx, wikiGitRepo, entry.Entry.Tree(ctx, wikiGitRepo))) {
			continue
		}
		var wikiName wiki_service.WebPath
		if entry.Entry.IsDir() {
			wikiName, err = wiki_service.GitDirPathToWebPath(entry.Entry.Name())
		} else {
			wikiName, err = wiki_service.GitPathToWebPath(entry.Entry.Name())
		}
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.ServerError("WikiFilenameToName", err)
			return
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, PageMeta{
			Name:         displayName,
			IsDir:        entry.Entry.IsDir(),
			SubURL:       wiki_service.WebPathToURLPath(wiki_service.WebPath(path.Join(string(dirPath), string(wikiName)))),
			GitEntryName: path.Join(treePath, entry.Entry.Name()),
			UpdatedUnix:  timeutil.TimeStamp(entry.Commit.Committer.When.Unix()),
		})
	}
	ctx.Data["Pages"] = pages
	ctx.Data["PageDir"] = wiki_service.WebPathToURLPath(dirPath)
	if parentDir := path.Dir(string(dirPath)); parentDir != "." {
		ctx.Data["ParentPageDir"] = wiki_service.WebPathToURLPath(wiki_service.WebPath(parentDir))
	}

	ctx.HTML(http.StatusOK, tplWikiPages)
}

// WikiRaw outputs raw blob requested by user (image for example)
func WikiRaw(ctx *context.Context) {
	wikiGitRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(nil)
			return
		}
		ctx.ServerError("findEntryForfile", err)
		return
	}

	providedWebPath, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	providedGitPath := wiki_service.WebPathToGitPath(providedWebPath)
	var entry *git.TreeEntry
	if commit != nil {
		// Try to find a file with that name
		entry, err = findEntryForFile(ctx, wikiGitRepo, commit, providedGitPath)
		if err != nil && !git.IsErrNotExist(err) {
			ctx.ServerError("findFile", err)
			return
		}

		if entry == nil {
			// Try to find a wiki page with that name
			providedGitPath = strings.TrimSuffix(providedGitPath, ".md")
			entry, err = findEntryForFile(ctx, wikiGitRepo, commit, providedGitPath)
			if err != nil && !git.IsErrNotExist(err) {
				ctx.ServerError("findFile", err)
				return
			}
		}
	}

	if entry != nil {
		if err = common.ServeBlob(ctx.Base, ctx.Repo.Repository, ctx.Repo.TreePath, entry.Blob(wikiGitRepo), nil); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	}

	ctx.NotFound(nil)
}

func wikiHandleEditError(ctx *context.Context, wikiName wiki_service.WebPath, err error) {
	if repo_model.IsErrWikiReservedName(err) {
		ctx.JSONErrorWithField(ctx.Tr("repo.wiki.reserved_page", wikiName), "title")
	} else if repo_model.IsErrWikiAlreadyExist(err) {
		ctx.JSONErrorWithField(ctx.Tr("repo.wiki.page_already_exists"), "title")
	} else {
		ctx.ServerError("EditWiki", err)
	}
}

// NewWiki render wiki create page
func NewWiki(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	dirPath, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	ctx.Data["PageDir"] = wiki_service.WebPathToURLPath(dirPath)

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Data["title"] = "Home"
	}
	if ctx.FormString("title") != "" {
		ctx.Data["title"] = ctx.FormString("title")
	}

	ctx.HTML(http.StatusOK, tplWikiNew)
}

// NewWikiPost response for wiki create request
func NewWikiPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.WikiEditForm](ctx)
	if form == nil {
		return
	}

	dirPath, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	wikiName := wiki_service.UserTitleToWebPath(string(dirPath), form.Title)
	if form.Message == "" {
		form.Message = ctx.Locale.TrString("repo.editor.add", form.Title)
	}

	err = wiki_service.AddWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName, form.Content, form.Message)
	if err != nil {
		wikiHandleEditError(ctx, wikiName, err)
		return
	}

	notify_service.NewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(wikiName))
}

// EditWiki render wiki modify page
func EditWiki(ctx *context.Context) {
	ctx.Data["PageIsWikiEdit"] = true

	if !repo_service.HasWiki(ctx, ctx.Repo.Repository) {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	renderEditPage(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplWikiNew)
}

// EditWikiPost response for wiki modify request
func EditWikiPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.WikiEditForm](ctx)
	if form == nil {
		return
	}

	oldWikiName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	dirPath := path.Dir(string(oldWikiName))
	if dirPath == "." {
		dirPath = ""
	}
	newWikiName := wiki_service.UserTitleToWebPath(dirPath, form.Title)
	if form.Message == "" {
		form.Message = ctx.Locale.TrString("repo.editor.update", form.Title)
	}

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		wikiHandleEditError(ctx, newWikiName, err)
		return
	}

	notify_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(newWikiName))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	wikiName, err := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if err != nil {
		ctx.NotFound(err)
		return
	}
	wikiName = util.IfZero(wikiName, "Home")
	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		ctx.ServerError("DeleteWikiPage", err)
		return
	}

	notify_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/wiki/")
}
