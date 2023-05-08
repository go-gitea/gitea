// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/forms"
	wiki_service "code.gitea.io/gitea/services/wiki"
)

const (
	tplWikiStart    base.TplName = "repo/wiki/start"
	tplWikiView     base.TplName = "repo/wiki/view"
	tplWikiRevision base.TplName = "repo/wiki/revision"
	tplWikiNew      base.TplName = "repo/wiki/new"
	tplWikiPages    base.TplName = "repo/wiki/pages"
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
		ctx.NotFound("MustEnableWiki", nil)
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
	wikiRepo, err := git.OpenRepository(ctx, ctx.Repo.Repository.WikiPath())
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return nil, nil, err
	}

	commit, err := wikiRepo.GetBranchCommit(wiki_service.DefaultBranch)
	if err != nil {
		return wikiRepo, nil, err
	}
	return wikiRepo, commit, nil
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

// wikiContentsByName returns the contents of a wiki page, along with a boolean
// indicating whether the page exists. Writes to ctx if an error occurs.
func wikiContentsByName(ctx *context.Context, commit *git.Commit, wikiName wiki_service.WebPath) ([]byte, *git.TreeEntry, string, bool) {
	gitFilename := wiki_service.WebPathToGitPath(wikiName)
	entry, err := findEntryForFile(commit, gitFilename)
	if err != nil && !git.IsErrNotExist(err) {
		ctx.ServerError("findEntryForFile", err)
		return nil, nil, "", false
	} else if entry == nil {
		return nil, nil, "", true
	}
	return wikiContentsByEntry(ctx, entry), entry, gitFilename, false
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
	pageName := wiki_service.WebPathFromRequest(ctx.Params("*"))
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

	rctx := &markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeDocumentMetas(),
		IsWiki:    true,
	}
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

	if rctx.SidebarTocNode != nil {
		sb := &strings.Builder{}
		err = markdown.SpecializedMarkdown().Renderer().Render(sb, nil, rctx.SidebarTocNode)
		if err != nil {
			log.Error("Failed to render wiki sidebar TOC: %v", err)
		} else {
			ctx.Data["sidebarTocContent"] = sb.String()
		}
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount(wiki_service.DefaultBranch, pageFilename)
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
	pageName := wiki_service.WebPathFromRequest(ctx.Params("*"))
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
	commitsCount, _ := wikiRepo.FileCommitsCount(wiki_service.DefaultBranch, pageFilename)
	ctx.Data["CommitCount"] = commitsCount

	// get page
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	// get Commit Count
	commitsHistory, err := wikiRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: wiki_service.DefaultBranch,
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
	ctx.Data["Commits"] = git_model.ConvertFromGitCommit(ctx, commitsHistory, ctx.Repo.Repository)

	pager := context.NewPagination(int(commitsCount), setting.Git.CommitsRangeSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	return wikiRepo, entry
}

func renderEditPage(ctx *context.Context) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		if !git.IsErrNotExist(err) {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}
	defer func() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
	}()

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.Params("*"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	_, displayName := wiki_service.WebPathToUserTitle(pageName)
	ctx.Data["PageURL"] = wiki_service.WebPathToURLPath(pageName)
	ctx.Data["old_title"] = displayName
	ctx.Data["Title"] = displayName
	ctx.Data["title"] = displayName

	// lookup filename in wiki - get filecontent, gitTree entry , real filename
	data, entry, _, noEntry := wikiContentsByName(ctx, commit, pageName)
	if noEntry {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/?action=_pages")
	}
	if entry == nil || ctx.Written() {
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
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
		NewWikiPost(ctx)
		return
	case "_delete":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
		DeleteWikiPagePost(ctx)
		return
	}

	if !ctx.Repo.CanWrite(unit.TypeWiki) {
		ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
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
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
			return
		}
		EditWiki(ctx)
		return
	case "_new":
		if !ctx.Repo.CanWrite(unit.TypeWiki) {
			ctx.NotFound(ctx.Req.URL.RequestURI(), nil)
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
	if markup.Type(wikiPath) != markdown.MarkupName {
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
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		return
	}
	defer func() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
	}()

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	pages := make([]PageMeta, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsRegular() {
			continue
		}
		c, err := wikiRepo.GetCommitByPath(entry.Name())
		if err != nil {
			ctx.ServerError("GetCommit", err)
			return
		}
		wikiName, err := wiki_service.GitPathToWebPath(entry.Name())
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
			GitEntryName: entry.Name(),
			UpdatedUnix:  timeutil.TimeStamp(c.Author.When.Unix()),
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
			ctx.NotFound("findEntryForFile", nil)
			return
		}
		ctx.ServerError("findEntryForfile", err)
		return
	}

	providedWebPath := wiki_service.WebPathFromRequest(ctx.Params("*"))
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
		if err = common.ServeBlob(ctx, entry.Blob(), time.Time{}); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	}

	ctx.NotFound("findEntryForFile", nil)
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
		form.Message = ctx.Tr("repo.editor.add", form.Title)
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

	notification.NotifyNewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)

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

	oldWikiName := wiki_service.WebPathFromRequest(ctx.Params("*"))
	newWikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Tr("repo.editor.update", form.Title)
	}

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		ctx.ServerError("EditWikiPage", err)
		return
	}

	notification.NotifyEditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wiki_service.WebPathToURLPath(newWikiName))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	wikiName := wiki_service.WebPathFromRequest(ctx.Params("*"))
	if len(wikiName) == 0 {
		wikiName = "Home"
	}

	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		ctx.ServerError("DeleteWikiPage", err)
		return
	}

	notification.NotifyDeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/wiki/",
	})
}
