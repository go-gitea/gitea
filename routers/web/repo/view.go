// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	gocontext "context"
	"encoding/base64"
	"fmt"
	gotemplate "html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/feed"
)

const (
	tplRepoEMPTY    base.TplName = "repo/empty"
	tplRepoHome     base.TplName = "repo/home"
	tplRepoViewList base.TplName = "repo/view_list"
	tplWatchers     base.TplName = "repo/watchers"
	tplForks        base.TplName = "repo/forks"
	tplMigrating    base.TplName = "repo/migrate/migrating"
)

type namedBlob struct {
	name      string
	isSymlink bool
	blob      *git.Blob
}

// FIXME: There has to be a more efficient way of doing this
func getReadmeFileFromPath(ctx *context.Context, commit *git.Commit, treePath string) (*namedBlob, error) {
	tree, err := commit.SubTree(treePath)
	if err != nil {
		return nil, err
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}

	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localisation - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", ctx.Language()), ".txt", "") // sorted by priority
	extCount := len(exts)
	readmeFiles := make([]*namedBlob, extCount+1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if i, ok := markup.IsReadmeFileExtension(entry.Name(), exts...); ok {
			if readmeFiles[i] == nil || base.NaturalSortLess(readmeFiles[i].name, entry.Blob().Name()) {
				name := entry.Name()
				isSymlink := entry.IsLink()
				target := entry
				if isSymlink {
					target, err = entry.FollowLinks()
					if err != nil && !git.IsErrBadLink(err) {
						return nil, err
					}
				}
				if target != nil && (target.IsExecutable() || target.IsRegular()) {
					readmeFiles[i] = &namedBlob{
						name,
						isSymlink,
						target.Blob(),
					}
				}
			}
		}
	}
	var readmeFile *namedBlob
	for _, f := range readmeFiles {
		if f != nil {
			readmeFile = f
			break
		}
	}
	return readmeFile, nil
}

func renderDirectory(ctx *context.Context, treeLink string) {
	entries := renderDirectoryFiles(ctx, 1*time.Second)
	if ctx.Written() {
		return
	}

	if ctx.Repo.TreePath != "" {
		ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)
	}

	// Check permission to add or upload new file.
	if ctx.Repo.CanWrite(unit_model.TypeCode) && ctx.Repo.IsViewBranch {
		ctx.Data["CanAddFile"] = !ctx.Repo.Repository.IsArchived
		ctx.Data["CanUploadFile"] = setting.Repository.Upload.Enabled && !ctx.Repo.Repository.IsArchived
	}

	readmeFile, readmeTreelink := findReadmeFile(ctx, entries, treeLink)
	if ctx.Written() || readmeFile == nil {
		return
	}

	renderReadmeFile(ctx, readmeFile, readmeTreelink)
}

// localizedExtensions prepends the provided language code with and without a
// regional identifier to the provided extension.
// Note: the language code will always be lower-cased, if a region is present it must be separated with a `-`
// Note: ext should be prefixed with a `.`
func localizedExtensions(ext, languageCode string) (localizedExts []string) {
	if len(languageCode) < 1 {
		return []string{ext}
	}

	lowerLangCode := "." + strings.ToLower(languageCode)

	if strings.Contains(lowerLangCode, "-") {
		underscoreLangCode := strings.ReplaceAll(lowerLangCode, "-", "_")
		indexOfDash := strings.Index(lowerLangCode, "-")
		// e.g. [.zh-cn.md, .zh_cn.md, .zh.md, .md]
		return []string{lowerLangCode + ext, underscoreLangCode + ext, lowerLangCode[:indexOfDash] + ext, ext}
	}

	// e.g. [.en.md, .md]
	return []string{lowerLangCode + ext, ext}
}

