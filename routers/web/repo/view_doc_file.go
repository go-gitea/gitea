// Copyright 2026 The Gitea Authors. All rights reserved.
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

type docKind struct {
	ID        string   // stable query value, e.g. "contributing"
	LabelKey  string   // locale key, e.g. "repo.tab_contributing"
	Icon      string   // octicon name, e.g. "octicon-book"
	Stems     []string // case-insensitive filename stems
	Localized bool     // apply localizedExtensions (README only)
}

func (rk docKind) Extensions(lang string) []string {
	if rk.Localized {
		return append(localizedExtensions(".md", lang), ".txt", "")
	}
	return []string{".md", ".txt", ""}
}

var docKinds = []docKind{
	{ID: "readme", LabelKey: "repo.tab_readme", Icon: "octicon-book", Stems: []string{"readme"}, Localized: true},
	{ID: "code-of-conduct", LabelKey: "repo.tab_code_of_conduct", Icon: "octicon-code-of-conduct", Stems: []string{"code_of_conduct", "code-of-conduct"}},
	{ID: "contributing", LabelKey: "repo.tab_contributing", Icon: "octicon-git-pull-request", Stems: []string{"contributing"}},
	{ID: "license", LabelKey: "repo.tab_license", Icon: "octicon-law", Stems: []string{"license", "licence", "copying"}},
	{ID: "security", LabelKey: "repo.tab_security", Icon: "octicon-shield-check", Stems: []string{"security"}},
}

// docFile is a doc kind matched to the file found for it.
type docFile struct {
	Kind      *docKind       // the kind of doc file, e.g. README
	SubFolder string         // the sub-folder in which the file was found, e.g. ".gitea"
	Entry     *git.TreeEntry // the file entry found for this doc kind
}

// docTab is a doc file rendered as a tab.
type docTab struct {
	Kind   *docKind // the doc kind, e.g. "readme"
	Path   string   // the file path relative to the repo root
	Link   string   // the link to this tab, e.g. "?doc-tab=readme"
	Active bool     // whether this tab is the currently selected one
}

// findDocFilesInRepoTree locates at most one file per doc kind, trying the
// well-known sub-folders first when visiting the repo root. Subdirectories only
// search for the README. Navigation entries are passed to reduce calls to
// ListEntries(), so this has precondition:
//
//	rootSubEntries == ctx.Repo.Commit.SubTree(ctx.Repo.TreePath).ListEntries()
//
// tested by integration tests ViewRepoDirectoryReadme and ViewRepoDirectoryDocTabs
func findDocFilesInRepoTree(ctx *context.Context, treePath string, tree *git.TreeEntry, rootSubEntries []*git.TreeEntry) ([]docFile, error) {
	gitRepo := ctx.Repo.GitRepo
	var dirEntries []*git.TreeEntry
	if treePath == "" {
		// only try the special sub-folders when visiting the repo root
		wellKnownSubDirs := findDocWellKnownSubDirs(rootSubEntries)
		dirEntries = []*git.TreeEntry{wellKnownSubDirs.entryGitea, wellKnownSubDirs.entryGitHub, tree, wellKnownSubDirs.entryDocs}
	} else {
		dirEntries = []*git.TreeEntry{tree}
	}

	// list each directory lazily and only once, the kinds may need different directories
	entriesCache := make(map[*git.TreeEntry][]*git.TreeEntry, len(dirEntries))
	listDir := func(dirEntry *git.TreeEntry) ([]*git.TreeEntry, error) {
		if dirEntry == tree {
			return rootSubEntries, nil
		}
		if entries, ok := entriesCache[dirEntry]; ok {
			return entries, nil
		}
		entries, err := dirEntry.Tree(ctx, gitRepo).ListEntries(ctx, gitRepo)
		if err != nil {
			return nil, err
		}
		entriesCache[dirEntry] = entries
		return entries, nil
	}

	found := make([]docFile, 0, len(docKinds))
	for i := range docKinds {
		kind := &docKinds[i]
		if treePath != "" && kind.ID != "readme" {
			continue // only the README is shown in subdirectories
		}
		for _, dirEntry := range dirEntries {
			if dirEntry == nil {
				continue
			}
			entries, err := listDir(dirEntry)
			if err != nil {
				return nil, err
			}
			subFolder := ""
			if dirEntry != tree {
				subFolder = dirEntry.Name()
			}
			if entry := findDocFileInEntries(ctx, path.Join(treePath, subFolder), entries, kind); entry != nil {
				found = append(found, docFile{Kind: kind, SubFolder: subFolder, Entry: entry})
				break
			}
		}
	}
	return found, nil
}

