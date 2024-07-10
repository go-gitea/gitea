// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	repo_model "code.gitea.io/gitea/models/repo"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	api "code.gitea.io/gitea/modules/structs"
)

func ToExploreCodeSearchResults(total int, results []*code_indexer.Result, repoMaps map[int64]*repo_model.Repository) api.ExploreCodeResult {
	out := api.ExploreCodeResult{
		Total:   total,
		Results: make([]api.ExploreCodeSearchItem, 0, len(results)),
	}
	for _, res := range results {
		if repo := repoMaps[res.RepoID]; repo != nil {
			for _, r := range res.Lines {
				out.Results = append(out.Results, api.ExploreCodeSearchItem{
					RepoName:   repo.FullName(),
					FilePath:   res.Filename,
					LineNumber: r.Num,
					LineText:   r.RawContent,
				})
			}
		}
	}
	return out
}
