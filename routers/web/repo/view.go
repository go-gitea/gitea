// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	gocontext "context"
	"encoding/base64"
	"fmt"
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
	"code.gitea.io/gitea/modules/actions"
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
	issue_service "code.gitea.io/gitea/services/issue"

	"github.com/nektos/act/pkg/model"
)

const (
	tplRepoEMPTY    base.TplName = "repo/empty"
	tplRepoHome     base.TplName = "repo/home"
	tplRepoViewList base.TplName = "repo/view_list"
	tplWatchers     base.TplName = "repo/watchers"
	tplForks        base.TplName = "repo/forks"
	tplMigrating    base.TplName = "repo/migrate/migrating"
)

// locate a README for a tree in one of the supported paths.
//
// entries is passed to reduce calls to ListEntries(), so
// this has precondition:
//
//	entries == ctx.Repo.Commit.SubTree(ctx.Repo.TreePath).ListEntries()
//
// FIXME: There has to be a more efficient way of doing this
func findReadmeFileInEntries(ctx *context.Context, entries []*git.TreeEntry, tryWellKnownDirs bool) (string, *git.TreeEntry, error) {
	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localisation - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", ctx.Locale.Language()), ".txt", "") // sorted by priority
	extCount := len(exts)
	readmeFiles := make([]*git.TreeEntry, extCount+1)

	docsEntries := make([]*git.TreeEntry, 3) // (one of docs/, .gitea/ or .github/)
	for _, entry := range entries {
		if tryWellKnownDirs && entry.IsDir() {
			// as a special case for the top-level repo introduction README,
			// fall back to subfolders, looking for e.g. docs/README.md, .gitea/README.zh-CN.txt, .github/README.txt, ...
			// (note that docsEntries is ignored unless we are at the root)
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
		if i, ok := util.IsReadmeFileExtension(entry.Name(), exts...); ok {
			log.Debug("Potential readme file: %s", entry.Name())
			if readmeFiles[i] == nil || base.NaturalSortLess(readmeFiles[i].Name(), entry.Blob().Name()) {
				if entry.IsLink() {
					target, err := entry.FollowLinks()
					if err != nil && !git.IsErrBadLink(err) {
						return "", nil, err
					} else if target != nil && (target.IsExecutable() || target.IsRegular()) {
						readmeFiles[i] = entry
					}
				} else {
					readmeFiles[i] = entry
				}
			}
		}
	}
	var readmeFile *git.TreeEntry
	for _, f := range readmeFiles {
		if f != nil {
			readmeFile = f
			break
		}
	}

	if ctx.Repo.TreePath == "" && readmeFile == nil {
		for _, subTreeEntry := range docsEntries {
			if subTreeEntry == nil {
				continue
			}
			subTree := subTreeEntry.Tree()
			if subTree == nil {
				// this should be impossible; if subTreeEntry exists so should this.
				continue
			}
			var err error
			childEntries, err := subTree.ListEntries()
			if err != nil {
				return "", nil, err
			}

			subfolder, readmeFile, err := findReadmeFileInEntries(ctx, childEntries, false)
			if err != nil && !git.IsErrNotExist(err) {
				return "", nil, err
			}
			if readmeFile != nil {
				return path.Join(subTreeEntry.Name(), subfolder), readmeFile, nil
			}
		}
	}

	return "", readmeFile, nil
}

func renderDirectory(ctx *context.Context, treeLink string) {
	entries := renderDirectoryFiles(ctx, 1*time.Second)
	if ctx.Written() {
		return
	}

	if ctx.Repo.TreePath != "" {
		ctx.Data["HideRepoInfo"] = true
		ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)
	}

	subfolder, readmeFile, err := findReadmeFileInEntries(ctx, entries, true)
	if err != nil {
		ctx.ServerError("findReadmeFileInEntries", err)
		return
	}

	renderReadmeFile(ctx, subfolder, readmeFile, treeLink)
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
		// e.g. [.zh-cn.md, .zh_cn.md, .zh.md, _zh.md, .md]
		return []string{lowerLangCode + ext, underscoreLangCode + ext, lowerLangCode[:indexOfDash] + ext, "_" + lowerLangCode[1:indexOfDash] + ext, ext}
	}

	// e.g. [.en.md, .md]
	return []string{lowerLangCode + ext, ext}
}

type fileInfo struct {
	isTextFile bool
	isLFSFile  bool
	fileSize   int64
	lfsMeta    *lfs.Pointer
	st         typesniffer.SniffedType
}

