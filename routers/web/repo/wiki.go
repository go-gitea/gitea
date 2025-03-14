// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	gocontext "context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	git_service "code.gitea.io/gitea/services/git"
	notify_service "code.gitea.io/gitea/services/notify"
	wiki_service "code.gitea.io/gitea/services/wiki"
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
	if !ctx.Repo.CanRead(unit.TypeWiki) &&
		!ctx.Repo.CanRead(unit.TypeExternalWiki) {
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

	unit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalWiki)
	if err == nil {
		ctx.Redirect(unit.ExternalWikiConfig().ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meta information
type PageMeta struct {
	Name         string
	SubURL       string
	GitEntryName string
	UpdatedUnix  timeutil.TimeStamp
}

// findEntryForFile finds the tree entry for a target filepath.
func findEntryForFile(commit *git.Commit, target string) (*git.TreeEntry, error) {
	entry, err := commit.GetTreeEntryByPath(target)
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
	return commit.GetTreeEntryByPath(unescapedTarget)
}

func findWikiRepoCommit(ctx *context.Context) (*git.Repository, *git.Commit, error) {
	wikiGitRepo, errGitRepo := gitrepo.OpenWikiRepository(ctx, ctx.Repo.Repository)
	if errGitRepo != nil {
		ctx.ServerError("OpenRepository", errGitRepo)
		return nil, nil, errGitRepo
	}

	commit, errCommit := wikiGitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultWikiBranch)
	if git.IsErrNotExist(errCommit) {
		// if the default branch recorded in database is out of sync, then re-sync it
		gitRepoDefaultBranch, errBranch := gitrepo.GetWikiDefaultBranch(ctx, ctx.Repo.Repository)
		if errBranch != nil {
			return wikiGitRepo, nil, errBranch
		}
		// update the default branch in the database
		errDb := repo_model.UpdateRepositoryCols(ctx, &repo_model.Repository{ID: ctx.Repo.Repository.ID, DefaultWikiBranch: gitRepoDefaultBranch}, "default_wiki_branch")
		if errDb != nil {
			return wikiGitRepo, nil, errDb
		}
		ctx.Repo.Repository.DefaultWikiBranch = gitRepoDefaultBranch
		// retry to get the commit from the correct default branch
		commit, errCommit = wikiGitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultWikiBranch)
	}
	if errCommit != nil {
		return wikiGitRepo, nil, errCommit
	}
	return wikiGitRepo, commit, nil
}

// wikiContentsByEntry returns the contents of the wiki page referenced by the
// given tree entry. Writes to ctx if an error occurs.
func wikiContentsByEntry(ctx *context.Context, entry *git.TreeEntry) []byte {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		ctx.ServerError("Blob.Data", err)
		return nil
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		ctx.ServerError("ReadAll", err)
		return nil
	}
	return content
}