func findReadmeFile(ctx *context.Context, entries git.Entries, treeLink string) (*namedBlob, string) {
	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localisation - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", ctx.Language()), ".txt", "") // sorted by priority
	extCount := len(exts)
	readmeFiles := make([]*namedBlob, extCount+1)

	docsEntries := make([]*git.TreeEntry, 3) // (one of docs/, .gitea/ or .github/)
	for _, entry := range entries {
		if entry.IsDir() {
			lowerName := strings.ToLower(entry.Name())
			switch lowerName {
			case "docs":
				if entry.Name() == "docs" || docsEntries[0] == nil {
					docsEntries[0] = entry
				}
			case ".gitea":
				if entry.Name() == ".gitea" || docsEntries[1] == nil {
					docsEntries[1] = entry
				}
			case ".github":
				if entry.Name() == ".github" || docsEntries[2] == nil {
					docsEntries[2] = entry
				}
			}
			continue
		}

		if i, ok := markup.IsReadmeFileExtension(entry.Name(), exts...); ok {
			log.Debug("Potential readme file: %s", entry.Name())
			name := entry.Name()
			isSymlink := entry.IsLink()
			target := entry
			if isSymlink {
				var err error
				target, err = entry.FollowLinks()
				if err != nil && !git.IsErrBadLink(err) {
					ctx.ServerError("FollowLinks", err)
					return nil, ""
				}
			}
			if target != nil && (target.IsExecutable() || target.IsRegular()) {
				readmeFiles[i] = &namedBlob{
					name,
					isSymlink,
					target.Blob(),
				}
			}
		}
	}

	var readmeFile *namedBlob
	readmeTreelink := treeLink
	for _, f := range readmeFiles {
		if f != nil {
			readmeFile = f
			break
		}
	}

	if ctx.Repo.TreePath == "" && readmeFile == nil {
		for _, entry := range docsEntries {
			if entry == nil {
				continue
			}
			var err error
			readmeFile, err = getReadmeFileFromPath(ctx, ctx.Repo.Commit, entry.GetSubJumpablePathName())
			if err != nil {
				ctx.ServerError("getReadmeFileFromPath", err)
				return nil, ""
			}
			if readmeFile != nil {
				readmeFile.name = entry.Name() + "/" + readmeFile.name
				readmeTreelink = treeLink + "/" + util.PathEscapeSegments(entry.GetSubJumpablePathName())
				break
			}
		}
	}
	return readmeFile, readmeTreelink
}

