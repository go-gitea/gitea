// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/git"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/markup"
)

const (
	tplWikiStart base.TplName = "repo/wiki/start"
	tplWikiView  base.TplName = "repo/wiki/view"
	tplWikiNew   base.TplName = "repo/wiki/new"
	tplWikiPages base.TplName = "repo/wiki/pages"
)

// MustEnableWiki check if wiki is enabled, if external then redirect
func MustEnableWiki(ctx *context.Context) {
	if !ctx.Repo.Repository.UnitEnabled(models.UnitTypeWiki) &&
		!ctx.Repo.Repository.UnitEnabled(models.UnitTypeExternalWiki) {
		ctx.Handle(404, "MustEnableWiki", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalWiki)
	if err == nil {
		ctx.Redirect(unit.ExternalWikiConfig().ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meat information
type PageMeta struct {
	Name    string
	URL     string
	Updated time.Time
}

func urlEncoded(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		return str
	}
	return u.String()
}
func urlDecoded(str string) string {
	res, err := url.QueryUnescape(str)
	if err != nil {
		return str
	}
	return res
}

// commitTreeBlobEntry processes found file and checks if it matches search target
func commitTreeBlobEntry(entry *git.TreeEntry, path string, targets []string, textOnly bool) *git.TreeEntry {
	name := entry.Name()
	ext := filepath.Ext(name)
	if !textOnly || markdown.IsMarkdownFile(name) || ext == ".textile" {
		for _, target := range targets {
			if matchName(path, target) || matchName(urlEncoded(path), target) || matchName(urlDecoded(path), target) {
				return entry
			}
			pathNoExt := strings.TrimSuffix(path, ext)
			if matchName(pathNoExt, target) || matchName(urlEncoded(pathNoExt), target) || matchName(urlDecoded(pathNoExt), target) {
				return entry
			}
		}
	}
	return nil
}

// commitTreeDirEntry is a recursive file tree traversal function
func commitTreeDirEntry(repo *git.Repository, commit *git.Commit, entries []*git.TreeEntry, prevPath string, targets []string, textOnly bool) (*git.TreeEntry, error) {
	for i := range entries {
		entry := entries[i]
		var path string
		if len(prevPath) == 0 {
			path = entry.Name()
		} else {
			path = prevPath + "/" + entry.Name()
		}
		if entry.Type == git.ObjectBlob {
			// File
			if res := commitTreeBlobEntry(entry, path, targets, textOnly); res != nil {
				return res, nil
			}
		} else if entry.IsDir() {
			// Directory
			// Get our tree entry, handling all possible errors
			var err error
			var tree *git.Tree
			if tree, err = repo.GetTree(entry.ID.String()); tree == nil || err != nil {
				if err == nil {
					err = fmt.Errorf("repo.GetTree(%s) => nil", entry.ID.String())
				}
				return nil, err
			}
			// Found us, get children entries
			var ls git.Entries
			if ls, err = tree.ListEntries(); err != nil {
				return nil, err
			}
			// Call itself recursively to find needed entry
			var te *git.TreeEntry
			if te, err = commitTreeDirEntry(repo, commit, ls, path, targets, textOnly); err != nil {
				return nil, err
			}
			if te != nil {
				return te, nil
			}
		}
	}
	return nil, nil
}

// commitTreeEntry is a first step of commitTreeDirEntry, which should be never called directly
func commitTreeEntry(repo *git.Repository, commit *git.Commit, targets []string, textOnly bool) (*git.TreeEntry, error) {
	entries, err := commit.ListEntries()
	if err != nil {
		return nil, err
	}
	return commitTreeDirEntry(repo, commit, entries, "", targets, textOnly)
}

// findFile finds the best match for given filename in repo file tree
func findFile(repo *git.Repository, commit *git.Commit, target string, textOnly bool) (*git.TreeEntry, error) {
	targets := []string{target, urlEncoded(target), urlDecoded(target)}
	var entry *git.TreeEntry
	var err error
	if entry, err = commitTreeEntry(repo, commit, targets, textOnly); err != nil {
		return nil, err
	}
	return entry, nil
}

// matchName matches generic name representation of the file with required one
func matchName(target, name string) bool {
	if len(target) != len(name) {
		return false
	}
	name = strings.ToLower(name)
	target = strings.ToLower(target)
	if name == target {
		return true
	}
	target = strings.Replace(target, " ", "?", -1)
	target = strings.Replace(target, "-", "?", -1)
	for i := range name {
		ch := name[i]
		reqCh := target[i]
		if ch != reqCh {
			if string(reqCh) != "?" {
				return false
			}
		}
	}
	return true
}

func findWikiRepoCommit(ctx *context.Context) (*git.Repository, *git.Commit, error) {
	wikiRepo, err := git.OpenRepository(ctx.Repo.Repository.WikiPath())
	if err != nil {
		// ctx.Handle(500, "OpenRepository", err)
		return nil, nil, err
	}
	if !wikiRepo.IsBranchExist("master") {
		return wikiRepo, nil, nil
	}

	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		ctx.Handle(500, "GetBranchCommit", err)
		return wikiRepo, nil, err
	}
	return wikiRepo, commit, nil
}

func renderWikiPage(ctx *context.Context, isViewPage bool) (*git.Repository, *git.TreeEntry) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		return nil, nil
	}
	if commit == nil {
		return wikiRepo, nil
	}

	// Get page list.
	if isViewPage {
		entries, err := commit.ListEntries()
		if err != nil {
			ctx.Handle(500, "ListEntries", err)
			return nil, nil
		}
		pages := []PageMeta{}
		for i := range entries {
			if entries[i].Type == git.ObjectBlob {
				name := entries[i].Name()
				ext := filepath.Ext(name)
				if markdown.IsMarkdownFile(name) || ext == ".textile" {
					name = strings.TrimSuffix(name, ext)
					if name == "" || name == "_Sidebar" || name == "_Footer" || name == "_Header" {
						continue
					}
					pages = append(pages, PageMeta{
						Name: models.ToWikiPageName(name),
						URL:  name,
					})
				}
			}
		}
		ctx.Data["Pages"] = pages
	}

	pageURL := ctx.Params(":page")
	if len(pageURL) == 0 {
		pageURL = "Home"
	}
	ctx.Data["PageURL"] = pageURL

	pageName := models.ToWikiPageName(pageURL)
	ctx.Data["old_title"] = pageName
	ctx.Data["Title"] = pageName
	ctx.Data["title"] = pageName
	ctx.Data["RequireHighlightJS"] = true

	var entry *git.TreeEntry
	if entry, err = findFile(wikiRepo, commit, pageName, true); err != nil {
		ctx.Handle(500, "findFile", err)
		return nil, nil
	}
	if entry == nil {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki/_pages")
		return nil, nil
	}
	blob := entry.Blob()
	r, err := blob.Data()
	if err != nil {
		ctx.Handle(500, "Data", err)
		return nil, nil
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		ctx.Handle(500, "ReadAll", err)
		return nil, nil
	}
	sidebarPresent := false
	sidebarContent := []byte{}
	sentry, err := findFile(wikiRepo, commit, "_Sidebar", true)
	if err == nil && sentry != nil {
		r, err = sentry.Blob().Data()
		if err == nil {
			dataSB, err := ioutil.ReadAll(r)
			if err == nil {
				sidebarPresent = true
				sidebarContent = dataSB
			}
		}
	}
	footerPresent := false
	footerContent := []byte{}
	sentry, err = findFile(wikiRepo, commit, "_Footer", true)
	if err == nil && sentry != nil {
		r, err = sentry.Blob().Data()
		if err == nil {
			dataSB, err := ioutil.ReadAll(r)
			if err == nil {
				footerPresent = true
				footerContent = dataSB
			}
		}
	}
	if isViewPage {
		metas := ctx.Repo.Repository.ComposeMetas()
		ctx.Data["content"] = markdown.RenderWiki(data, ctx.Repo.RepoLink, metas)
		ctx.Data["sidebarPresent"] = sidebarPresent
		ctx.Data["sidebarContent"] = markdown.RenderWiki(sidebarContent, ctx.Repo.RepoLink, metas)
		ctx.Data["footerPresent"] = footerPresent
		ctx.Data["footerContent"] = markdown.RenderWiki(footerContent, ctx.Repo.RepoLink, metas)
	} else {
		ctx.Data["content"] = string(data)
		ctx.Data["sidebarPresent"] = false
		ctx.Data["sidebarContent"] = ""
		ctx.Data["footerPresent"] = false
		ctx.Data["footerContent"] = ""
	}

	return wikiRepo, entry
}

