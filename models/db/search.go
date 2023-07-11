// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

// SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchOrderByOldest                SearchOrderBy = "created_unix ASC"
	SearchOrderByNewest                SearchOrderBy = "created_unix DESC"
	SearchOrderBySize                  SearchOrderBy = "size ASC"
	SearchOrderBySizeReverse           SearchOrderBy = "size DESC"
	SearchOrderByGitSize               SearchOrderBy = "git_size ASC"
	SearchOrderByGitSizeReverse        SearchOrderBy = "git_size DESC"
	SearchOrderByLFSSize               SearchOrderBy = "lfs_size ASC"
	SearchOrderByLFSSizeReverse        SearchOrderBy = "lfs_size DESC"
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

const (
	// Which means a condition to filter the records which don't match any id.
	// It's different from zero which means the condition could be ignored.
	NoConditionID = -1
)