func renderReadmeFile(ctx *context.Context, readmeFile *namedBlob, readmeTreelink string) {
	ctx.Data["RawFileLink"] = ""
	ctx.Data["ReadmeInList"] = true
	ctx.Data["ReadmeExist"] = true
	ctx.Data["FileIsSymlink"] = readmeFile.isSymlink

	dataRc, err := readmeFile.blob.DataAsync()
	if err != nil {
		ctx.ServerError("Data", err)
		return
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)
	isTextFile := st.IsText()

	ctx.Data["FileIsText"] = isTextFile
	ctx.Data["FileName"] = readmeFile.name
	fileSize := int64(0)
	isLFSFile := false
	ctx.Data["IsLFSFile"] = false

	// FIXME: what happens when README file is an image?
	if isTextFile && setting.LFS.StartServer {
		pointer, _ := lfs.ReadPointerFromBuffer(buf)
		if pointer.IsValid() {
			meta, err := git_model.GetLFSMetaObjectByOid(ctx.Repo.Repository.ID, pointer.Oid)
			if err != nil && err != git_model.ErrLFSObjectNotExist {
				ctx.ServerError("GetLFSMetaObject", err)
				return
			}
			if meta != nil {
				ctx.Data["IsLFSFile"] = true
				isLFSFile = true

				// OK read the lfs object
				var err error
				dataRc, err = lfs.ReadMetaObject(pointer)
				if err != nil {
					ctx.ServerError("ReadMetaObject", err)
					return
				}
				defer dataRc.Close()

				buf = make([]byte, 1024)
				n, err = util.ReadAtMost(dataRc, buf)
				if err != nil {
					ctx.ServerError("Data", err)
					return
				}
				buf = buf[:n]

				st = typesniffer.DetectContentType(buf)
				isTextFile = st.IsText()
				ctx.Data["IsTextFile"] = isTextFile

				fileSize = meta.Size
				ctx.Data["FileSize"] = meta.Size
				filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(readmeFile.name))
				ctx.Data["RawFileLink"] = fmt.Sprintf("%s.git/info/lfs/objects/%s/%s", ctx.Repo.Repository.HTMLURL(), url.PathEscape(meta.Oid), url.PathEscape(filenameBase64))
			}
		}
	}

	if !isTextFile {
		return
	}

	if !isLFSFile {
		fileSize = readmeFile.blob.Size()
	}

	if fileSize >= setting.UI.MaxDisplayFileSize {
		// Pretend that this is a normal text file to display 'This file is too large to be shown'
		ctx.Data["IsFileTooLarge"] = true
		ctx.Data["IsTextFile"] = true
		ctx.Data["FileSize"] = fileSize
		return
	}

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))

	if markupType := markup.Type(readmeFile.name); markupType != "" {
		ctx.Data["IsMarkup"] = true
		ctx.Data["MarkupType"] = markupType

		ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, &markup.RenderContext{
			Ctx:          ctx,
			RelativePath: path.Join(ctx.Repo.TreePath, readmeFile.name), // ctx.Repo.TreePath is the directory not the Readme so we must append the Readme filename (and path).
			URLPrefix:    readmeTreelink,
			Metas:        ctx.Repo.Repository.ComposeDocumentMetas(),
			GitRepo:      ctx.Repo.GitRepo,
		}, rd)
		if err != nil {
			log.Error("Render failed for %s in %-v: %v Falling back to rendering source", readmeFile.name, ctx.Repo.Repository, err)
			buf := &bytes.Buffer{}
			ctx.Data["EscapeStatus"], _ = charset.EscapeControlReader(rd, buf, ctx.Locale)
			ctx.Data["FileContent"] = strings.ReplaceAll(
				gotemplate.HTMLEscapeString(buf.String()), "\n", `<br>`,
			)
		}
	} else {
		ctx.Data["IsRenderedHTML"] = true
		buf := &bytes.Buffer{}
		ctx.Data["EscapeStatus"], err = charset.EscapeControlReader(rd, &charset.BreakWriter{Writer: buf}, ctx.Locale, charset.RuneNBSP)
		if err != nil {
			log.Error("Read failed: %v", err)
		}

		ctx.Data["FileContent"] = buf.String()
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

	ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)

	fileSize := blob.Size()
	ctx.Data["FileIsSymlink"] = entry.IsLink()
	ctx.Data["FileName"] = blob.Name()
	ctx.Data["RawFileLink"] = rawLink + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

	if ctx.Repo.TreePath == ".editorconfig" {
		_, editorconfigErr := ctx.Repo.GetEditorconfig(ctx.Repo.Commit)
		ctx.Data["FileError"] = editorconfigErr
	}

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)
	isTextFile := st.IsText()

	isLFSFile := false
	isDisplayingSource := ctx.FormString("display") == "source"
	isDisplayingRendered := !isDisplayingSource

	// Check for LFS meta file
	if isTextFile && setting.LFS.StartServer {
		pointer, _ := lfs.ReadPointerFromBuffer(buf)
		if pointer.IsValid() {
			meta, err := git_model.GetLFSMetaObjectByOid(ctx.Repo.Repository.ID, pointer.Oid)
			if err != nil && err != git_model.ErrLFSObjectNotExist {
				ctx.ServerError("GetLFSMetaObject", err)
				return
			}
			if meta != nil {
				isLFSFile = true

				// OK read the lfs object
				var err error
				dataRc, err = lfs.ReadMetaObject(pointer)
				if err != nil {
					ctx.ServerError("ReadMetaObject", err)
					return
				}
				defer dataRc.Close()

				buf = make([]byte, 1024)
				n, err = util.ReadAtMost(dataRc, buf)
				if err != nil {
					ctx.ServerError("Data", err)
					return
				}
				buf = buf[:n]

				st = typesniffer.DetectContentType(buf)
				isTextFile = st.IsText()

				fileSize = meta.Size
				ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/media/" + ctx.Repo.BranchNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
			}
		}
	}

	isRepresentableAsText := st.IsRepresentableAsText()
	if !isRepresentableAsText {
		// If we can't show plain text, always try to render.
		isDisplayingSource = false
		isDisplayingRendered = true
	}
	ctx.Data["IsLFSFile"] = isLFSFile
	ctx.Data["FileSize"] = fileSize
	ctx.Data["IsTextFile"] = isTextFile
	ctx.Data["IsRepresentableAsText"] = isRepresentableAsText
	ctx.Data["IsDisplayingSource"] = isDisplayingSource
	ctx.Data["IsDisplayingRendered"] = isDisplayingRendered
	ctx.Data["IsTextSource"] = isTextFile || isDisplayingSource

	// Check LFS Lock
	lfsLock, err := git_model.GetTreePathLock(ctx.Repo.Repository.ID, ctx.Repo.TreePath)
	ctx.Data["LFSLock"] = lfsLock
	if err != nil {
		ctx.ServerError("GetTreePathLock", err)
		return
	}
	if lfsLock != nil {
		u, err := user_model.GetUserByID(lfsLock.OwnerID)
		if err != nil {
			ctx.ServerError("GetTreePathLock", err)
			return
		}
		ctx.Data["LFSLockOwner"] = u.Name
		ctx.Data["LFSLockOwnerHomeLink"] = u.HomeLink()
		ctx.Data["LFSLockHint"] = ctx.Tr("repo.editor.this_file_locked")
	}

	// Assume file is not editable first.
	if isLFSFile {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_lfs_files")
	} else if !isRepresentableAsText {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_non_text_files")
	}

	switch {
	case isRepresentableAsText:
		if st.IsSvgImage() {
			ctx.Data["IsImageFile"] = true
			ctx.Data["HasSourceRenderedToggle"] = true
		}

		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))

		shouldRenderSource := ctx.FormString("display") == "source"
		readmeExist := markup.IsReadmeFile(blob.Name())
		ctx.Data["ReadmeExist"] = readmeExist

		markupType := markup.Type(blob.Name())
		// If the markup is detected by custom markup renderer it should not be reset later on
		// to not pass it down to the render context.
		detected := false
		if markupType == "" {
			detected = true
			markupType = markup.DetectRendererType(blob.Name(), bytes.NewReader(buf))
		}
		if markupType != "" {
			ctx.Data["HasSourceRenderedToggle"] = true
		}

		if markupType != "" && !shouldRenderSource {
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType
			if !detected {
				markupType = ""
			}
			metas := ctx.Repo.Repository.ComposeDocumentMetas()
			metas["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
			ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, &markup.RenderContext{
				Ctx:          ctx,
				Type:         markupType,
				RelativePath: ctx.Repo.TreePath,
				URLPrefix:    path.Dir(treeLink),
				Metas:        metas,
				GitRepo:      ctx.Repo.GitRepo,
			}, rd)
			if err != nil {
				ctx.ServerError("Render", err)
				return
			}
			// to prevent iframe load third-party url
			ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
		} else if readmeExist && !shouldRenderSource {
			buf := &bytes.Buffer{}
			ctx.Data["IsRenderedHTML"] = true

			ctx.Data["EscapeStatus"], _ = charset.EscapeControlReader(rd, buf, ctx.Locale)

			ctx.Data["FileContent"] = strings.ReplaceAll(
				gotemplate.HTMLEscapeString(buf.String()), "\n", `<br>`,
			)
		} else {
			buf, _ := io.ReadAll(rd)

			// empty: 0 lines; "a": one line; "a\n": two lines; "a\nb": two lines;
			// the NumLines is only used for the display on the UI: "xxx lines"
			if len(buf) == 0 {
				ctx.Data["NumLines"] = 0
			} else {
				ctx.Data["NumLines"] = bytes.Count(buf, []byte{'\n'}) + 1
			}
			ctx.Data["NumLinesSet"] = true

			language := ""

			indexFilename, worktree, deleteTemporaryFile, err := ctx.Repo.GitRepo.ReadTreeToTemporaryIndex(ctx.Repo.CommitID)
			if err == nil {
				defer deleteTemporaryFile()

				filename2attribute2info, err := ctx.Repo.GitRepo.CheckAttribute(git.CheckAttributeOpts{
					CachedOnly: true,
					Attributes: []string{"linguist-language", "gitlab-language"},
					Filenames:  []string{ctx.Repo.TreePath},
					IndexFile:  indexFilename,
					WorkTree:   worktree,
				})
				if err != nil {
					log.Error("Unable to load attributes for %-v:%s. Error: %v", ctx.Repo.Repository, ctx.Repo.TreePath, err)
				}

				language = filename2attribute2info[ctx.Repo.TreePath]["linguist-language"]
				if language == "" || language == "unspecified" {
					language = filename2attribute2info[ctx.Repo.TreePath]["gitlab-language"]
				}
				if language == "unspecified" {
					language = ""
				}
			}
			fileContent, err := highlight.File(blob.Name(), language, buf)
			if err != nil {
				log.Error("highlight.File failed, fallback to plain text: %v", err)
				fileContent = highlight.PlainText(buf)
			}
			status := &charset.EscapeStatus{}
			statuses := make([]*charset.EscapeStatus, len(fileContent))
			for i, line := range fileContent {
				statuses[i], fileContent[i] = charset.EscapeControlHTML(line, ctx.Locale)
				status = status.Or(statuses[i])
			}
			ctx.Data["EscapeStatus"] = status
			ctx.Data["FileContent"] = fileContent
			ctx.Data["LineEscapeStatus"] = statuses
		}
		if !isLFSFile {
			if ctx.Repo.CanEnableEditor(ctx.Doer) {
				if lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID {
					ctx.Data["CanEditFile"] = false
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
				} else {
					ctx.Data["CanEditFile"] = true
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.edit_this_file")
				}
			} else if !ctx.Repo.IsViewBranch {
				ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
			} else if !ctx.Repo.CanWriteToBranch(ctx.Doer, ctx.Repo.BranchName) {
				ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.fork_before_edit")
			}
		}

	case st.IsPDF():
		ctx.Data["IsPDFFile"] = true
	case st.IsVideo():
		ctx.Data["IsVideoFile"] = true
	case st.IsAudio():
		ctx.Data["IsAudioFile"] = true
	case st.IsImage() && (setting.UI.SVG.Enabled || !st.IsSvgImage()):
		ctx.Data["IsImageFile"] = true
	default:
		if fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		if markupType := markup.Type(blob.Name()); markupType != "" {
			rd := io.MultiReader(bytes.NewReader(buf), dataRc)
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType
			ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, &markup.RenderContext{
				Ctx:          ctx,
				RelativePath: ctx.Repo.TreePath,
				URLPrefix:    path.Dir(treeLink),
				Metas:        ctx.Repo.Repository.ComposeDocumentMetas(),
				GitRepo:      ctx.Repo.GitRepo,
			}, rd)
			if err != nil {
				ctx.ServerError("Render", err)
				return
			}
		}
	}

	if ctx.Repo.CanEnableEditor(ctx.Doer) {
		if lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID {
			ctx.Data["CanDeleteFile"] = false
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
		} else {
			ctx.Data["CanDeleteFile"] = true
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.delete_this_file")
		}
	} else if !ctx.Repo.IsViewBranch {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
	} else if !ctx.Repo.CanWriteToBranch(ctx.Doer, ctx.Repo.BranchName) {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
	}
}

