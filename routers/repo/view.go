// Copyright 2017 The Gitea Authors. All rights reserved.
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
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplRepoEMPTY base.TplName = "repo/empty"
	tplRepoHome  base.TplName = "repo/home"
	tplWatchers  base.TplName = "repo/watchers"
	tplForks     base.TplName = "repo/forks"
	tplMigrating base.TplName = "repo/migrating"
)

func renderDirectory(ctx *context.Context, treeLink string) {
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.SubTree", git.IsErrNotExist, err)
		return
	}

	entries, err := tree.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	entries.CustomSort(base.NaturalSortLess)

	var latestCommit *git.Commit
	ctx.Data["Files"], latestCommit, err = entries.GetCommitsInfo(ctx.Repo.Commit, ctx.Repo.TreePath, nil)
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return
	}

	// 3 for the extensions in exts[] in order
	// the last one is for a readme that doesn't
	// strictly match an extension
	var readmeFiles [4]*git.Blob
	var exts = []string{".md", ".txt", ""} // sorted by priority
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		for i, ext := range exts {
			if markup.IsReadmeFile(entry.Name(), ext) {
				readmeFiles[i] = entry.Blob()
			}
		}

		if markup.IsReadmeFile(entry.Name()) {
			readmeFiles[3] = entry.Blob()
		}
	}

	var readmeFile *git.Blob
	for _, f := range readmeFiles {
		if f != nil {
			readmeFile = f
			break
		}
	}

	if readmeFile != nil {
		ctx.Data["RawFileLink"] = ""
		ctx.Data["ReadmeInList"] = true
		ctx.Data["ReadmeExist"] = true

		dataRc, err := readmeFile.DataAsync()
		if err != nil {
			ctx.ServerError("Data", err)
			return
		}
		defer dataRc.Close()

		buf := make([]byte, 1024)
		n, _ := dataRc.Read(buf)
		buf = buf[:n]

		isTextFile := base.IsTextFile(buf)
		ctx.Data["FileIsText"] = isTextFile
		ctx.Data["FileName"] = readmeFile.Name()
		fileSize := int64(0)
		isLFSFile := false
		ctx.Data["IsLFSFile"] = false

		// FIXME: what happens when README file is an image?
		if isTextFile && setting.LFS.StartServer {
			meta := lfs.IsPointerFile(&buf)
			if meta != nil {
				meta, err = ctx.Repo.Repository.GetLFSMetaObjectByOid(meta.Oid)
				if err != nil && err != models.ErrLFSObjectNotExist {
					ctx.ServerError("GetLFSMetaObject", err)
					return
				}
			}

			if meta != nil {
				ctx.Data["IsLFSFile"] = true
				isLFSFile = true

				// OK read the lfs object
				var err error
				dataRc, err = lfs.ReadMetaObject(meta)
				if err != nil {
					ctx.ServerError("ReadMetaObject", err)
					return
				}
				defer dataRc.Close()

				buf = make([]byte, 1024)
				n, err = dataRc.Read(buf)
				if err != nil {
					ctx.ServerError("Data", err)
					return
				}
				buf = buf[:n]

				isTextFile = base.IsTextFile(buf)
				ctx.Data["IsTextFile"] = isTextFile

				fileSize = meta.Size
				ctx.Data["FileSize"] = meta.Size
				filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(readmeFile.Name()))
				ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s.git/info/lfs/objects/%s/%s", setting.AppURL, ctx.Repo.Repository.FullName(), meta.Oid, filenameBase64)
			}
		}

		if !isLFSFile {
			fileSize = readmeFile.Size()
		}

		if isTextFile {
			if fileSize >= setting.UI.MaxDisplayFileSize {
				// Pretend that this is a normal text file to display 'This file is too large to be shown'
				ctx.Data["IsFileTooLarge"] = true
				ctx.Data["IsTextFile"] = true
				ctx.Data["FileSize"] = fileSize
			} else {
				d, _ := ioutil.ReadAll(dataRc)
				buf = charset.ToUTF8WithFallback(append(buf, d...))

				if markupType := markup.Type(readmeFile.Name()); markupType != "" {
					ctx.Data["IsMarkup"] = true
					ctx.Data["MarkupType"] = string(markupType)
					ctx.Data["FileContent"] = string(markup.Render(readmeFile.Name(), buf, treeLink, ctx.Repo.Repository.ComposeMetas()))
				} else {
					ctx.Data["IsRenderedHTML"] = true
					ctx.Data["FileContent"] = strings.Replace(
						gotemplate.HTMLEscapeString(string(buf)), "\n", `<br>`, -1,
					)
				}
			}
		}
	}

	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	ctx.Data["LatestCommit"] = latestCommit
	verification := models.ParseCommitWithSignature(latestCommit)

	if err := models.CalculateTrustStatus(verification, ctx.Repo.Repository, nil); err != nil {
		ctx.ServerError("CalculateTrustStatus", err)
		return
	}
	ctx.Data["LatestCommitVerification"] = verification

	ctx.Data["LatestCommitUser"] = models.ValidateCommitWithEmail(latestCommit)

	statuses, err := models.GetLatestCommitStatus(ctx.Repo.Repository, ctx.Repo.Commit.ID.String(), 0)
	if err != nil {
		log.Error("GetLatestCommitStatus: %v", err)
	}

	ctx.Data["LatestCommitStatus"] = models.CalcCommitStatus(statuses)

	// Check permission to add or upload new file.
	if ctx.Repo.CanWrite(models.UnitTypeCode) && ctx.Repo.IsViewBranch {
		ctx.Data["CanAddFile"] = !ctx.Repo.Repository.IsArchived
		ctx.Data["CanUploadFile"] = setting.Repository.Upload.Enabled && !ctx.Repo.Repository.IsArchived
	}
}

