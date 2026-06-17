// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"context"
	"errors"
	"html/template"
	"strings"

	"gitea.dev/models/perm/access"
	"gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git/languagestats"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/indexer/code"
	"gitea.dev/modules/markup"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	gitea_context "gitea.dev/services/context"
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

	gitRepo, err := gitrepo.OpenRepository(ctx, dbRepo)
	if err != nil {
		return "", err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(opts.CommitID)
	if err != nil {
		return "", err
	}

	language, _ := languagestats.GetFileLanguage(ctx, gitRepo, opts.CommitID, opts.FilePath)
	blob, err := commit.GetBlobByPath(opts.FilePath)
	if err != nil {
		return "", err
	}

	if blob.Size() > setting.UI.MaxDisplayFileSize {
		return "", errors.New("file is too large")
	}

	dataRc, err := blob.DataAsync()
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
		escapeStatus = escapeStatus.Or(lineEscapeStatus[i])
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
