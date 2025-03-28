// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgrep

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/setting"
)

func indexSettingToGitGrepPathspecList() (list []string) {
	for _, expr := range setting.Indexer.IncludePatterns {
		list = append(list, ":(glob)"+expr.PatternString())
	}
	for _, expr := range setting.Indexer.ExcludePatterns {
		list = append(list, ":(glob,exclude)"+expr.PatternString())
	}
	return list
}

func PerformSearch(ctx context.Context, page int, repoID int64, gitRepo *git.Repository, ref git.RefName, keyword string, searchMode indexer.SearchModeType) (searchResults []*code_indexer.Result, total int, err error) {
	grepMode := git.GrepModeWords
	if searchMode == indexer.SearchModeExact {
		grepMode = git.GrepModeExact
	} else if searchMode == indexer.SearchModeRegexp {
		grepMode = git.GrepModeRegexp
	}
	res, err := git.GrepSearch(ctx, gitRepo, keyword, git.GrepOptions{
		ContextLineNumber: 1,
		GrepMode:          grepMode,
		RefName:           ref.String(),
		PathspecList:      indexSettingToGitGrepPathspecList(),
	})
	if err != nil {
		// TODO: if no branch exists, it reports: exit status 128, fatal: this operation must be run in a work tree.
		return nil, 0, fmt.Errorf("git.GrepSearch: %w", err)
	}
	commitID, err := gitRepo.GetRefCommitID(ref.String())
	if err != nil {
		return nil, 0, fmt.Errorf("gitRepo.GetRefCommitID: %w", err)
	}

	total = len(res)
	pageStart := min((page-1)*setting.UI.RepoSearchPagingNum, len(res))
	pageEnd := min(page*setting.UI.RepoSearchPagingNum, len(res))
	res = res[pageStart:pageEnd]
	for _, r := range res {
		searchResults = append(searchResults, &code_indexer.Result{
			RepoID:   repoID,
			Filename: r.Filename,
			CommitID: commitID,
			// UpdatedUnix: not supported yet
			// Language:    not supported yet
			// Color:       not supported yet
			Lines: code_indexer.HighlightSearchResultCode(r.Filename, "", r.LineNumbers, strings.Join(r.LineCodes, "\n")),
		})
	}
	return searchResults, total, nil
}
