// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

type FileInfo struct {
	isTextFile bool
	isLFSFile  bool
	fileSize   int64
	lfsMeta    *lfs.Pointer
	St         typesniffer.SniffedType
}

func (f *FileInfo) IsTextFile() bool {
	return f.isTextFile
}

func (f *FileInfo) IsLFSFile() bool {
	return f.isLFSFile
}

func (f *FileInfo) FileSize() int64 {
	return f.fileSize
}

func GetFileReader(repoID int64, blob *git.Blob) ([]byte, io.ReadCloser, *FileInfo, error) {
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
		return buf, dataRc, &FileInfo{isTextFile, false, blob.Size(), nil, st}, nil
	}

	pointer, _ := lfs.ReadPointerFromBuffer(buf)
	if !pointer.IsValid() { // fallback to plain file
		return buf, dataRc, &FileInfo{isTextFile, false, blob.Size(), nil, st}, nil
	}

	meta, err := git_model.GetLFSMetaObjectByOid(db.DefaultContext, repoID, pointer.Oid)
	if err != nil && err != git_model.ErrLFSObjectNotExist { // fallback to plain file
		return buf, dataRc, &FileInfo{isTextFile, false, blob.Size(), nil, st}, nil
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

	return buf, dataRc, &FileInfo{st.IsText(), true, meta.Size, &meta.Pointer, st}, nil
}

func MarkupRender(ctx *context.Context, renderCtx *markup.RenderContext, input io.Reader) (escaped *charset.EscapeStatus, output string, err error) {
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

// locate a README for a tree in one of the supported paths.
//
// entries is passed to reduce calls to ListEntries(), so
// this has precondition:
//
//	entries == ctx.Repo.Commit.SubTree(ctx.Repo.TreePath).ListEntries()
//
// FIXME: There has to be a more efficient way of doing this
func FindReadmeFileInEntries(ctx *context.Context, entries []*git.TreeEntry, tryWellKnownDirs bool) (string, *git.TreeEntry, error) {
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

			subfolder, readmeFile, err := FindReadmeFileInEntries(ctx, childEntries, false)
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

func ReadmeFile(ctx *context.Context, subfolder string, readmeFile *git.TreeEntry, readmeTreelink string) {
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

	buf, dataRc, fInfo, err := GetFileReader(ctx.Repo.Repository.ID, target.Blob())
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

		ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = MarkupRender(ctx, &markup.RenderContext{
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
