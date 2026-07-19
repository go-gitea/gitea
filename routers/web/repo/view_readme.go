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
//
// entries is passed to reduce calls to ListEntries(), so
// this has precondition:
//
//	entries == ctx.Repo.Commit.SubTree(ctx.Repo.TreePath).ListEntries()
func findReadmeFileInEntries(ctx *context.Context, parentDir string, entries []*git.TreeEntry, tryWellKnownDirs bool) (string, *git.TreeEntry, error) {
	var giteaEntry, githubEntry, docsEntry *git.TreeEntry

	// Create a list of extensions in priority order
	// 1. Markdown files - with and without localisation - e.g. README.en-us.md or README.md
	// 2. Txt files - e.g. README.txt
	// 3. No extension - e.g. README
	exts := append(localizedExtensions(".md", ctx.Locale.Language()), ".txt", "") // sorted by priority
	extCount := len(exts)
	readmeFiles := make([]*git.TreeEntry, extCount+1)

	for _, entry := range entries {
		if entry.IsDir() {
			if tryWellKnownDirs {
				lowerName := strings.ToLower(entry.Name())
				switch lowerName {
				case ".gitea":
					if entry.Name() == ".gitea" || giteaEntry == nil {
						giteaEntry = entry
					}
				case ".github":
					if entry.Name() == ".github" || githubEntry == nil {
						githubEntry = entry
					}
				case "docs":
					if entry.Name() == "docs" || docsEntry == nil {
						docsEntry = entry
					}
				}
			}
			continue
		}

		if i, ok := util.IsReadmeFileExtension(entry.Name(), exts...); ok {
			fullPath := path.Join(parentDir, entry.Name())
			if readmeFiles[i] == nil || base.NaturalSortCompare(readmeFiles[i].Name(), entry.Blob(ctx.Repo.GitRepo).Name()) < 0 {
				if entry.IsLink() {
					res, err := git.EntryFollowLinks(ctx, ctx.Repo.GitRepo, ctx.Repo.Commit, fullPath, entry)
					if err == nil && (res.TargetEntry.IsExecutable() || res.TargetEntry.IsRegular()) {
						readmeFiles[i] = entry
					}
				} else {
					readmeFiles[i] = entry
				}
			}
		}
	}

	if ctx.Repo.TreePath == "" && tryWellKnownDirs {
		for _, subTreeEntry := range []*git.TreeEntry{giteaEntry, githubEntry} {
			if subTreeEntry == nil {
				continue
			}
			subTree := subTreeEntry.Tree(ctx, ctx.Repo.GitRepo)
			if subTree == nil {
				// this should be impossible; if subTreeEntry exists so should this.
				continue
			}
			childEntries, err := subTree.ListEntries(ctx, ctx.Repo.GitRepo)
			if err != nil {
				return "", nil, err
			}

			subfolder, readmeFile, err := findReadmeFileInEntries(ctx, path.Join(parentDir, subTreeEntry.Name()), childEntries, false)
			if err != nil && !git.IsErrNotExist(err) {
				return "", nil, err
			}
			if readmeFile != nil {
				return path.Join(subTreeEntry.Name(), subfolder), readmeFile, nil
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

	if ctx.Repo.TreePath == "" && tryWellKnownDirs && readmeFile == nil {
		if docsEntry != nil {
			subTree := docsEntry.Tree(ctx, ctx.Repo.GitRepo)
			if subTree != nil {
				childEntries, err := subTree.ListEntries(ctx, ctx.Repo.GitRepo)
				if err != nil {
					return "", nil, err
				}

				subfolder, readmeFile, err := findReadmeFileInEntries(ctx, path.Join(parentDir, docsEntry.Name()), childEntries, false)
				if err != nil && !git.IsErrNotExist(err) {
					return "", nil, err
				}
				if readmeFile != nil {
					return path.Join(docsEntry.Name(), subfolder), readmeFile, nil
				}
			}
		}
	}

	return "", readmeFile, nil
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
