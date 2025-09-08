// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"path"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/attribute"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	issue_service "code.gitea.io/gitea/services/issue"

	"github.com/nektos/act/pkg/model"
)

func prepareLatestCommitInfo(ctx *context.Context) bool {
	commit, err := ctx.Repo.Commit.GetCommitByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return false
	}

	return loadLatestCommitData(ctx, commit)
}

func prepareFileViewLfsAttrs(ctx *context.Context) (*attribute.Attributes, bool) {
	attrsMap, err := attribute.CheckAttributes(ctx, ctx.Repo.GitRepo, ctx.Repo.CommitID, attribute.CheckAttributeOpts{
		Filenames:  []string{ctx.Repo.TreePath},
		Attributes: []string{attribute.LinguistGenerated, attribute.LinguistVendored, attribute.LinguistLanguage, attribute.GitlabLanguage},
	})
	if err != nil {
		ctx.ServerError("attribute.CheckAttributes", err)
		return nil, false
	}
	attrs := attrsMap[ctx.Repo.TreePath]
	if attrs == nil {
		// this case shouldn't happen, just in case.
		setting.PanicInDevOrTesting("no attributes found for %s", ctx.Repo.TreePath)
		attrs = attribute.NewAttributes()
	}
	ctx.Data["IsVendored"], ctx.Data["IsGenerated"] = attrs.GetVendored().Value(), attrs.GetGenerated().Value()
	return attrs, true
}

func handleFileViewRenderMarkup(ctx *context.Context, filename string, sniffedType typesniffer.SniffedType, prefetchBuf []byte, utf8Reader io.Reader) bool {
	markupType := markup.DetectMarkupTypeByFileName(filename)
	if markupType == "" {
		markupType = markup.DetectRendererType(filename, sniffedType, prefetchBuf)
	}
	if markupType == "" {
		return false
	}

	ctx.Data["HasSourceRenderedToggle"] = true

	if ctx.FormString("display") == "source" {
		return false
	}

	ctx.Data["MarkupType"] = markupType
	metas := ctx.Repo.Repository.ComposeRepoFileMetas(ctx)
	metas["RefTypeNameSubURL"] = ctx.Repo.RefTypeNameSubURL()
	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefPath:  ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath: path.Dir(ctx.Repo.TreePath),
	}).
		WithMarkupType(markupType).
		WithRelativePath(ctx.Repo.TreePath).
		WithMetas(metas)

	var err error
	ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, rctx, utf8Reader)
	if err != nil {
		ctx.ServerError("Render", err)
		return true
	}
	// to prevent iframe from loading third-party url
	ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
	return true
}

func handleFileViewRenderSource(ctx *context.Context, filename string, attrs *attribute.Attributes, fInfo *fileInfo, utf8Reader io.Reader) bool {
	if ctx.FormString("display") == "rendered" || !fInfo.st.IsRepresentableAsText() {
		return false
	}

	if !fInfo.st.IsText() {
		if ctx.FormString("display") == "" {
			// not text but representable as text, e.g. SVG
			// since there is no "display" is specified, let other renders to handle
			return false
		}
		ctx.Data["HasSourceRenderedToggle"] = true
	}

	buf, _ := io.ReadAll(utf8Reader)
	// The Open Group Base Specification: https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap03.html
	//   empty: 0 lines; "a": 1 incomplete-line; "a\n": 1 line; "a\nb": 1 line, 1 incomplete-line;
	// Gitea uses the definition (like most modern editors):
	//   empty: 0 lines; "a": 1 line; "a\n": 2 lines; "a\nb": 2 lines;
	//   When rendering, the last empty line is not rendered in UI, while the line-number is still counted, to tell users that the file contains a trailing EOL.
	//   To make the UI more consistent, it could use an icon mark to indicate that there is no trailing EOL, and show line-number as the rendered lines.
	// This NumLines is only used for the display on the UI: "xxx lines"
	if len(buf) == 0 {
		ctx.Data["NumLines"] = 0
	} else {
		ctx.Data["NumLines"] = bytes.Count(buf, []byte{'\n'}) + 1
	}

	language := attrs.GetLanguage().Value()
	fileContent, lexerName, err := highlight.File(filename, language, buf)
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
	return true
}

func handleFileViewRenderImage(ctx *context.Context, fInfo *fileInfo, prefetchBuf []byte) bool {
	if !fInfo.st.IsImage() {
		return false
	}
	if fInfo.st.IsSvgImage() && !setting.UI.SVG.Enabled {
		return false
	}
	if fInfo.st.IsSvgImage() {
		ctx.Data["HasSourceRenderedToggle"] = true
	} else {
		img, _, err := image.DecodeConfig(bytes.NewReader(prefetchBuf))
		if err == nil { // ignore the error for the formats that are not supported by image.DecodeConfig
			ctx.Data["ImageSize"] = fmt.Sprintf("%dx%dpx", img.Width, img.Height)
		}
	}
	return true
}

