// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"html/template"
	"strings"

	"gitea.dev/models/perm/access"
	"gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/languagestats"
	"gitea.dev/modules/indexer/code"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/gitdiff"
)

func renderRepoFileCodePreview(ctx context.Context, opts markup.RenderCodePreviewOptions) (template.HTML, error) {
	opts.LineStop = max(opts.LineStop, opts.LineStart)
	lineCount := opts.LineStop - opts.LineStart + 1
	if lineCount <= 0 || lineCount > 140 /* GitHub at most show 140 lines */ {
		lineCount = 10
		opts.LineStop = opts.LineStart + lineCount
	}

	dbRepo, err := repo.GetRepositoryByOwnerAndName(ctx, opts.OwnerName, opts.RepoName)
	if err != nil {
		return "", err
	}

	webCtx := gitea_context.GetWebContext(ctx)
	if webCtx == nil {
		return "", errors.New("context is not a web context")
	}
	doer := webCtx.Doer

	perms, err := access.GetDoerRepoPermission(ctx, dbRepo, doer)
	if err != nil {
		return "", err
	}
	if !perms.CanRead(unit.TypeCode) {
		return "", util.ErrPermissionDenied
	}

	gitRepo, err := git.OpenRepository(ctx, dbRepo)
	if err != nil {
		return "", err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(ctx, opts.CommitID)
	if err != nil {
		return "", err
	}

	language, _ := languagestats.GetFileLanguage(ctx, gitRepo, opts.CommitID, opts.FilePath)
	blob, err := commit.GetBlobByPath(ctx, gitRepo, opts.FilePath)
	if err != nil {
		return "", err
	}

	if blob.Size(ctx) > setting.UI.MaxDisplayFileSize {
		return "", errors.New("file is too large")
	}

	dataRc, err := blob.DataAsync(ctx)
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	reader := bufio.NewReader(dataRc)
	for i := 1; i < opts.LineStart; i++ {
		if _, err = reader.ReadBytes('\n'); err != nil {
			return "", err
		}
	}

	lineNums := make([]int, 0, lineCount)
	lineCodes := make([]string, 0, lineCount)
	for i := opts.LineStart; i <= opts.LineStop; i++ {
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			break
		}

		lineNums = append(lineNums, i)
		lineCodes = append(lineCodes, line)
	}
	realLineStop := max(opts.LineStart, opts.LineStart+len(lineNums)-1)
	highlightLines := code.HighlightSearchResultCode(opts.FilePath, language, lineNums, strings.Join(lineCodes, ""))

	escapeStatus := &charset.EscapeStatus{}
	lineEscapeStatus := make([]*charset.EscapeStatus, len(highlightLines))
	for i, hl := range highlightLines {
		lineEscapeStatus[i], hl.FormattedContent = charset.EscapeControlHTML(hl.FormattedContent, webCtx.Base.Locale, charset.EscapeOptionsForView())
		escapeStatus.Combine(lineEscapeStatus[i])
	}

	return webCtx.RenderToHTML("base/markup_codepreview", map[string]any{
		"FullURL":          opts.FullURL,
		"FilePath":         opts.FilePath,
		"LineStart":        opts.LineStart,
		"LineStop":         realLineStop,
		"RepoName":         opts.RepoName,
		"RepoLink":         dbRepo.Link(),
		"CommitID":         opts.CommitID,
		"HighlightLines":   highlightLines,
		"EscapeStatus":     escapeStatus,
		"LineEscapeStatus": lineEscapeStatus,
	})
}

func getCodeSuggestionSource(ctx context.Context, anchor markup.CodeSuggestionAnchor) (markup.CodeSuggestionSource, error) {
	webCtx := gitea_context.GetWebContext(ctx)
	if webCtx == nil || webCtx.Repo == nil || !webCtx.Repo.Permission.CanRead(unit.TypeCode) {
		return markup.CodeSuggestionSource{}, util.ErrNotExist
	}
	lines, err := ReadCodeSuggestionLines(ctx, webCtx.Repo.GitRepo, anchor)
	if err != nil {
		return markup.CodeSuggestionSource{}, err
	}
	return markup.CodeSuggestionSource{
		Heading:           webCtx.Locale.TrString("repo.diff.suggestion.suggested_change"),
		OriginalLines:     lines,
		HighlightLinePair: gitdiff.HighlightDiffLinePair,
	}, nil
}

func ReadCodeSuggestionLines(ctx context.Context, repo *git.Repository, anchor markup.CodeSuggestionAnchor) ([]string, error) {
	if repo == nil || anchor.StartLine == 0 || anchor.StartLine > anchor.EndLine {
		return nil, util.ErrNotExist
	}
	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return nil, err
	}
	if !git.IsStringValidObjectID(objectFormat, anchor.CommitID) {
		return nil, util.ErrNotExist
	}
	type fileKey struct {
		repo               *git.Repository
		commitID, filePath string
	}
	lines, err := cache.GetWithContextCache(ctx, "codeSuggestionFileLines", fileKey{repo, anchor.CommitID, anchor.FilePath}, func(ctx context.Context, key fileKey) ([]string, error) {
		commit, err := key.repo.GetCommit(ctx, key.commitID)
		if err != nil {
			return nil, err
		}
		blob, err := commit.GetBlobByPath(ctx, key.repo, key.filePath)
		if err != nil {
			return nil, err
		}
		content, err := blob.GetBlobContent(ctx, setting.UI.MaxDisplayFileSize+1)
		if err != nil || content == "" || int64(len(content)) > setting.UI.MaxDisplayFileSize || strings.ContainsRune(content, 0) {
			return nil, cmp.Or[error](err, util.ErrNotExist)
		}
		return strings.Split(strings.TrimSuffix(strings.ReplaceAll(content, "\r\n", "\n"), "\n"), "\n"), nil
	})
	if err != nil || anchor.EndLine > uint64(len(lines)) {
		return nil, cmp.Or[error](err, util.ErrNotExist)
	}
	return lines[anchor.StartLine-1 : anchor.EndLine], nil
}