func markupRender(ctx *context.Context, renderCtx *markup.RenderContext, input io.Reader) (escaped *charset.EscapeStatus, output string, err error) {
	markupRd, markupWr := io.Pipe()
	defer markupWr.Close()
	done := make(chan struct{})
	go func() {
		sb := &strings.Builder{}
		// We allow NBSP here this is rendered
		escaped, _ = charset.EscapeControlReader(markupRd, sb, ctx.Locale, charset.RuneNBSP)
		output = sb.String()
		close(done)
	}()
	err = markup.Render(renderCtx, input, markupWr)
	_ = markupWr.CloseWithError(err)
	<-done
	return escaped, output, err
}

func safeURL(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	u.User = nil
	return u.String()
}

func checkHomeCodeViewable(ctx *context.Context) {
	if len(ctx.Repo.Units) > 0 {
		if ctx.Repo.Repository.IsBeingCreated() {
			task, err := admin_model.GetMigratingTask(ctx.Repo.Repository.ID)
			if err != nil {
				if admin_model.IsErrTaskDoesNotExist(err) {
					ctx.Data["Repo"] = ctx.Repo
					ctx.Data["CloneAddr"] = ""
					ctx.Data["Failed"] = true
					ctx.HTML(http.StatusOK, tplMigrating)
					return
				}
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
			ctx.Data["Failed"] = task.Status == structs.TaskStatusFailed
			ctx.HTML(http.StatusOK, tplMigrating)
			return
		}

		if ctx.IsSigned {
			// Set repo notification-status read if unread
			if err := activities_model.SetRepoReadBy(ctx, ctx.Repo.Repository.ID, ctx.Doer.ID); err != nil {
				ctx.ServerError("ReadBy", err)
				return
			}
		}

		var firstUnit *unit_model.Unit
		for _, repoUnit := range ctx.Repo.Units {
			if repoUnit.Type == unit_model.TypeCode {
				return
			}

			unit, ok := unit_model.Units[repoUnit.Type]
			if ok && (firstUnit == nil || !firstUnit.IsLessThan(unit)) {
				firstUnit = &unit
			}
		}

		if firstUnit != nil {
			ctx.Redirect(fmt.Sprintf("%s%s", ctx.Repo.Repository.Link(), firstUnit.URI))
			return
		}
	}

	ctx.NotFound("Home", fmt.Errorf(ctx.Tr("units.error.no_unit_allowed_repo")))
}

// Home render repository home page
func Home(ctx *context.Context) {
	isFeed, _, showFeedType := feed.GetFeedType(ctx.Params(":reponame"), ctx.Req)
	if isFeed {
		feed.ShowRepoFeed(ctx, ctx.Repo.Repository, showFeedType)
		return
	}

	ctx.Data["FeedURL"] = ctx.Repo.Repository.HTMLURL()

	checkHomeCodeViewable(ctx)
	if ctx.Written() {
		return
	}

	renderCode(ctx)
}

// LastCommit returns lastCommit data for the provided branch/tag/commit and directory (in url) and filenames in body
func LastCommit(ctx *context.Context) {
	checkHomeCodeViewable(ctx)
	if ctx.Written() {
		return
	}

	renderDirectoryFiles(ctx, 0)
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
	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["BranchLink"] = branchLink

	ctx.HTML(http.StatusOK, tplRepoViewList)
}

func renderDirectoryFiles(ctx *context.Context, timeout time.Duration) git.Entries {
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.SubTree", git.IsErrNotExist, err)
		return nil
	}

	ctx.Data["LastCommitLoaderURL"] = ctx.Repo.RepoLink + "/lastcommit/" + url.PathEscape(ctx.Repo.CommitID) + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return nil
	}

	if !entry.IsDir() {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return nil
	}

	allEntries, err := tree.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return nil
	}
	allEntries.CustomSort(base.NaturalSortLess)

	commitInfoCtx := gocontext.Context(ctx)
	if timeout > 0 {
		var cancel gocontext.CancelFunc
		commitInfoCtx, cancel = gocontext.WithTimeout(ctx, timeout)
		defer cancel()
	}

	selected := make(container.Set[string])
	selected.AddMultiple(ctx.FormStrings("f[]")...)

	entries := allEntries
	if len(selected) > 0 {
		entries = make(git.Entries, 0, len(selected))
		for _, entry := range allEntries {
			if selected.Contains(entry.Name()) {
				entries = append(entries, entry)
			}
		}
	}

	var latestCommit *git.Commit
	ctx.Data["Files"], latestCommit, err = entries.GetCommitsInfo(commitInfoCtx, ctx.Repo.Commit, ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return nil
	}

	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	ctx.Data["LatestCommit"] = latestCommit
	if latestCommit != nil {

		verification := asymkey_model.ParseCommitWithSignature(latestCommit)

		if err := asymkey_model.CalculateTrustStatus(verification, ctx.Repo.Repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
			return repo_model.IsOwnerMemberCollaborator(ctx.Repo.Repository, user.ID)
		}, nil); err != nil {
			ctx.ServerError("CalculateTrustStatus", err)
			return nil
		}
		ctx.Data["LatestCommitVerification"] = verification
		ctx.Data["LatestCommitUser"] = user_model.ValidateCommitWithEmail(latestCommit)

		statuses, _, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, latestCommit.ID.String(), db.ListOptions{})
		if err != nil {
			log.Error("GetLatestCommitStatus: %v", err)
		}

		ctx.Data["LatestCommitStatus"] = git_model.CalcCommitStatus(statuses)
		ctx.Data["LatestCommitStatuses"] = statuses
	}

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	ctx.Data["TreeLink"] = treeLink
	ctx.Data["SSHDomain"] = setting.SSH.Domain

	return allEntries
}