// wikiEntryByName returns the entry of a wiki page, along with a boolean
// indicating whether the entry exists. Writes to ctx if an error occurs.
// The last return value indicates whether the file should be returned as a raw file
func wikiEntryByName(ctx *context.Context, commit *git.Commit, wikiName wiki_service.WebPath) (*git.TreeEntry, string, bool, bool) {
	isRaw := false
	gitFilename := wiki_service.WebPathToGitPath(wikiName)
	entry, err := findEntryForFile(commit, gitFilename)
	if err != nil && !git.IsErrNotExist(err) {
		ctx.ServerError("findEntryForFile", err)
		return nil, "", false, false
	}
	if entry == nil {
		// check if the file without ".md" suffix exists
		gitFilename := strings.TrimSuffix(gitFilename, ".md")
		entry, err = findEntryForFile(commit, gitFilename)
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
func wikiContentsByName(ctx *context.Context, commit *git.Commit, wikiName wiki_service.WebPath) ([]byte, *git.TreeEntry, string, bool) {
	entry, gitFilename, noEntry, _ := wikiEntryByName(ctx, commit, wikiName)
	if entry == nil {
		return nil, nil, "", true
	}
	return wikiContentsByEntry(ctx, entry), entry, gitFilename, noEntry
}

func renderViewPage(ctx *context.Context) (*git.Repository, *git.TreeEntry) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return nil, nil
	}

	// Get page list.
	entries, err := commit.ListEntries()
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.ServerError("ListEntries", err)
		return nil, nil
	}
	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsRegular() {
			continue
		}
		wikiName, err := wiki_service.GitPathToWebPath(entry.Name())
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			if wikiRepo != nil {
				wikiRepo.Close()
			}
			ctx.ServerError("WikiFilenameToName", err)
			return nil, nil
		} else if wikiName == "_Sidebar" || wikiName == "_Footer" {
			continue
		}
		_, displayName := wiki_service.WebPathToUserTitle(wikiName)
		pages = append(pages, PageMeta{
			Name:         displayName,
			SubURL:       wiki_service.WebPathToURLPath(wikiName),
			GitEntryName: entry.Name(),
		})
	}
	ctx.Data["Pages"] = pages

	// get requested page name
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	isSideBar := pageName == "_Sidebar"
	isFooter := pageName == "_Footer"

	// lookup filename in wiki - get gitTree entry , real filename
	entry, pageFilename, noEntry, isRaw := wikiEntryByName(ctx, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.Redirect(util.URLJoin(ctx.Repo.RepoLink, "wiki/raw", string(pageName)))
	}
	if entry == nil || ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		return nil, nil
	}

	// get filecontent
	data := wikiContentsByEntry(ctx, entry)
	if ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		return nil, nil
	}

	var sidebarContent []byte
	if !isSideBar {
		sidebarContent, _, _, _ = wikiContentsByName(ctx, commit, "_Sidebar")
		if ctx.Written() {
			if wikiRepo != nil {
				wikiRepo.Close()
			}
			return nil, nil
		}
	} else {
		sidebarContent = data
	}

	var footerContent []byte
	if !isFooter {
		footerContent, _, _, _ = wikiContentsByName(ctx, commit, "_Footer")
		if ctx.Written() {
			if wikiRepo != nil {
				wikiRepo.Close()
			}
			return nil, nil
		}
	} else {
		footerContent = data
	}

	rctx := renderhelper.NewRenderContextRepoWiki(ctx, ctx.Repo.Repository)

	buf := &strings.Builder{}
	renderFn := func(data []byte) (escaped *charset.EscapeStatus, output string, err error) {
		markupRd, markupWr := io.Pipe()
		defer markupWr.Close()
		done := make(chan struct{})
		go func() {
			// We allow NBSP here this is rendered
			escaped, _ = charset.EscapeControlReader(markupRd, buf, ctx.Locale, charset.RuneNBSP)
			output = buf.String()
			buf.Reset()
			close(done)
		}()

		err = markdown.Render(rctx, bytes.NewReader(data), markupWr)
		_ = markupWr.CloseWithError(err)
		<-done
		return escaped, output, err
	}

	ctx.Data["EscapeStatus"], ctx.Data["content"], err = renderFn(data)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.ServerError("Render", err)
		return nil, nil
	}

	if rctx.SidebarTocNode != nil {
		sb := &strings.Builder{}
		err = markdown.SpecializedMarkdown(rctx).Renderer().Render(sb, nil, rctx.SidebarTocNode)
		if err != nil {
			log.Error("Failed to render wiki sidebar TOC: %v", err)
		} else {
			ctx.Data["sidebarTocContent"] = sb.String()
		}
	}

	if !isSideBar {
		buf.Reset()
		ctx.Data["sidebarEscapeStatus"], ctx.Data["sidebarContent"], err = renderFn(sidebarContent)
		if err != nil {
			if wikiRepo != nil {
				wikiRepo.Close()
			}
			ctx.ServerError("Render", err)
			return nil, nil
		}
		ctx.Data["sidebarPresent"] = sidebarContent != nil
	} else {
		ctx.Data["sidebarPresent"] = false
	}

	if !isFooter {
		buf.Reset()
		ctx.Data["footerEscapeStatus"], ctx.Data["footerContent"], err = renderFn(footerContent)
		if err != nil {
			if wikiRepo != nil {
				wikiRepo.Close()
			}
			ctx.ServerError("Render", err)
			return nil, nil
		}
		ctx.Data["footerPresent"] = footerContent != nil
	} else {
		ctx.Data["footerPresent"] = false
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount(ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	return wikiRepo, entry
}

func renderRevisionPage(ctx *context.Context) (*git.Repository, *git.TreeEntry) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return nil, nil
	}

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name

	// lookup filename in wiki - get filecontent, gitTree entry , real filename
	data, entry, pageFilename, noEntry := wikiContentsByName(ctx, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if entry == nil || ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		return nil, nil
	}

	ctx.Data["content"] = string(data)
	ctx.Data["sidebarPresent"] = false
	ctx.Data["sidebarContent"] = ""
	ctx.Data["footerPresent"] = false
	ctx.Data["footerContent"] = ""

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount(ctx.Repo.Repository.DefaultWikiBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	// get page
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	// get Commit Count
	commitsHistory, err := wikiRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: ctx.Repo.Repository.DefaultWikiBranch,
			File:     pageFilename,
			Page:     page,
		})
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.ServerError("CommitsByFileAndRange", err)
		return nil, nil
	}
	ctx.Data["Commits"], err = git_service.ConvertFromGitCommit(ctx, commitsHistory, ctx.Repo.Repository)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.ServerError("ConvertFromGitCommit", err)
		return nil, nil
	}

	pager := context.NewPagination(int(commitsCount), setting.Git.CommitsRangeSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	return wikiRepo, entry
}