func getFileReader(repoID int64, blob *git.Blob) ([]byte, io.ReadCloser, *fileInfo, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return nil, nil, nil, err
	}

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	st := typesniffer.DetectContentType(buf)
	isTextFile := st.IsText()

	// FIXME: what happens when README file is an image?
	if !isTextFile || !setting.LFS.StartServer {
		return buf, dataRc, &fileInfo{isTextFile, false, blob.Size(), nil, st}, nil
	}

	pointer, _ := lfs.ReadPointerFromBuffer(buf)
	if !pointer.IsValid() { // fallback to plain file
		return buf, dataRc, &fileInfo{isTextFile, false, blob.Size(), nil, st}, nil
	}

	meta, err := git_model.GetLFSMetaObjectByOid(db.DefaultContext, repoID, pointer.Oid)
	if err != nil && err != git_model.ErrLFSObjectNotExist { // fallback to plain file
		return buf, dataRc, &fileInfo{isTextFile, false, blob.Size(), nil, st}, nil
	}

	dataRc.Close()
	if err != nil {
		return nil, nil, nil, err
	}

	dataRc, err = lfs.ReadMetaObject(pointer)
	if err != nil {
		return nil, nil, nil, err
	}

	buf = make([]byte, 1024)
	n, err = util.ReadAtMost(dataRc, buf)
	if err != nil {
		dataRc.Close()
		return nil, nil, nil, err
	}
	buf = buf[:n]

	st = typesniffer.DetectContentType(buf)

	return buf, dataRc, &fileInfo{st.IsText(), true, meta.Size, &meta.Pointer, st}, nil
}

func renderReadmeFile(ctx *context.Context, subfolder string, readmeFile *git.TreeEntry, readmeTreelink string) {
	target := readmeFile
	if readmeFile != nil && readmeFile.IsLink() {
		target, _ = readmeFile.FollowLinks()
	}
	if target == nil {
		// if findReadmeFile() failed and/or gave us a broken symlink (which it shouldn't)
		// simply skip rendering the README
		return
	}

	ctx.Data["RawFileLink"] = ""
	ctx.Data["ReadmeInList"] = true
	ctx.Data["ReadmeExist"] = true
	ctx.Data["FileIsSymlink"] = readmeFile.IsLink()

	buf, dataRc, fInfo, err := getFileReader(ctx.Repo.Repository.ID, target.Blob())
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["FileIsText"] = fInfo.isTextFile
	ctx.Data["FileName"] = path.Join(subfolder, readmeFile.Name())
	ctx.Data["IsLFSFile"] = fInfo.isLFSFile

	if fInfo.isLFSFile {
		filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(readmeFile.Name()))
		ctx.Data["RawFileLink"] = fmt.Sprintf("%s.git/info/lfs/objects/%s/%s", ctx.Repo.Repository.Link(), url.PathEscape(fInfo.lfsMeta.Oid), url.PathEscape(filenameBase64))
	}

	if !fInfo.isTextFile {
		return
	}

	if fInfo.fileSize >= setting.UI.MaxDisplayFileSize {
		// Pretend that this is a normal text file to display 'This file is too large to be shown'
		ctx.Data["IsFileTooLarge"] = true
		ctx.Data["IsTextFile"] = true
		ctx.Data["FileSize"] = fInfo.fileSize
		return
	}

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))

	if markupType := markup.Type(readmeFile.Name()); markupType != "" {
		ctx.Data["IsMarkup"] = true
		ctx.Data["MarkupType"] = markupType

		ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, &markup.RenderContext{
			Ctx:          ctx,
			RelativePath: path.Join(ctx.Repo.TreePath, readmeFile.Name()), // ctx.Repo.TreePath is the directory not the Readme so we must append the Readme filename (and path).
			URLPrefix:    path.Join(readmeTreelink, subfolder),
			Metas:        ctx.Repo.Repository.ComposeDocumentMetas(),
			GitRepo:      ctx.Repo.GitRepo,
		}, rd)
		if err != nil {
			log.Error("Render failed for %s in %-v: %v Falling back to rendering source", readmeFile.Name(), ctx.Repo.Repository, err)
			buf := &bytes.Buffer{}
			ctx.Data["EscapeStatus"], _ = charset.EscapeControlStringReader(rd, buf, ctx.Locale)
			ctx.Data["FileContent"] = buf.String()
		}
	} else {
		ctx.Data["IsPlainText"] = true
		buf := &bytes.Buffer{}
		ctx.Data["EscapeStatus"], err = charset.EscapeControlStringReader(rd, buf, ctx.Locale)
		if err != nil {
			log.Error("Read failed: %v", err)
		}

		ctx.Data["FileContent"] = buf.String()
	}
}

