// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import "strings"

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
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

// WithIDTiebreaker appends the table's id in the last term's direction so tied rows keep one order across pages
func WithIDTiebreaker[S ~string](orderBy S, table string) S {
	idColumn := "`" + table + "`.id"
	if strings.TrimSpace(string(orderBy)) == "" {
		return S(idColumn + " ASC")
	}
	terms := strings.Split(string(orderBy), ",")
	for _, term := range terms {
		column, _ := cutOrderDirection(term)
		column = strings.ToLower(strings.ReplaceAll(column, "`", ""))
		if column == "id" || column == table+".id" {
			return orderBy // MSSQL rejects a column repeated in ORDER BY
		}
	}
	_, direction := cutOrderDirection(terms[len(terms)-1])
	return orderBy + S(", "+idColumn+" "+direction)
}

func cutOrderDirection(term string) (column, direction string) {
	column = strings.TrimSpace(term)
	if idx := strings.LastIndexAny(column, " \t\n"); idx >= 0 {
		if last := strings.ToUpper(column[idx+1:]); last == "ASC" || last == "DESC" {
			return strings.TrimSpace(column[:idx]), last
		}
	}
	return column, "ASC"
}

// NoConditionID means a condition to filter the records which don't match any id.
// eg: "milestone_id=-1" means "find the items without any milestone.
const NoConditionID int64 = -1