func renderEditPage(ctx *context.Context) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	defer func() {
		if wikiRepo != nil {
			_ = wikiRepo.Close()
		}
	}()
	if err != nil {
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	// lookup filename in wiki -  gitTree entry , real filename
	entry, _, noEntry, isRaw := wikiEntryByName(ctx, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if isRaw {
		ctx.HTTPError(http.StatusForbidden, "Editing of raw wiki files is not allowed")
	}
	if entry == nil || ctx.Written() {
		return
	}

	// get filecontent
	data := wikiContentsByEntry(ctx, entry)
	if ctx.Written() {
		return
	}

	ctx.Data["content"] = string(data)
	ctx.Data["sidebarPresent"] = false
	ctx.Data["sidebarContent"] = ""
	ctx.Data["footerPresent"] = false
	ctx.Data["footerContent"] = ""
}

// WikiPost renders post of wiki page
func WikiPost(ctx *context.Context) {
	switch ctx.FormString("action") {
	case "_new":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWikiPost(ctx)
		return
	case "_delete":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		DeleteWikiPagePost(ctx)
		return
	}

	if !ctx.Repo.CanWrite(unit.TypeWiki) {
		ctx.NotFound(nil)
		return
	}
	EditWikiPost(ctx)
}

// Wiki renders single wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	switch ctx.FormString("action") {
	case "_pages":
		WikiPages(ctx)
		return
	case "_revision":
		WikiRevision(ctx)
		return
	case "_edit":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		EditWiki(ctx)
		return
	case "_new":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(nil)
			return
		}
		NewWiki(ctx)
		return
	}

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiRepo, entry := renderViewPage(ctx)
	defer func() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
	}()
	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiPath := entry.Name()
	if markup.DetectMarkupTypeByFileName(wikiPath) != markdown.MarkupName {
		ext := strings.ToUpper(filepath.Ext(wikiPath))
		ctx.Data["FormatWarning"] = fmt.Sprintf("%s rendering is not supported at the moment. Rendered as Markdown.", ext)
	}
	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(wikiPath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}
	ctx.Data["Author"] = lastCommit.Author

	ctx.HTML(http.StatusOK, tplWikiView)
}

// WikiRevision renders file revision list of wiki page
func WikiRevision(ctx *context.Context) {
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	wikiRepo, entry := renderRevisionPage(ctx)
	defer func() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
	}()

	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(http.StatusOK, tplWikiStart)
		return
	}

	// Get last change information.
	wikiPath := entry.Name()
	lastCommit, err := wikiRepo.GetCommitByPath(wikiPath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}
	ctx.Data["Author"] = lastCommit.Author

	ctx.HTML(http.StatusOK, tplWikiRevision)
}