func prepareFileView(ctx *context.Context, entry *git.TreeEntry) {
	ctx.Data["IsViewFile"] = true
	ctx.Data["HideRepoInfo"] = true

	if !prepareLatestCommitInfo(ctx) {
		return
	}

	blob := entry.Blob()

	ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefFullName.ShortName())
	ctx.Data["FileIsSymlink"] = entry.IsLink()
	ctx.Data["FileTreePath"] = ctx.Repo.TreePath
	ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/raw/" + ctx.Repo.RefTypeNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

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
	} else if issue_service.IsCodeOwnerFile(ctx.Repo.TreePath) {
		if data, err := blob.GetBlobContent(setting.UI.MaxDisplayFileSize); err == nil {
			_, warnings := issue_model.GetCodeOwnersFromContent(ctx, data)
			if len(warnings) > 0 {
				ctx.Data["FileWarning"] = strings.Join(warnings, "\n")
			}
		}
	}

	// Don't call any other repository functions depends on git.Repository until the dataRc closed to
	// avoid creating an unnecessary temporary cat file.
	buf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, blob)
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	if fInfo.isLFSFile() {
		ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/media/" + ctx.Repo.RefTypeNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	if !prepareFileViewEditorButtons(ctx) {
		return
	}

	ctx.Data["IsLFSFile"] = fInfo.isLFSFile()
	ctx.Data["FileSize"] = fInfo.fileSize
	ctx.Data["IsRepresentableAsText"] = fInfo.st.IsRepresentableAsText()
	ctx.Data["IsExecutable"] = entry.IsExecutable()
	ctx.Data["CanCopyContent"] = fInfo.st.IsRepresentableAsText() || fInfo.st.IsImage()

	attrs, ok := prepareFileViewLfsAttrs(ctx)
	if !ok {
		return
	}

	// TODO: in the future maybe we need more accurate flags, for example:
	// * IsRepresentableAsText: some files are text, some are not
	// * IsRenderableXxx: some files are rendered by backend "markup" engine, some are rendered by frontend (pdf, 3d)
	// * DefaultViewMode: when there is no "display" query parameter, which view mode should be used by default, source or rendered

	utf8Reader := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
	switch {
	case fInfo.fileSize >= setting.UI.MaxDisplayFileSize:
		ctx.Data["IsFileTooLarge"] = true
	case handleFileViewRenderMarkup(ctx, entry.Name(), fInfo.st, buf, utf8Reader):
		// it also sets ctx.Data["FileContent"] and more
		ctx.Data["IsMarkup"] = true
	case handleFileViewRenderSource(ctx, entry.Name(), attrs, fInfo, utf8Reader):
		// it also sets ctx.Data["FileContent"] and more
		ctx.Data["IsDisplayingSource"] = true
	case handleFileViewRenderImage(ctx, fInfo, buf):
		ctx.Data["IsImageFile"] = true
	case fInfo.st.IsVideo():
		ctx.Data["IsVideoFile"] = true
	case fInfo.st.IsAudio():
		ctx.Data["IsAudioFile"] = true
	default:
		// unable to render anything, show the "view raw" or let frontend handle it
	}
}

func prepareFileViewEditorButtons(ctx *context.Context) bool {
	// archived or mirror repository, the buttons should not be shown
	if !ctx.Repo.Repository.CanEnableEditor() {
		return true
	}

	// The buttons should not be shown if it's not a branch
	if !ctx.Repo.RefFullName.IsBranch() {
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
		return true
	}

	if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName) {
		ctx.Data["CanEditFile"] = true
		ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.fork_before_edit")
		ctx.Data["CanDeleteFile"] = true
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
		return true
	}

	lfsLock, err := git_model.GetTreePathLock(ctx, ctx.Repo.Repository.ID, ctx.Repo.TreePath)
	ctx.Data["LFSLock"] = lfsLock
	if err != nil {
		ctx.ServerError("GetTreePathLock", err)
		return false
	}
	if lfsLock != nil {
		u, err := user_model.GetUserByID(ctx, lfsLock.OwnerID)
		if err != nil {
			ctx.ServerError("GetTreePathLock", err)
			return false
		}
		ctx.Data["LFSLockOwner"] = u.Name
		ctx.Data["LFSLockOwnerHomeLink"] = u.HomeLink()
		ctx.Data["LFSLockHint"] = ctx.Tr("repo.editor.this_file_locked")
	}

	// it's a lfs file and the user is not the owner of the lock
	isLFSLocked := lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID
	ctx.Data["CanEditFile"] = !isLFSLocked
	ctx.Data["EditFileTooltip"] = util.Iif(isLFSLocked, ctx.Tr("repo.editor.this_file_locked"), ctx.Tr("repo.editor.edit_this_file"))
	ctx.Data["CanDeleteFile"] = !isLFSLocked
	ctx.Data["DeleteFileTooltip"] = util.Iif(isLFSLocked, ctx.Tr("repo.editor.this_file_locked"), ctx.Tr("repo.editor.delete_this_file"))
	return true
}