func renderFile(ctx *context.Context, entry *git.TreeEntry, treeLink, rawLink string) {
	ctx.Data["IsViewFile"] = true
	ctx.Data["HideRepoInfo"] = true
	blob := entry.Blob()
	buf, dataRc, fInfo, err := getFileReader(ctx.Repo.Repository.ID, blob)
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)
	ctx.Data["FileIsSymlink"] = entry.IsLink()
	ctx.Data["FileName"] = blob.Name()
	ctx.Data["RawFileLink"] = rawLink + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

	if ctx.Repo.TreePath == ".editorconfig" {
		_, editorconfigWarning, editorconfigErr := ctx.Repo.GetEditorconfig(ctx.Repo.Commit)
		if editorconfigWarning != nil {
			ctx.Data["FileWarning"] = strings.TrimSpace(editorconfigWarning.Error())
		}
		if editorconfigErr != nil {
			ctx.Data["FileError"] = strings.TrimSpace(editorconfigErr.Error())
		}
	} else if issue_service.IsTemplateConfig(ctx.Repo.TreePath) {
		_, issueConfigErr := issue_service.GetTemplateConfig(ctx.Repo.GitRepo, ctx.Repo.TreePath, ctx.Repo.Commit)
		if issueConfigErr != nil {
			ctx.Data["FileError"] = strings.TrimSpace(issueConfigErr.Error())
		}
	} else if actions.IsWorkflow(ctx.Repo.TreePath) {
		content, err := actions.GetContentFromEntry(entry)
		if err != nil {
			log.Error("actions.GetContentFromEntry: %v", err)
		}
		_, workFlowErr := model.ReadWorkflow(bytes.NewReader(content))
		if workFlowErr != nil {
			ctx.Data["FileError"] = ctx.Locale.Tr("actions.runs.invalid_workflow_helper", workFlowErr.Error())
		}
	}

	isDisplayingSource := ctx.FormString("display") == "source"
	isDisplayingRendered := !isDisplayingSource

	if fInfo.isLFSFile {
		ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/media/" + ctx.Repo.BranchNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	isRepresentableAsText := fInfo.st.IsRepresentableAsText()
	if !isRepresentableAsText {
		// If we can't show plain text, always try to render.
		isDisplayingSource = false
		isDisplayingRendered = true
	}
	ctx.Data["IsLFSFile"] = fInfo.isLFSFile
	ctx.Data["FileSize"] = fInfo.fileSize
	ctx.Data["IsTextFile"] = fInfo.isTextFile
	ctx.Data["IsRepresentableAsText"] = isRepresentableAsText
	ctx.Data["IsDisplayingSource"] = isDisplayingSource
	ctx.Data["IsDisplayingRendered"] = isDisplayingRendered

	isTextSource := fInfo.isTextFile || isDisplayingSource
	ctx.Data["IsTextSource"] = isTextSource
	if isTextSource {
		ctx.Data["CanCopyContent"] = true
	}

	// Check LFS Lock
	lfsLock, err := git_model.GetTreePathLock(ctx, ctx.Repo.Repository.ID, ctx.Repo.TreePath)
	ctx.Data["LFSLock"] = lfsLock
	if err != nil {
		ctx.ServerError("GetTreePathLock", err)
		return
	}
	if lfsLock != nil {
		u, err := user_model.GetUserByID(ctx, lfsLock.OwnerID)
		if err != nil {
			ctx.ServerError("GetTreePathLock", err)
			return
		}
		ctx.Data["LFSLockOwner"] = u.Name
		ctx.Data["LFSLockOwnerHomeLink"] = u.HomeLink()
		ctx.Data["LFSLockHint"] = ctx.Tr("repo.editor.this_file_locked")
	}

	// Assume file is not editable first.
	if fInfo.isLFSFile {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_lfs_files")
	} else if !isRepresentableAsText {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.cannot_edit_non_text_files")
	}

	switch {
	case isRepresentableAsText:
		if fInfo.st.IsSvgImage() {
			ctx.Data["IsImageFile"] = true
			ctx.Data["CanCopyContent"] = true
			ctx.Data["HasSourceRenderedToggle"] = true
		}

		if fInfo.fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))

		shouldRenderSource := ctx.FormString("display") == "source"
		readmeExist := util.IsReadmeFileName(blob.Name())
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
			fileContent, lexerName, err := highlight.File(blob.Name(), language, buf)
			ctx.Data["LexerName"] = lexerName
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
		if !fInfo.isLFSFile {
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

	case fInfo.st.IsPDF():
		ctx.Data["IsPDFFile"] = true
	case fInfo.st.IsVideo():
		ctx.Data["IsVideoFile"] = true
	case fInfo.st.IsAudio():
		ctx.Data["IsAudioFile"] = true
	case fInfo.st.IsImage() && (setting.UI.SVG.Enabled || !fInfo.st.IsSvgImage()):
		ctx.Data["IsImageFile"] = true
		ctx.Data["CanCopyContent"] = true
	default:
		if fInfo.fileSize >= setting.UI.MaxDisplayFileSize {
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

func checkCitationFile(ctx *context.Context, entry *git.TreeEntry) {
	if entry.Name() != "" {
		return
	}
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		ctx.NotFoundOrServerError("Repo.Commit.SubTree", git.IsErrNotExist, err)
		return
	}
	allEntries, err := tree.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	for _, entry := range allEntries {
		if entry.Name() == "CITATION.cff" || entry.Name() == "CITATION.bib" {
			ctx.Data["CitiationExist"] = true
			// Read Citation file contents
			blob := entry.Blob()
			dataRc, err := blob.DataAsync()
			if err != nil {
				ctx.ServerError("DataAsync", err)
				return
			}
			defer dataRc.Close()
			buf := make([]byte, 1024)
			n, err := util.ReadAtMost(dataRc, buf)
			if err != nil {
				ctx.ServerError("ReadAtMost", err)
				return
			}
			buf = buf[:n]
			ctx.PageData["citationFileContent"] = string(buf)
			break
		}
	}
}

// Home render repository home page
func Home(ctx *context.Context) {
	if setting.Other.EnableFeed {
		isFeed, _, showFeedType := feed.GetFeedType(ctx.Params(":reponame"), ctx.Req)
		if isFeed {
			switch {
			case ctx.Link == fmt.Sprintf("%s.%s", ctx.Repo.RepoLink, showFeedType):
				feed.ShowRepoFeed(ctx, ctx.Repo.Repository, showFeedType)
			case ctx.Repo.TreePath == "":
				feed.ShowBranchFeed(ctx, ctx.Repo.Repository, showFeedType)
			case ctx.Repo.TreePath != "":
				feed.ShowFileFeed(ctx, ctx.Repo.Repository, showFeedType)
			}
			return
		}
	}

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

		verification := asymkey_model.ParseCommitWithSignature(ctx, latestCommit)

		if err := asymkey_model.CalculateTrustStatus(verification, ctx.Repo.Repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
			return repo_model.IsOwnerMemberCollaborator(ctx.Repo.Repository, user.ID)
		}, nil); err != nil {
			ctx.ServerError("CalculateTrustStatus", err)
			return nil
		}
		ctx.Data["LatestCommitVerification"] = verification
		ctx.Data["LatestCommitUser"] = user_model.ValidateCommitWithEmail(ctx, latestCommit)

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
	ctx.Data["RepositoryUploadEnabled"] = setting.Repository.Upload.Enabled

	if ctx.Repo.Commit == nil || ctx.Repo.Repository.IsEmpty || ctx.Repo.Repository.IsBroken() {
		showEmpty := true
		var err error
		if ctx.Repo.GitRepo != nil {
			showEmpty, err = ctx.Repo.GitRepo.IsEmpty()
			if err != nil {
				log.Error("GitRepo.IsEmpty: %v", err)
				ctx.Repo.Repository.Status = repo_model.RepositoryBroken
				showEmpty = true
				ctx.Flash.Error(ctx.Tr("error.occurred"), true)
			}
		}
		if showEmpty {
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

		// the repo's IsEmpty has been updated, redirect to this page to make sure middlewares can get the correct values
		link := ctx.Link
		if ctx.Req.URL.RawQuery != "" {
			link += "?" + ctx.Req.URL.RawQuery
		}
		ctx.Redirect(link)
		return
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

	checkCitationFile(ctx, entry)
	if ctx.Written() {
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
	ctx.Data["Title"] = ctx.Tr("repo.forks")

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
		if err = fork.LoadOwner(ctx); err != nil {
			ctx.ServerError("LoadOwner", err)
			return
		}
	}

	ctx.Data["Forks"] = forks

	ctx.HTML(http.StatusOK, tplForks)
}