func renderFile(ctx *context.Context, entry *git.TreeEntry, treeLink, rawLink string) {
	ctx.Data["IsViewFile"] = true

	blob := entry.Blob()
	dataRc, err := blob.DataAsync()
	if err != nil {
		ctx.ServerError("DataAsync", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["Title"] = ctx.Data["Title"].(string) + " - " + ctx.Repo.TreePath + " at " + ctx.Repo.BranchName

	fileSize := blob.Size()
	ctx.Data["FileSize"] = fileSize
	ctx.Data["FileName"] = blob.Name()
	ctx.Data["HighlightClass"] = highlight.FileNameToHighlightClass(blob.Name())
	ctx.Data["RawFileLink"] = rawLink + "/" + ctx.Repo.TreePath

	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]

	isTextFile := base.IsTextFile(buf)
	isLFSFile := false
	ctx.Data["IsTextFile"] = isTextFile

	//Check for LFS meta file
	if isTextFile && setting.LFS.StartServer {
		meta := lfs.IsPointerFile(&buf)
		if meta != nil {
			meta, err = ctx.Repo.Repository.GetLFSMetaObjectByOid(meta.Oid)
			if err != nil && err != models.ErrLFSObjectNotExist {
				ctx.ServerError("GetLFSMetaObject", err)
				return
			}
		}
		if meta != nil {
			ctx.Data["IsLFSFile"] = true
			isLFSFile = true

			// OK read the lfs object
			var err error
			dataRc, err = lfs.ReadMetaObject(meta)
			if err != nil {
				ctx.ServerError("ReadMetaObject", err)
				return
			}
			defer dataRc.Close()

			buf = make([]byte, 1024)
			n, err = dataRc.Read(buf)
			if err != nil {
				ctx.ServerError("Data", err)
				return
			}
			buf = buf[:n]

			isTextFile = base.IsTextFile(buf)
			ctx.Data["IsTextFile"] = isTextFile

			fileSize = meta.Size
			ctx.Data["FileSize"] = meta.Size
			filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(blob.Name()))
			ctx.Data["RawFileLink"] = fmt.Sprintf("%s%s.git/info/lfs/objects/%s/%s", setting.AppURL, ctx.Repo.Repository.FullName(), meta.Oid, filenameBase64)
		}
	}
	// Check LFS Lock
	lfsLock, err := ctx.Repo.Repository.GetTreePathLock(ctx.Repo.TreePath)
	ctx.Data["LFSLock"] = lfsLock
	if err != nil {
		ctx.ServerError("GetTreePathLock", err)
		return
	}
	if lfsLock != nil {
		ctx.Data["LFSLockOwner"] = lfsLock.Owner.DisplayName()
		ctx.Data["LFSLockHint"] = ctx.Tr("repo.editor.this_file_locked")
	}

	// Assume file is not editable first.
	if isLFSFile {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_lfs_files")
	} else if !isTextFile {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_non_text_files")
	}

	switch {
	case isTextFile:
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		d, _ := ioutil.ReadAll(dataRc)
		buf = charset.ToUTF8WithFallback(append(buf, d...))

		readmeExist := markup.IsReadmeFile(blob.Name())
		ctx.Data["ReadmeExist"] = readmeExist
		if markupType := markup.Type(blob.Name()); markupType != "" {
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType
			ctx.Data["FileContent"] = string(markup.Render(blob.Name(), buf, path.Dir(treeLink), ctx.Repo.Repository.ComposeMetas()))
		} else if readmeExist {
			ctx.Data["IsRenderedHTML"] = true
			ctx.Data["FileContent"] = strings.Replace(
				gotemplate.HTMLEscapeString(string(buf)), "\n", `<br>`, -1,
			)
		} else {
			// Building code view blocks with line number on server side.
			var fileContent string
			if content, err := charset.ToUTF8WithErr(buf); err != nil {
				log.Error("ToUTF8WithErr: %v", err)
				fileContent = string(buf)
			} else {
				fileContent = content
			}

			var output bytes.Buffer
			lines := strings.Split(fileContent, "\n")
			ctx.Data["NumLines"] = len(lines)
			if len(lines) == 1 && lines[0] == "" {
				// If the file is completely empty, we show zero lines at the line counter
				ctx.Data["NumLines"] = 0
			}
			ctx.Data["NumLinesSet"] = true

			//Remove blank line at the end of file
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
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
				output.WriteString(fmt.Sprintf(`<span id="L%[1]d" data-line-number="%[1]d"></span>`, i+1))
			}
			ctx.Data["LineNums"] = gotemplate.HTML(output.String())
		}
		if !isLFSFile {
			if ctx.Repo.CanEnableEditor() {
				if lfsLock != nil && lfsLock.OwnerID != ctx.User.ID {
					ctx.Data["CanEditFile"] = false
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
				} else {
					ctx.Data["CanEditFile"] = true
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.edit_this_file")
				}
			} else if !ctx.Repo.IsViewBranch {
				ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
			} else if !ctx.Repo.CanWrite(models.UnitTypeCode) {
				ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.fork_before_edit")
			}
		}

	case base.IsPDFFile(buf):
		ctx.Data["IsPDFFile"] = true
	case base.IsVideoFile(buf):
		ctx.Data["IsVideoFile"] = true
	case base.IsAudioFile(buf):
		ctx.Data["IsAudioFile"] = true
	case base.IsImageFile(buf):
		ctx.Data["IsImageFile"] = true
	default:
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		if markupType := markup.Type(blob.Name()); markupType != "" {
			d, _ := ioutil.ReadAll(dataRc)
			buf = append(buf, d...)
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType
			ctx.Data["FileContent"] = string(markup.Render(blob.Name(), buf, path.Dir(treeLink), ctx.Repo.Repository.ComposeMetas()))
		}

	}

	if ctx.Repo.CanEnableEditor() {
		if lfsLock != nil && lfsLock.OwnerID != ctx.User.ID {
			ctx.Data["CanDeleteFile"] = false
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
		} else {
			ctx.Data["CanDeleteFile"] = true
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.delete_this_file")
		}
	} else if !ctx.Repo.IsViewBranch {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
	} else if !ctx.Repo.CanWrite(models.UnitTypeCode) {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
	}
}