// Wiki renders single wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["PageIsWiki"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(200, tplWikiStart)
		return
	}

	wikiRepo, entry := renderWikiPage(ctx, true)
	if ctx.Written() {
		return
	}
	if entry == nil {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(200, tplWikiStart)
		return
	}

	ename := entry.Name()
	if markup.Type(ename) != markdown.MarkupName {
		ext := strings.ToUpper(filepath.Ext(ename))
		ctx.Data["FormatWarning"] = fmt.Sprintf("%s rendering is not supported at the moment. Rendered as Markdown.", ext)
	}
	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(ename)
	if err != nil {
		ctx.Handle(500, "GetCommitByPath", err)
		return
	}
	ctx.Data["Author"] = lastCommit.Author

	ctx.HTML(200, tplWikiView)
}

// WikiPages render wiki pages list page
func WikiPages(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.pages")
	ctx.Data["PageIsWiki"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		return
	}

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.Handle(500, "ListEntries", err)
		return
	}
	pages := make([]PageMeta, 0, len(entries))
	for i := range entries {
		if entries[i].Type == git.ObjectBlob {
			c, err := wikiRepo.GetCommitByPath(entries[i].Name())
			if err != nil {
				ctx.Handle(500, "GetCommit", err)
				return
			}
			name := entries[i].Name()
			ext := filepath.Ext(name)
			if markdown.IsMarkdownFile(name) || ext == ".textile" {
				name = strings.TrimSuffix(name, ext)
				if name == "" {
					continue
				}
				pages = append(pages, PageMeta{
					Name:    models.ToWikiPageName(name),
					URL:     name,
					Updated: c.Author.When,
				})
			}
		}
	}
	ctx.Data["Pages"] = pages

	ctx.HTML(200, tplWikiPages)
}

