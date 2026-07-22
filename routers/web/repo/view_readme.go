// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"path"
	"strings"

	"gitea.dev/models/renderhelper"
	"gitea.dev/modules/base"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

// locate a README for a tree in one of the supported paths.
// entries are passed to reduce calls to ListEntries(), so this has precondition:
//
//	entries == ctx.Repo.Commit.SubTree(ctx.Repo.TreePath).ListEntries()
//
// this function is tested by integration test ViewRepoDirectoryReadme
func findReadmeFileInRepoTree(ctx *context.Context, treePath string, tree *git.TreeEntry, rootSubEntries []*git.TreeEntry) (subFolder string, _ *git.TreeEntry, err error) {
	gitRepo := ctx.Repo.GitRepo
	var dirEntries []*git.TreeEntry
	if treePath == "" {
		// only try the special sub-folders when visiting the repo root
		wellKnownSubDirs := findReadmeWellKnownSubDirs(rootSubEntries)
		dirEntries = []*git.TreeEntry{wellKnownSubDirs.entryGitea, wellKnownSubDirs.entryGitHub, tree, wellKnownSubDirs.entryDocs}
	} else {
		dirEntries = []*git.TreeEntry{tree}
	}

	for _, dirEntry := range dirEntries {
		if dirEntry == nil {
			continue
		}

		var dirSubEntries []*git.TreeEntry
		if dirEntry == tree {
			subFolder, dirSubEntries = "", rootSubEntries
		} else {
			subFolder = dirEntry.Name()
			dirSubEntries, err = dirEntry.Tree(ctx, gitRepo).ListEntries(ctx, gitRepo)
			if err != nil {
				return "", nil, err
			}
		}
		found := findReadmeFileInEntries(ctx, path.Join(treePath, subFolder), dirSubEntries)
		if found != nil {
			return subFolder, found, nil
		}
	}
	return "", nil, nil
}

func findReadmeWellKnownSubDirs(entries []*git.TreeEntry) (ret struct{ entryGitea, entryGitHub, entryDocs *git.TreeEntry }) {
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lowerName := strings.ToLower(entry.Name())
		switch lowerName {
		case ".gitea":
			if entry.Name() == ".gitea" || ret.entryGitea == nil {
				ret.entryGitea = entry
			}
		case ".github":
			if entry.Name() == ".github" || ret.entryGitHub == nil {
				ret.entryGitHub = entry
			}
		case "docs":
			if entry.Name() == "docs" || ret.entryDocs == nil {
				ret.entryDocs = entry
			}
		}
	}
	return ret
}

func findReadmeFileInEntries(ctx *context.Context, parentPath string, entries []*git.TreeEntry) *git.TreeEntry {
	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localization - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", ctx.Locale.Language()), ".txt", "") // sorted by priority
	extCount := len(exts)
	readmeFiles := make([]*git.TreeEntry, extCount+1) // ext weight can be len(exts), so here "+1"

	for _, entry := range entries {
		extWeight, ok := util.IsReadmeFileExtension(entry.Name(), exts...)
		if !ok {
			continue
		}
		fullPath := path.Join(parentPath, entry.Name())
		if readmeFiles[extWeight] == nil || base.NaturalSortCompare(readmeFiles[extWeight].Name(), entry.Blob(ctx.Repo.GitRepo).Name()) < 0 {
			if entry.IsLink() {
				res, err := git.EntryFollowLinks(ctx, ctx.Repo.GitRepo, ctx.Repo.Commit, fullPath, entry)
				if err == nil && (res.TargetEntry.IsExecutable() || res.TargetEntry.IsRegular()) {
					readmeFiles[extWeight] = entry
				}
			} else {
				readmeFiles[extWeight] = entry
			}
		}
	}
	for _, f := range readmeFiles {
		if f != nil {
			return f
		}
	}
	return nil
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

func prepareToRenderReadmeFile(ctx *context.Context, subfolder string, readmeFile *git.TreeEntry) {
	if readmeFile == nil {
		return
	}

	readmeFullPath := path.Join(ctx.Repo.TreePath, subfolder, readmeFile.Name())
	readmeTargetEntry := readmeFile
	if readmeFile.IsLink() {
		if res, err := git.EntryFollowLinks(ctx, ctx.Repo.GitRepo, ctx.Repo.Commit, readmeFullPath, readmeFile); err == nil {
			readmeTargetEntry = res.TargetEntry
		} else {
			readmeTargetEntry = nil // if we cannot resolve the symlink, we cannot render the readme, ignore the error
		}
	}
	if readmeTargetEntry == nil {
		return // if no valid README entry found, skip rendering the README
	}

	ctx.Data["RawFileLink"] = ""
	ctx.Data["ReadmeInList"] = path.Join(subfolder, readmeFile.Name()) // the relative path to the readme file to the current tree path
	ctx.Data["ReadmeExist"] = true
	ctx.Data["FileIsSymlink"] = readmeFile.IsLink()

	buf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, readmeTargetEntry.Blob(ctx.Repo.GitRepo))
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["FileIsText"] = fInfo.st.IsText()
	ctx.Data["FileTreePath"] = readmeFullPath
	ctx.Data["FileSize"] = fInfo.blobOrLfsSize
	ctx.Data["IsLFSFile"] = fInfo.isLFSFile()

	if fInfo.isLFSFile() {
		filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(readmeFile.Name()))
		ctx.Data["RawFileLink"] = fmt.Sprintf("%s.git/info/lfs/objects/%s/%s", ctx.Repo.Repository.Link(), url.PathEscape(fInfo.lfsMeta.Oid), url.PathEscape(filenameBase64))
	}

	if !fInfo.st.IsText() {
		return
	}

	if fInfo.blobOrLfsSize >= setting.UI.MaxDisplayFileSize {
		// Pretend that this is a normal text file to display 'This file is too large to be shown'
		ctx.Data["IsFileTooLarge"] = true
		return
	}

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})

	rctx := renderhelper.NewRenderContextRepoFile(ctx, ctx.Repo.Repository, renderhelper.RepoFileOptions{
		CurrentRefSubURL: ctx.Repo.RefTypeNameSubURL(),
		CurrentTreePath:  path.Dir(readmeFullPath),
	}).WithRelativePath(readmeFullPath)
	renderer := rctx.DetectMarkupRenderer(buf)
	if renderer != nil {
		ctx.Data["RenderAsMarkup"] = "markup-inplace"
		ctx.Data["MarkupType"] = rctx.RenderOptions.MarkupType
		ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRenderToHTML(ctx, rctx, renderer, rd)
		if err != nil {
			log.Error("Render failed for %s in %-v: %v Falling back to rendering source", readmeFile.Name(), ctx.Repo.Repository, err)
			delete(ctx.Data, "RenderAsMarkup")
		}
	}

	if ctx.Data["RenderAsMarkup"] == nil {
		ctx.Data["IsPlainText"] = true
		content, err := io.ReadAll(rd)
		if err != nil {
			log.Error("Read readme content failed: %v", err)
		}
		contentEscaped := template.HTMLEscapeString(util.UnsafeBytesToString(content))
		ctx.Data["EscapeStatus"], ctx.Data["FileContent"] = charset.EscapeControlHTML(template.HTML(contentEscaped), ctx.Locale)
	}

	if !fInfo.isLFSFile() && ctx.Repo.Repository.CanEnableEditor() {
		ctx.Data["CanEditReadmeFile"] = true
	}
}
