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
	SearchOrderBySubjectAlphabetically SearchOrderBy = "CASE WHEN subject != '' THEN subject ELSE name END ASC"
	SearchOrderBySubjectReverse        SearchOrderBy = "CASE WHEN subject != '' THEN subject ELSE name END DESC"
	SearchOrderByScore                 SearchOrderBy = "relevance_score ASC, CASE WHEN subject != '' THEN subject ELSE name END ASC"
	SearchOrderByScoreReverse          SearchOrderBy = "relevance_score DESC, CASE WHEN subject != '' THEN subject ELSE name END ASC"
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchOrderByOldest                SearchOrderBy = "created_unix ASC"
	SearchOrderByNewest                SearchOrderBy = "created_unix DESC"
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

// NoConditionID means a condition to filter the records which don't match any id.
// eg: "milestone_id=-1" means "find the items without any milestone.
const NoConditionID int64 = -1
