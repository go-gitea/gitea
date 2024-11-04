// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
)

// IndexerData data stored in the conversation indexer
type IndexerData struct {
	ID       int64 `json:"id"`
	RepoID   int64 `json:"repo_id"`
	IsPublic bool  `json:"is_public"` // If the repo is public

	// Fields used for keyword searching
	Comments []string `json:"comments"`

	// Fields used for filtering
	UpdatedUnix timeutil.TimeStamp `json:"updated_unix"`

	// Fields used for sorting
	// UpdatedUnix is both used for filtering and sorting.
	// ID is used for sorting too, to make the sorting stable.
	CreatedUnix  timeutil.TimeStamp `json:"created_unix"`
	DeadlineUnix timeutil.TimeStamp `json:"deadline_unix"`
	CommentCount int64              `json:"comment_count"`
}

// Match represents on search result
type Match struct {
	ID    int64   `json:"id"`
	Score float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int64
	Hits  []Match
}

// SearchOptions represents search options.
//
// It has a slightly different design from database query options.
// In database query options, a field is never a pointer, so it could be confusing when it's zero value:
// Do you want to find data with a field value of 0, or do you not specify the field in the options?
// To avoid this confusion, db introduced db.NoConditionID(-1).
// So zero value means the field is not specified in the search options, and db.NoConditionID means "== 0" or "id NOT IN (SELECT id FROM ...)"
// It's still not ideal, it trapped developers many times.
// And sometimes -1 could be a valid value, like conversation ID, negative numbers indicate exclusion.
// Since db.NoConditionID is for "db" (the package name is db), it makes sense not to use it in the indexer:
// Why do bleve/elasticsearch/meilisearch indexers need to know about db.NoConditionID?
// So in SearchOptions, we use pointer for fields which could be not specified,
// and always use the value to filter if it's not nil, even if it's zero or negative.
// It can handle almost all cases, if there is an exception, we can add a new field, like NoLabelOnly.
// Unfortunately, we still use db for the indexer and have to convert between db.NoConditionID and nil for legacy reasons.
type SearchOptions struct {
	Keyword string // keyword to search

	IsFuzzyKeyword bool // if false the levenshtein distance is 0

	RepoIDs   []int64 // repository IDs which the conversations belong to
	AllPublic bool    // if include all public repositories

	UpdatedAfterUnix  optional.Option[int64]
	UpdatedBeforeUnix optional.Option[int64]

	Paginator *db.ListOptions

	SortBy SortBy // sort by field
}

// Copy returns a copy of the options.
// Be careful, it's not a deep copy, so `SearchOptions.RepoIDs = {...}` is OK while `SearchOptions.RepoIDs[0] = ...` is not.
func (o *SearchOptions) Copy(edit ...func(options *SearchOptions)) *SearchOptions {
	if o == nil {
		return nil
	}
	v := *o
	for _, e := range edit {
		e(&v)
	}
	return &v
}

// used for optimized conversation index based search
func (o *SearchOptions) IsKeywordNumeric() bool {
	_, err := strconv.Atoi(o.Keyword)
	return err == nil
}

type SortBy string

const (
	SortByCreatedDesc  SortBy = "-created_unix"
	SortByUpdatedDesc  SortBy = "-updated_unix"
	SortByCommentsDesc SortBy = "-comment_count"
	SortByDeadlineDesc SortBy = "-deadline_unix"
	SortByCreatedAsc   SortBy = "created_unix"
	SortByUpdatedAsc   SortBy = "updated_unix"
	SortByCommentsAsc  SortBy = "comment_count"
	SortByDeadlineAsc  SortBy = "deadline_unix"
	// Unsupported sort types which are supported by conversations.ConversationsOptions.SortType:
	//
	//  - "priorityrepo":
	//                    It's impossible to support it in the indexer.
	//                    It is based on the specified repository in the request, so we cannot add static field to the indexer.
	//                    If we do something like that query the conversations in the specified repository first then append other conversations,
	//                    it will break the pagination.
	//
	// - "project-column-sorting":
	//                    Although it's possible to support it by adding project.ProjectConversation.Sorting to the indexer,
	//                    but what if the conversation belongs to multiple projects?
	//                    Since it's unsupported to search conversations with keyword in project page, we don't need to support it.
)
