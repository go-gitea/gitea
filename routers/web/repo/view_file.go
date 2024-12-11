// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"path"
	"slices"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/actions"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	issue_service "code.gitea.io/gitea/services/issue"
	files_service "code.gitea.io/gitea/services/repository/files"

	"github.com/nektos/act/pkg/model"
)

func prepareToRenderFile(ctx *context.Context, entry *git.TreeEntry) {
	ctx.Data["IsViewFile"] = true
	ctx.Data["HideRepoInfo"] = true
	blob := entry.Blob()
	buf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, blob)
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["Title"] = ctx.Tr("repo.file.title", ctx.Repo.Repository.Name+"/"+path.Base(ctx.Repo.TreePath), ctx.Repo.RefName)
	ctx.Data["FileIsSymlink"] = entry.IsLink()
	ctx.Data["FileName"] = blob.Name()
	ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL() + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

	commit, err := ctx.Repo.Commit.GetCommitByPath(ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetCommitByPath", err)
		return
	}

	if !loadLatestCommitData(ctx, commit) {
		return
	}

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
	} else if slices.Contains([]string{"CODEOWNERS", "docs/CODEOWNERS", ".gitea/CODEOWNERS"}, ctx.Repo.TreePath) {
		if data, err := blob.GetBlobContent(setting.UI.MaxDisplayFileSize); err == nil {
			_, warnings := issue_model.GetCodeOwnersFromContent(ctx, data)
			if len(warnings) > 0 {
				ctx.Data["FileWarning"] = strings.Join(warnings, "\n")
			}
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
	ctx.Data["IsExecutable"] = entry.IsExecutable()

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
		if fInfo.fileSize >= setting.UI.MaxDisplayFileSize {
			ctx.Data["IsFileTooLarge"] = true
			break
		}

		if fInfo.st.IsSvgImage() {
			ctx.Data["IsImageFile"] = true
			ctx.Data["CanCopyContent"] = true
			ctx.Data["HasSourceRenderedToggle"] = true
		}

		rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})

		shouldRenderSource := ctx.FormString("display") == "source"
		readmeExist := util.IsReadmeFileName(blob.Name())
		ctx.Data["ReadmeExist"] = readmeExist

		markupType := markup.DetectMarkupTypeByFileName(blob.Name())
		if markupType == "" {
			markupType = markup.DetectRendererType(blob.Name(), bytes.NewReader(buf))
		}
		if markupType != "" {
			ctx.Data["HasSourceRenderedToggle"] = true
		}
		if markupType != "" && !shouldRenderSource {
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType
			metas := ctx.Repo.Repository.ComposeDocumentMetas(ctx)
			metas["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
			rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
				CurrentRefPath:  ctx.Repo.BranchNameSubURL(),
				CurrentTreePath: path.Dir(ctx.Repo.TreePath),
			}).
				WithMarkupType(markupType).
				WithRelativePath(ctx.Repo.TreePath).
				WithMetas(metas)

			ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, rctx, rd)
			if err != nil {
				ctx.ServerError("Render", err)
				return
			}
			// to prevent iframe load third-party url
			ctx.Resp.Header().Add("Content-Security-Policy", "frame-src 'self'")
		} else {
			buf, _ := io.ReadAll(rd)

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

			language, err := files_service.TryGetContentLanguage(ctx.Repo.GitRepo, ctx.Repo.CommitID, ctx.Repo.TreePath)
			if err != nil {
				log.Error("Unable to get file language for %-v:%s. Error: %v", ctx.Repo.Repository, ctx.Repo.TreePath, err)
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
			if ctx.Repo.CanEnableEditor(ctx, ctx.Doer) {
				if lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID {
					ctx.Data["CanEditFile"] = false
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
				} else {
					ctx.Data["CanEditFile"] = true
					ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.edit_this_file")
				}
			} else if !ctx.Repo.IsViewBranch {
				ctx.Data["EditFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
			} else if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName) {
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

		// TODO: this logic duplicates with "isRepresentableAsText=true", it is not the same as "LFSFileGet" in "lfs.go"
		// It is used by "external renders", markupRender will execute external programs to get rendered content.
		if markupType := markup.DetectMarkupTypeByFileName(blob.Name()); markupType != "" {
			rd := io.MultiReader(bytes.NewReader(buf), dataRc)
			ctx.Data["IsMarkup"] = true
			ctx.Data["MarkupType"] = markupType

			rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
				CurrentRefPath:  ctx.Repo.BranchNameSubURL(),
				CurrentTreePath: path.Dir(ctx.Repo.TreePath),
			}).
				WithMarkupType(markupType).
				WithRelativePath(ctx.Repo.TreePath)

			ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRender(ctx, rctx, rd)
			if err != nil {
				ctx.ServerError("Render", err)
				return
			}
		}
	}

	if ctx.Repo.GitRepo != nil {
		checker, deferable := ctx.Repo.GitRepo.CheckAttributeReader(ctx.Repo.CommitID)
		if checker != nil {
			defer deferable()
			attrs, err := checker.CheckPath(ctx.Repo.TreePath)
			if err == nil {
				ctx.Data["IsVendored"] = git.AttributeToBool(attrs, git.AttributeLinguistVendored).Value()
				ctx.Data["IsGenerated"] = git.AttributeToBool(attrs, git.AttributeLinguistGenerated).Value()
			}
		}
	}

	if fInfo.st.IsImage() && !fInfo.st.IsSvgImage() {
		img, _, err := image.DecodeConfig(bytes.NewReader(buf))
		if err == nil {
			// There are Image formats go can't decode
			// Instead of throwing an error in that case, we show the size only when we can decode
			ctx.Data["ImageSize"] = fmt.Sprintf("%dx%dpx", img.Width, img.Height)
		}
	}

	if ctx.Repo.CanEnableEditor(ctx, ctx.Doer) {
		if lfsLock != nil && lfsLock.OwnerID != ctx.Doer.ID {
			ctx.Data["CanDeleteFile"] = false
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.this_file_locked")
		} else {
			ctx.Data["CanDeleteFile"] = true
			ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.delete_this_file")
		}
	} else if !ctx.Repo.IsViewBranch {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_be_on_a_branch")
	} else if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, ctx.Repo.BranchName) {
		ctx.Data["DeleteFileTooltip"] = ctx.Tr("repo.editor.must_have_write_access")
	}
}
