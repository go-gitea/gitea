// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"io/ioutil"
	"strings"
	"time"

	"code.gitea.io/git"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markdown"
)

const (
	tplWikiStart base.TplName = "repo/wiki/start"
	tplWikiView  base.TplName = "repo/wiki/view"
	tplWikiNew   base.TplName = "repo/wiki/new"
	tplWikiPages base.TplName = "repo/wiki/pages"
)

// MustEnableWiki check if wiki is enabled, if external then redirect
func MustEnableWiki(ctx *context.Context) {
	if !ctx.Repo.Repository.EnableWiki {
		ctx.Handle(404, "MustEnableWiki", nil)
		return
	}

	if ctx.Repo.Repository.EnableExternalWiki {
		ctx.Redirect(ctx.Repo.Repository.ExternalWikiURL)
		return
	}
}

// PageMeta wiki page meat information
type PageMeta struct {
	Name    string
	URL     string
	Updated time.Time
}

func renderWikiPage(ctx *context.Context, isViewPage bool) (*git.Repository, string) {
	wikiRepo, err := git.OpenRepository(ctx.Repo.Repository.WikiPath())
	if err != nil {
		ctx.Handle(500, "OpenRepository", err)
		return nil, ""
	}
	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		ctx.Handle(500, "GetBranchCommit", err)
		return nil, ""
	}

	// Get page list.
	if isViewPage {
		entries, err := commit.ListEntries()
		if err != nil {
			ctx.Handle(500, "ListEntries", err)
			return nil, ""
		}
		pages := make([]PageMeta, 0, len(entries))
		for i := range entries {
			if entries[i].Type == git.ObjectBlob && strings.HasSuffix(entries[i].Name(), ".md") {
				name := strings.TrimSuffix(entries[i].Name(), ".md")
				pages = append(pages, PageMeta{
					Name: name,
					URL:  models.ToWikiPageURL(name),
				})
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

	blob, err := commit.GetBlobByPath(pageName + ".md")
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.Redirect(ctx.Repo.RepoLink + "/wiki/_pages")
		} else {
			ctx.Handle(500, "GetBlobByPath", err)
		}
		return nil, ""
	}
	r, err := blob.Data()
	if err != nil {
		ctx.Handle(500, "Data", err)
		return nil, ""
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		ctx.Handle(500, "ReadAll", err)
		return nil, ""
	}
	if isViewPage {
		ctx.Data["content"] = string(markdown.Render(data, ctx.Repo.RepoLink, ctx.Repo.Repository.ComposeMetas()))
	} else {
		ctx.Data["content"] = string(data)
	}

	return wikiRepo, pageName
}

// Wiki render wiki page
func Wiki(ctx *context.Context) {
	ctx.Data["PageIsWiki"] = true

	if !ctx.Repo.Repository.HasWiki() {
		ctx.Data["Title"] = ctx.Tr("repo.wiki")
		ctx.HTML(200, tplWikiStart)
		return
	}

	wikiRepo, pageName := renderWikiPage(ctx, true)
	if ctx.Written() {
		return
	}

	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(pageName + ".md")
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

	wikiRepo, err := git.OpenRepository(ctx.Repo.Repository.WikiPath())
	if err != nil {
		ctx.Handle(500, "OpenRepository", err)
		return
	}
	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		ctx.Handle(500, "GetBranchCommit", err)
		return
	}

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.Handle(500, "ListEntries", err)
		return
	}
	pages := make([]PageMeta, 0, len(entries))
	for i := range entries {
		if entries[i].Type == git.ObjectBlob && strings.HasSuffix(entries[i].Name(), ".md") {
			c, err := wikiRepo.GetCommitByPath(entries[i].Name())
			if err != nil {
				ctx.Handle(500, "GetCommit", err)
				return
			}
			name := strings.TrimSuffix(entries[i].Name(), ".md")
			pages = append(pages, PageMeta{
				Name:    name,
				URL:     models.ToWikiPageURL(name),
				Updated: c.Author.When,
			})
		}
	}
	ctx.Data["Pages"] = pages

	ctx.HTML(200, tplWikiPages)
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

	if err := ctx.Repo.Repository.AddWikiPage(ctx.User, form.Title, form.Content, form.Message); err != nil {
		if models.IsErrWikiAlreadyExist(err) {
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.wiki.page_already_exists"), tplWikiNew, &form)
		} else {
			ctx.Handle(500, "AddWikiPage", err)
		}
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + models.ToWikiPageURL(form.Title))
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

	if err := ctx.Repo.Repository.EditWikiPage(ctx.User, form.OldTitle, form.Title, form.Content, form.Message); err != nil {
		ctx.Handle(500, "EditWikiPage", err)
		return
	}

	ctx.Redirect(ctx.Repo.RepoLink + "/wiki/" + models.ToWikiPageURL(form.Title))
}

// DeleteWikiPagePost delete wiki page
func DeleteWikiPagePost(ctx *context.Context) {
	pageURL := ctx.Params(":page")
	if len(pageURL) == 0 {
		pageURL = "Home"
	}

	pageName := models.ToWikiPageName(pageURL)
	if err := ctx.Repo.Repository.DeleteWikiPage(ctx.User, pageName); err != nil {
		ctx.Handle(500, "DeleteWikiPage", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/wiki/",
	})
}