func findDocWellKnownSubDirs(entries []*git.TreeEntry) (ret struct{ entryGitea, entryGitHub, entryDocs *git.TreeEntry }) {
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

func findDocFileInEntries(ctx *context.Context, parentPath string, entries []*git.TreeEntry, kind *docKind) *git.TreeEntry {
	exts := kind.Extensions(ctx.Locale.Language())
	bestWeight := len(exts) + 1
	var bestEntry *git.TreeEntry

	for _, entry := range entries {
		weight, ok := util.MatchFileStem(entry.Name(), kind.Stems, exts)
		if !ok || weight > bestWeight {
			continue
		}
		if weight < bestWeight || (base.NaturalSortCompare(bestEntry.Name(), entry.Name()) < 0) {
			fullPath := path.Join(parentPath, entry.Name())
			if entry.IsLink() {
				res, err := git.EntryFollowLinks(ctx, ctx.Repo.GitRepo, ctx.Repo.Commit, fullPath, entry)
				if err != nil || !(res.TargetEntry.IsExecutable() || res.TargetEntry.IsRegular()) {
					continue
				}
			}
			bestWeight = weight
			bestEntry = entry
		}
	}

	return bestEntry
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

// prepareToRenderDocView builds the doc tabs and renders the selected doc file.
// docKinds order puts README first, so it is the default when nothing else is requested.
func prepareToRenderDocView(ctx *context.Context, docFiles []docFile) {
	if len(docFiles) == 0 {
		return
	}

	if ctx.Repo.TreePath != "" {
		// subdirectories show the README file name in the header instead of tabs
		for i := range docFiles {
			if docFiles[i].Kind.ID == "readme" {
				ctx.Data["ReadmeExist"] = true
				ctx.Data["ReadmePath"] = path.Join(ctx.Repo.TreePath, docFiles[i].SubFolder, docFiles[i].Entry.Name())
				prepareToRenderDocFile(ctx, &docFiles[i])
				return
			}
		}
		return
	}

	requestedID := ctx.FormString("doc-tab")
	selected := 0
	for i := range docFiles {
		if docFiles[i].Kind.ID == requestedID {
			selected = i
			break
		}
	}

	tabQuery := ctx.Req.URL.Query()
	tabQuery.Del("only_content") // never leak the fragment-only flag into tab links
	tabs := make([]docTab, 0, len(docFiles))
	for i := range docFiles {
		kind := docFiles[i].Kind
		if kind.ID == "readme" {
			ctx.Data["ReadmeExist"] = true
			ctx.Data["ReadmePath"] = path.Join(ctx.Repo.TreePath, docFiles[i].SubFolder, docFiles[i].Entry.Name())
		}
		tabQuery.Set("doc-tab", kind.ID)
		tabs = append(tabs, docTab{
			Kind:   kind,
			Path:   path.Join(docFiles[i].SubFolder, docFiles[i].Entry.Name()),
			Link:   ctx.Link + "?" + tabQuery.Encode(),
			Active: i == selected,
		})
	}
	ctx.Data["DocTabs"] = tabs

	prepareToRenderDocFile(ctx, &docFiles[selected])
}

func prepareToRenderDocFile(ctx *context.Context, doc *docFile) {
	if doc == nil || doc.Kind == nil || doc.Entry == nil {
		return
	}

	entry, subfolder := doc.Entry, doc.SubFolder
	fullPath := path.Join(ctx.Repo.TreePath, subfolder, entry.Name())
	targetEntry := entry
	if entry.IsLink() {
		if res, err := git.EntryFollowLinks(ctx, ctx.Repo.GitRepo, ctx.Repo.Commit, fullPath, entry); err == nil {
			targetEntry = res.TargetEntry
		} else {
			targetEntry = nil // if we cannot resolve the symlink, we cannot render the doc file, ignore the error
		}
	}
	if targetEntry == nil {
		return // if no valid doc file entry found, skip rendering
	}

	ctx.Data["RawFileLink"] = ""
	ctx.Data["DocFileInList"] = path.Join(subfolder, entry.Name()) // the relative path to the doc file to the current tree path
	ctx.Data["FileIsSymlink"] = entry.IsLink()

	buf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, targetEntry.Blob(ctx.Repo.GitRepo))
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	ctx.Data["FileIsText"] = fInfo.st.IsText()
	ctx.Data["FileTreePath"] = fullPath
	ctx.Data["FileSize"] = fInfo.blobOrLfsSize
	ctx.Data["IsLFSFile"] = fInfo.isLFSFile()

	if fInfo.isLFSFile() {
		filenameBase64 := base64.RawURLEncoding.EncodeToString([]byte(entry.Name()))
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
		CurrentTreePath:  path.Dir(fullPath),
	}).WithRelativePath(fullPath)
	renderer := rctx.DetectMarkupRenderer(buf)
	if renderer != nil {
		ctx.Data["RenderAsMarkup"] = "markup-inplace"
		ctx.Data["MarkupType"] = rctx.RenderOptions.MarkupType
		ctx.Data["EscapeStatus"], ctx.Data["FileContent"], err = markupRenderToHTML(ctx, rctx, renderer, rd)
		if err != nil {
			log.Error("Render failed for %s in %-v: %v Falling back to rendering source", entry.Name(), ctx.Repo.Repository, err)
			delete(ctx.Data, "RenderAsMarkup")
		}
	}

	if ctx.Data["RenderAsMarkup"] == nil {
		ctx.Data["IsPlainText"] = true
		content, err := io.ReadAll(rd)
		if err != nil {
			log.Error("Read doc file content failed: %v", err)
		}
		contentEscaped := template.HTMLEscapeString(util.UnsafeBytesToString(content))
		ctx.Data["EscapeStatus"], ctx.Data["FileContent"] = charset.EscapeControlHTML(template.HTML(contentEscaped), ctx.Locale)
	}

	if !fInfo.isLFSFile() && ctx.Repo.Repository.CanContentChange() {
		ctx.Data["CanEditDocFile"] = true
	}
}