// WikiPages render wiki pages list page
func WikiPages(ctx *context.Context) {
	if !ctx.Repo.Repository.HasWiki() {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	ctx.Data["Title"] = ctx.Tr("repo.wiki.pages")
	ctx.Data["CanWriteWiki"] = ctx.Repo.CanWrite(unit.TypeWiki) && !ctx.Repo.Repository.IsArchived

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	defer func() {
		if wikiRepo != nil {
			_ = wikiRepo.Close()
		}
	}()
	if err != nil {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	treePath := "" // To support list sub folders' pages in the future
	tree, err := commit.SubTree(treePath)
	if err != nil {
		ctx.ServerError("SubTree", err)
		return
	}

	allEntries, err := tree.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	allEntries.CustomSort(base.NaturalSortLess)

	entries, _, err := allEntries.GetCommitsInfo(gocontext.Context(ctx), commit, treePath)
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return
	}

	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.Entry.IsRegular() {
			continue
		}
		wikiName, err := wiki_service.GitPathToWebPath(entry.Entry.Name())
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
			SubURL:       wiki_service.WebPathToURLPath(wikiName),
			GitEntryName: entry.Entry.Name(),
			UpdatedUnix:  timeutil.TimeStamp(entry.Commit.Author.When.Unix()),
		})
	}
	ctx.Data["Pages"] = pages

	ctx.HTML(http.StatusOK, tplWikiPages)
}

// WikiRaw outputs raw blob requested by user (image for example)
func WikiRaw(ctx *context.Context) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	defer func() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
	}()

	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(nil)
			return
		}
		ctx.ServerError("findEntryForfile", err)
		return
	}

	providedWebPath := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	providedGitPath := wiki_service.WebPathToGitPath(providedWebPath)
	var entry *git.TreeEntry
	if commit != nil {
		// Try to find a file with that name
		entry, err = findEntryForFile(commit, providedGitPath)
		if err != nil && !git.IsErrNotExist(err) {
			ctx.ServerError("findFile", err)
			return
		}

		if entry == nil {
			// Try to find a wiki page with that name
			providedGitPath = strings.TrimSuffix(providedGitPath, ".md")
			entry, err = findEntryForFile(commit, providedGitPath)
			if err != nil && !git.IsErrNotExist(err) {
				ctx.ServerError("findFile", err)
				return
			}
		}
	}

	if entry != nil {
		if err = common.ServeBlob(ctx.Base, ctx.Repo.Repository, ctx.Repo.TreePath, entry.Blob(), nil); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	}

	ctx.NotFound(nil)
}

// NewWiki render wiki create page
func NewWiki(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["title"] = "Home"
	}
	if ctx.FormString("title") != "" {
		ctx.Data["title"] = ctx.FormString("title")
	}

	ctx.HTML(http.StatusOK, tplWikiNew)
}

// NewWikiPost response for wiki create request
func NewWikiPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewWikiForm)
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplWikiNew)
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.RenderWithErr(ctx.Tr("repo.issues.new.title_empty"), tplWikiNew, form)
		return
	}

	wikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Locale.TrString("repo.editor.add", form.Title)
	}

	if err := wiki_service.AddWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName, form.Content, form.Message); err != nil {
		if repo_model.IsErrWikiReservedName(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.reserved_page", wikiName), tplWikiNew, &form)
		} else if repo_model.IsErrWikiAlreadyExist(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.page_already_exists"), tplWikiNew, &form)
		} else {
			ctx.ServerError("AddWikiPage", err)
		}
		return
	}

	notify_service.NewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(wikiName))
}

// EditWiki render wiki modify page
func EditWiki(ctx *context.Context) {
	ctx.Data["PageIsWikiEdit"] = true

	if !ctx.Repo.Repository.HasWiki() {
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
	form := web.GetForm(ctx).(*forms.NewWikiForm)
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplWikiNew)
		return
	}

	oldWikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	newWikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Locale.TrString("repo.editor.update", form.Title)
	}

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		ctx.ServerError("EditWikiPage", err)
		return
	}

	notify_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(newWikiName))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	wikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw("*"))
	if len(wikiName) == 0 {
		wikiName = "Home"
	}

	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		ctx.ServerError("DeleteWikiPage", err)
		return
	}

	notify_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.JSONRedirect(ctx.Repo.RepoLink + "/wiki/")
}