func renderLanguageStats(ctx *context.Context) {
	langs, err := repo_model.GetTopLanguageStats(ctx.Repo.Repository, 5)
	if err != nil {
		ctx.ServerError("Repo.GetTopLanguageStats", err)
		return
	}

	ctx.Data["LanguageStats"] = langs
}

func renderRepoTopics(ctx *context.Context) {
	topics, _, err := repo_model.FindTopics(&repo_model.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("models.FindTopics", err)
		return
	}
	ctx.Data["Topics"] = topics
}

func renderCode(ctx *context.Context) {
	ctx.Data["PageIsViewCode"] = true

	if ctx.Repo.Repository.IsEmpty {
		reallyEmpty := true
		var err error
		if ctx.Repo.GitRepo != nil {
			reallyEmpty, err = ctx.Repo.GitRepo.IsEmpty()
			if err != nil {
				ctx.ServerError("GitRepo.IsEmpty", err)
				return
			}
		}
		if reallyEmpty {
			ctx.HTML(http.StatusOK, tplRepoEMPTY)
			return
		}
		// the repo is not really empty, so we should update the modal in database
		// such problem may be caused by:
		// 1) an error occurs during pushing/receiving.  2) the user replaces an empty git repo manually
		// and even more: the IsEmpty flag is deeply broken and should be removed with the UI changed to manage to cope with empty repos.
		// it's possible for a repository to be non-empty by that flag but still 500
		// because there are no branches - only tags -or the default branch is non-extant as it has been 0-pushed.
		ctx.Repo.Repository.IsEmpty = false
		if err = repo_model.UpdateRepositoryCols(ctx, ctx.Repo.Repository, "is_empty"); err != nil {
			ctx.ServerError("UpdateRepositoryCols", err)
			return
		}
		if err = repo_module.UpdateRepoSize(ctx, ctx.Repo.Repository); err != nil {
			ctx.ServerError("UpdateRepoSize", err)
			return
		}
	}

	title := ctx.Repo.Repository.Owner.Name + "/" + ctx.Repo.Repository.Name
	if len(ctx.Repo.Repository.Description) > 0 {
		title += ": " + ctx.Repo.Repository.Description
	}
	ctx.Data["Title"] = title

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink
	rawLink := ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL()

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	// Get Topics of this repo
	renderRepoTopics(ctx)
	if ctx.Written() {
		return
	}

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.GetTreeEntryByPath", git.IsErrNotExist, err)
		return
	}

	renderLanguageStats(ctx)
	if ctx.Written() {
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
	ctx.HTML(http.StatusOK, tplRepoHome)
}