func safeURL(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	u.User = nil
	return u.String()
}

// Home render repository home page
func Home(ctx *context.Context) {
	if len(ctx.Repo.Units) > 0 {
		if ctx.Repo.Repository.IsBeingCreated() {
			task, err := models.GetMigratingTask(ctx.Repo.Repository.ID)
			if err != nil {
				ctx.ServerError("models.GetMigratingTask", err)
				return
			}
			cfg, err := task.MigrateConfig()
			if err != nil {
				ctx.ServerError("task.MigrateConfig", err)
				return
			}

			ctx.Data["Repo"] = ctx.Repo
			ctx.Data["MigrateTask"] = task
			ctx.Data["CloneAddr"] = safeURL(cfg.CloneAddr)
			ctx.HTML(200, tplMigrating)
			return
		}

		var firstUnit *models.Unit
		for _, repoUnit := range ctx.Repo.Units {
			if repoUnit.Type == models.UnitTypeCode {
				renderCode(ctx)
				return
			}

			unit, ok := models.Units[repoUnit.Type]
			if ok && (firstUnit == nil || !firstUnit.IsLessThan(unit)) {
				firstUnit = &unit
			}
		}

		if firstUnit != nil {
			ctx.Redirect(fmt.Sprintf("%s/%s%s", setting.AppSubURL, ctx.Repo.Repository.FullName(), firstUnit.URI))
			return
		}
	}

	ctx.NotFound("Home", fmt.Errorf(ctx.Tr("units.error.no_unit_allowed_repo")))
}

func renderCode(ctx *context.Context) {
	ctx.Data["PageIsViewCode"] = true

	if ctx.Repo.Repository.IsEmpty {
		ctx.HTML(200, tplRepoEMPTY)
		return
	}

	title := ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	if len(ctx.Repo.Repository.Description) > 0 {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title
	ctx.Data["RequireHighlightJS"] = true

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL()

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	// Get Topics of this repo
	topics, err := models.FindTopics(&models.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("models.FindTopics", err)
		return
	}
	ctx.Data["Topics"] = topics

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
	pager := context.NewPagination(total, models.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	items, err := getter(pager.Paginater.Current())
	if err != nil {
		ctx.ServerError("getter", err)
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
		ctx.ServerError("GetForks", err)
		return
	}

	for _, fork := range forks {
		if err = fork.GetOwner(); err != nil {
			ctx.ServerError("GetOwner", err)
			return
		}
	}
	ctx.Data["Forks"] = forks

	ctx.HTML(200, tplForks)
}
