// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	gotemplate "html/template"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"github.com/Unknwon/paginater"
)

const (
	tplRepoBARE base.TplName = "repo/bare"
	tplRepoHome base.TplName = "repo/home"
	tplWatchers base.TplName = "repo/watchers"
	tplForks    base.TplName = "repo/forks"
)

func renderDirectory(ctx *context.Context, treeLink string) {
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.SubTree", git.IsErrNotExist, err)
		return
	}

	entries, err := tree.ListEntries()
	if err != nil {
		ctx.Handle(500, "ListEntries", err)
		return
	}
	entries.Sort()

	ctx.Data["Files"], err = entries.GetCommitsInfo(ctx.Repo.Commit, ctx.Repo.TreePath)
	if err != nil {
		ctx.Handle(500, "GetCommitsInfo", err)
		return
	}

	var readmeFile *git.Blob
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		tp, ok := markup.ReadmeFileType(entry.Name())
		if !ok {
			continue
		}

		readmeFile = entry.Blob()
		if tp != "" {
			break
		}
	}

	if readmeFile != nil {
		ctx.Data["RawFileLink"] = ""
		ctx.Data["ReadmeInList"] = true
		ctx.Data["ReadmeExist"] = true

		dataRc, err := readmeFile.Data()
		if err != nil {
			ctx.Handle(500, "Data", err)
			return
		}

		buf := make([]byte, 1024)
		n, _ := dataRc.Read(buf)
		buf = buf[:n]

		isTextFile := base.IsTextFile(buf)
		ctx.Data["FileIsText"] = isTextFile
		ctx.Data["FileName"] = readmeFile.Name()
		// FIXME: what happens when README file is an image?
		if isTextFile {
			d, _ := ioutil.ReadAll(dataRc)
			buf = append(buf, d...)
			newbuf := markup.Render(readmeFile.Name(), buf, treeLink, ctx.Repo.Repository.ComposeMetas())
			if newbuf != nil {
				ctx.Data["IsMarkdown"] = true
			} else {
				// FIXME This is the only way to show non-markdown files
				// instead of a broken "View Raw" link
				ctx.Data["IsMarkdown"] = true
				newbuf = bytes.Replace(buf, []byte("\n"), []byte(`<br>`), -1)
			}
			ctx.Data["FileContent"] = string(newbuf)
		}
	}

	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	latestCommit := ctx.Repo.Commit
	if len(ctx.Repo.TreePath) > 0 {
		latestCommit, err = ctx.Repo.Commit.GetCommitByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.Handle(500, "GetCommitByPath", err)
			return
		}
	}
	ctx.Data["LatestCommit"] = latestCommit
	ctx.Data["LatestCommitVerification"] = models.ParseCommitWithSignature(latestCommit)
	ctx.Data["LatestCommitUser"] = models.ValidateCommitWithEmail(latestCommit)

	// Check permission to add or upload new file.
	if ctx.Repo.IsWriter() && ctx.Repo.IsViewBranch {
		ctx.Data["CanAddFile"] = true
		ctx.Data["CanUploadFile"] = setting.Repository.Upload.Enabled
	}
}