// WikiRaw outputs raw blob requested by user (image for example)
func WikiRaw(ctx *context.Context) {
	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			return
		}
	}
	uri := ctx.Params("*")
	var entry *git.TreeEntry
	if commit != nil {
		entry, err = findFile(wikiRepo, commit, uri, false)
	}
	if err != nil || entry == nil {
		if entry == nil || commit == nil {
			defBranch := ctx.Repo.Repository.DefaultBranch
			if commit, err = ctx.Repo.GitRepo.GetBranchCommit(defBranch); commit == nil || err != nil {
				ctx.Handle(500, "GetBranchCommit", err)
				return
			}
			if entry, err = findFile(ctx.Repo.GitRepo, commit, uri, false); err != nil {
				ctx.Handle(500, "findFile", err)
				return
			}
			if entry == nil {
				ctx.Handle(404, "findFile", nil)
				return
			}
		} else {
			ctx.Handle(500, "findFile", err)
			return
		}
	}
	if err = ServeBlob(ctx, entry.Blob()); err != nil {
		ctx.Handle(500, "ServeBlob", err)
	}
}

// NewWiki render wiki create page
func NewWiki(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["title"] = "Home"
	}

	ctx.HTML(200, tplWikiNew)
}

// NewWikiPost response fro wiki create request
func NewWikiPost(ctx *context.Context, form auth.NewWikiForm) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplWikiNew)
		return
	}

	wikiPath := models.ToWikiPageURL(form.Title)

	if err := ctx.Repo.Repository.AddWikiPage(ctx.User, wikiPath, form.Content, form.Message); err != nil {
		if models.IsErrWikiAlreadyExist(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.page_already_exists"), tplWikiNew, &form)
		} else {
			ctx.Handle(500, "AddWikiPage", err)
		}
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + wikiPath)
}

// EditWiki render wiki modify page
func EditWiki(ctx *context.Context) {
	ctx.Data["PageIsWiki"] = true
	ctx.Data["PageIsWikiEdit"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Redirect(ctx.Repo.RepoLink + "/wiki")
		return
	}

	renderWikiPage(ctx, false)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplWikiNew)
}

// EditWikiPost response fro wiki modify request
func EditWikiPost(ctx *context.Context, form auth.NewWikiForm) {
	ctx.Data["Title"] = ctx.Tr("repo.wiki.new_page")
	ctx.Data["PageIsWiki"] = true
	ctx.Data["RequireSimpleMDE"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplWikiNew)
		return
	}

	oldWikiPath := models.ToWikiPageURL(ctx.Params(":page"))
	newWikiPath := models.ToWikiPageURL(form.Title)

	if err := ctx.Repo.Repository.EditWikiPage(ctx.User, oldWikiPath, newWikiPath, form.Content, form.Message); err != nil {
		ctx.Handle(500, "EditWikiPage", err)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + newWikiPath)
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	pageURL := models.ToWikiPageURL(ctx.Params(":page"))
	if len(pageURL) == 0 {
		pageURL = "Home"
	}

	if err := ctx.Repo.Repository.DeleteWikiPage(ctx.User, pageURL); err != nil {
		ctx.Handle(500, "DeleteWikiPage", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/wiki/",
	})
}