// RenderUserCards render a page show users according the input template
func RenderUserCards(ctx *context.Context, total int, getter func(opts db.ListOptions) ([]*user_model.User, error), tpl base.TplName) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}
	pager := context.NewPagination(total, setting.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	items, err := getter(db.ListOptions{
		Page:     pager.Paginater.Current(),
		PageSize: setting.ItemsPerPage,
	})
	if err != nil {
		ctx.ServerError("getter", err)
		return
	}
	ctx.Data["Cards"] = items

	ctx.HTML(http.StatusOK, tpl)
}

// Watchers render repository's watch users
func Watchers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.watchers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.watchers")
	ctx.Data["PageIsWatchers"] = true

	RenderUserCards(ctx, ctx.Repo.Repository.NumWatches, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetRepoWatchers(ctx.Repo.Repository.ID, opts)
	}, tplWatchers)
}

// Stars render repository's starred users
func Stars(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.stargazers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.stargazers")
	ctx.Data["PageIsStargazers"] = true
	RenderUserCards(ctx, ctx.Repo.Repository.NumStars, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetStargazers(ctx.Repo.Repository, opts)
	}, tplWatchers)
}

// Forks render repository's forked users
func Forks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repos.forks")

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	pager := context.NewPagination(ctx.Repo.Repository.NumForks, setting.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	forks, err := repo_model.GetForks(ctx.Repo.Repository, db.ListOptions{
		Page:     pager.Paginater.Current(),
		PageSize: setting.ItemsPerPage,
	})
	if err != nil {
		ctx.ServerError("GetForks", err)
		return
	}

	for _, fork := range forks {
		if err = fork.GetOwner(ctx); err != nil {
			ctx.ServerError("GetOwner", err)
			return
		}
	}

	ctx.Data["Forks"] = forks

	ctx.HTML(http.StatusOK, tplForks)
}