func renderFile(ctx *context.Context, entry *git.TreeEntry, treeLink, rawLink string) {
	ctx.Data["IsViewFile"] = true

	blob := entry.Blob()
	dataRc, err := blob.Data()
	if err != nil {
		ctx.Handle(500, "Data", err)
		return
	}

	ctx.Data["FileSize"] = blob.Size()
	ctx.Data["FileName"] = blob.Name()
	ctx.Data["HighlightClass"] = highlight.FileNameToHighlightClass(blob.Name())
	ctx.Data["RawFileLink"] = rawLink + "/" + ctx.Repo.TreePath

	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]

	isTextFile := base.IsTextFile(buf)
	ctx.Data["IsTextFile"] = isTextFile

	//Check for LFS meta file
	if isTextFile && setting.LFS.StartServer {
		headString := string(buf)
		if strings.HasPrefix(headString, models.LFSMetaFileIdentifier) {
			splitLines := strings.Split(headString, "\n")
			if len(splitLines) >= 3 {
				oid := strings.TrimPrefix(splitLines[1], models.LFSMetaFileOidPrefix)
				size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
				if len(oid) == 64 && err == nil {
					contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
					meta := &models.LFSMetaObject{Oid: oid}
					if contentStore.Exists(meta) {
						ctx.Data["IsTextFile"] = false
						isTextFile = false
						ctx.Data["IsLFSFile"] = true
						ctx.Data["FileSize"] = size
						filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(blob.Name()))
						ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s/info/lfs/objects/%s/%s", setting.AppURL, ctx.Repo.Repository.FullName(), oid, filenameBase64)
					}
				}
			}
		}
	}

	// Assume file is not editable first.
	if !isTextFile {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_non_text_files")
	}

	switch {
	case isTextFile:
		if blob.Size() >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = append(buf, d...)

		tp := markup.Type(blob.Name())
		isSupportedMarkup := tp != ""
		// FIXME: currently set IsMarkdown for compatible
		ctx.Data["IsMarkdown"] = isSupportedMarkup

		readmeExist := isSupportedMarkup || markup.IsReadmeFile(blob.Name())
		ctx.Data["ReadmeExist"] = readmeExist
		if readmeExist && isSupportedMarkup {
			ctx.Data["FileContent"] = string(markup.Render(blob.Name(), buf, path.Dir(treeLink), ctx.Repo.Repository.ComposeMetas()))
		} else {
			// Building code view blocks with line number on server side.
			var fileContent string
			if content, err := templates.ToUTF8WithErr(buf); err != nil {
				if err != nil {
					log.Error(4, "ToUTF8WithErr: %v", err)
				}
				fileContent = string(buf)
			} else {
				fileContent = content
			}

			var output bytes.Buffer
			lines := strings.Split(fileContent, "\n")
			for index, line := range lines {
				line = gotemplate.HTMLEscapeString(line)
				if index != len(lines)-1 {
					line += "\n"
				}
				output.WriteString(fmt.Sprintf(`<li class="L%d" rel="L%d">%s</li>`, index+1, index+1, line))
			}
			ctx.Data["FileContent"] = gotemplate.HTML(output.String())

			output.Reset()
			for i := 0; i < len(lines); i++ {
				output.WriteString(fmt.Sprintf(`<span id="L%d">%d</span>`, i+1, i+1))
			}
			ctx.Data["LineNums"] = gotemplate.HTML(output.String())
		}

		if ctx.Repo.CanEnableEditor() {
			ctx.Data["CanEditFile"] = true
			ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.edit_this_file")
		} else if !ctx.Repo.IsViewBranch {
			ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
		} else if !ctx.Repo.IsWriter() {
			ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.fork_before_edit")
		}

	case base.IsPDFFile(buf):
		ctx.Data["IsPDFFile"] = true
	case base.IsVideoFile(buf):
		ctx.Data["IsVideoFile"] = true
	case base.IsImageFile(buf):
		ctx.Data["IsImageFile"] = true
	}

	if ctx.Repo.CanEnableEditor() {
		ctx.Data["CanDeleteFile"] = true
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.delete_this_file")
	} else if !ctx.Repo.IsViewBranch {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
	} else if !ctx.Repo.IsWriter() {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
	}
}

// Home render repository home page
func Home(ctx *context.Context) {
	if len(ctx.Repo.Repository.Units) > 0 {
		tp := ctx.Repo.Repository.Units[0].Type
		if tp == models.UnitTypeCode {
			renderCode(ctx)
			return
		}

		unit, ok := models.Units[tp]
		if ok {
			ctx.Redirect(setting.AppSubURL + fmt.Sprintf("/%s%s",
				ctx.Repo.Repository.FullName(), unit.URI))
			return
		}
	}

	ctx.Handle(404, "Home", fmt.Errorf(ctx.Tr("units.error.no_unit_allowed_repo")))
}

func renderCode(ctx *context.Context) {
	ctx.Data["PageIsViewCode"] = true

	if ctx.Repo.Repository.IsBare {
		ctx.HTML(200, tplRepoBARE)
		return
	}

	title := ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	if len(ctx.Repo.Repository.Description) > 0 {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title
	ctx.Data["RequireHighlightJS"] = true

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchName
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchName

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	if entry.IsDir() {
		renderDirectory(ctx, treeLink)
	} else {
		renderFile(ctx, entry, treeLink, rawLink)
	}
	if ctx.Written() {
		return
	}

	var treeNames []string
	paths := make([]string, 0, 5)
	if len(ctx.Repo.TreePath) > 0 {
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
	ctx.HTML(200, tplRepoHome)
}

// RenderUserCards render a page show users according the input templaet
func RenderUserCards(ctx *context.Context, total int, getter func(page int) ([]*models.User, error), tpl base.TplName) {
	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}
	pager := paginater.New(total, models.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	items, err := getter(pager.Current())
	if err != nil {
		ctx.Handle(500, "getter", err)
		return
	}
	ctx.Data["Cards"] = items

	ctx.HTML(200, tpl)
}

// Watchers render repository's watch users
func Watchers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.watchers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.watchers")
	ctx.Data["PageIsWatchers"] = true
	RenderUserCards(ctx, ctx.Repo.Repository.NumWatches, ctx.Repo.Repository.GetWatchers, tplWatchers)
}

// Stars render repository's starred users
func Stars(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.stargazers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.stargazers")
	ctx.Data["PageIsStargazers"] = true
	RenderUserCards(ctx, ctx.Repo.Repository.NumStars, ctx.Repo.Repository.GetStargazers, tplWatchers)
}

// Forks render repository's forked users
func Forks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repos.forks")

	forks, err := ctx.Repo.Repository.GetForks()
	if err != nil {
		ctx.Handle(500, "GetForks", err)
		return
	}

	for _, fork := range forks {
		if err = fork.GetOwner(); err != nil {
			ctx.Handle(500, "GetOwner", err)
			return
		}
	}
	ctx.Data["Forks"] = forks

	ctx.HTML(200, tplForks)
}
