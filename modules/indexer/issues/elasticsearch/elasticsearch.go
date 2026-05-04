// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	es "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/util"
)

const issueIndexerLatestVersion = 3

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	*es.Indexer
}

func (b *Indexer) SupportedSearchModes() []indexer.SearchMode {
	// TODO: es supports fuzzy search, but our code doesn't at the moment, and actually the default fuzziness is already "AUTO"
	return indexer.SearchModesExactWords()
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	return &Indexer{Indexer: es.NewIndexer(url, indexerName, issueIndexerLatestVersion, defaultMapping)}
}

const (
	defaultMapping = `
{
	"mappings": {
		"properties": {
			"id": { "type": "integer", "index": true },
			"repo_id": { "type": "integer", "index": true },
			"is_public": { "type": "boolean", "index": true },

			"title": {  "type": "text", "index": true },
			"content": { "type": "text", "index": true },
			"comments": { "type" : "text", "index": true },

			"is_pull": { "type": "boolean", "index": true },
			"is_closed": { "type": "boolean", "index": true },
			"is_archived": { "type": "boolean", "index": true },
			"label_ids": { "type": "integer", "index": true },
			"no_label": { "type": "boolean", "index": true },
			"milestone_id": { "type": "integer", "index": true },
			"project_ids": { "type": "integer", "index": true },
			"no_project": { "type": "boolean", "index": true },
			"poster_id": { "type": "integer", "index": true },
			"assignee_id": { "type": "integer", "index": true },
			"mention_ids": { "type": "integer", "index": true },
			"reviewed_ids": { "type": "integer", "index": true },
			"review_requested_ids": { "type": "integer", "index": true },
			"subscriber_ids": { "type": "integer", "index": true },
			"updated_unix": { "type": "integer", "index": true },

			"created_unix": { "type": "integer", "index": true },
			"deadline_unix": { "type": "integer", "index": true },
			"comment_count": { "type": "integer", "index": true }
		}
	}
}
`
)

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, issues ...*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]
		return b.Indexer.Index(ctx, strconv.FormatInt(issue.ID, 10), issue)
	}

	ops := make([]es.BulkOp, 0, len(issues))
	for _, issue := range issues {
		ops = append(ops, es.IndexOp(strconv.FormatInt(issue.ID, 10), issue))
	}
	return b.Bulk(graceful.GetManager().HammerContext(), ops)
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	} else if len(ids) == 1 {
		return b.Indexer.Delete(ctx, strconv.FormatInt(ids[0], 10))
	}

	ops := make([]es.BulkOp, 0, len(ids))
	for _, id := range ids {
		ops = append(ops, es.DeleteOp(strconv.FormatInt(id, 10)))
	}
	return b.Bulk(graceful.GetManager().HammerContext(), ops)
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	query := es.NewBoolQuery()

	if options.Keyword != "" {
		searchMode := util.IfZero(options.SearchMode, b.SupportedSearchModes()[0].ModeValue)
		mm := es.NewMultiMatchQuery(options.Keyword, "title", "content", "comments")
		if searchMode == indexer.SearchModeExact {
			mm = mm.Type(es.MultiMatchTypePhrasePrefix)
		} else {
			mm = mm.Type(es.MultiMatchTypeBestFields).Operator("and")
		}
		query.Must(mm)
	}

	if len(options.RepoIDs) > 0 {
		q := es.NewBoolQuery()
		q.Should(es.TermsQuery("repo_id", es.ToAnySlice(options.RepoIDs)...))
		if options.AllPublic {
			q.Should(es.TermQuery("is_public", true))
		}
		query.Must(q)
	}

	if options.IsPull.Has() {
		query.Must(es.TermQuery("is_pull", options.IsPull.Value()))
	}
	if options.IsClosed.Has() {
		query.Must(es.TermQuery("is_closed", options.IsClosed.Value()))
	}
	if options.IsArchived.Has() {
		query.Must(es.TermQuery("is_archived", options.IsArchived.Value()))
	}

	if options.NoLabelOnly {
		query.Must(es.TermQuery("no_label", true))
	} else {
		if len(options.IncludedLabelIDs) > 0 {
			q := es.NewBoolQuery()
			for _, labelID := range options.IncludedLabelIDs {
				q.Must(es.TermQuery("label_ids", labelID))
			}
			query.Must(q)
		} else if len(options.IncludedAnyLabelIDs) > 0 {
			query.Must(es.TermsQuery("label_ids", es.ToAnySlice(options.IncludedAnyLabelIDs)...))
		}
		if len(options.ExcludedLabelIDs) > 0 {
			q := es.NewBoolQuery()
			for _, labelID := range options.ExcludedLabelIDs {
				q.MustNot(es.TermQuery("label_ids", labelID))
			}
			query.Must(q)
		}
	}

	if len(options.MilestoneIDs) > 0 {
		query.Must(es.TermsQuery("milestone_id", es.ToAnySlice(options.MilestoneIDs)...))
	}

	if options.NoProjectOnly {
		query.Must(es.TermQuery("no_project", true))
	} else if len(options.ProjectIDs) > 0 {
		// FIXME: ISSUE-MULTIPLE-PROJECTS-FILTER: this logic is not right, it should use "AND" but not "OR"
		query.Must(es.TermsQuery("project_ids", es.ToAnySlice(options.ProjectIDs)...))
	}

	if options.PosterID != "" {
		// "(none)" becomes 0, it means no poster
		posterIDInt64, _ := strconv.ParseInt(options.PosterID, 10, 64)
		query.Must(es.TermQuery("poster_id", posterIDInt64))
	}

	if options.AssigneeID != "" {
		if options.AssigneeID == "(any)" {
			query.Must(es.NewRangeQuery("assignee_id").Gte(1))
		} else {
			// "(none)" becomes 0, it means no assignee
			assigneeIDInt64, _ := strconv.ParseInt(options.AssigneeID, 10, 64)
			query.Must(es.TermQuery("assignee_id", assigneeIDInt64))
		}
	}

	if options.MentionID.Has() {
		query.Must(es.TermQuery("mention_ids", options.MentionID.Value()))
	}

	if options.ReviewedID.Has() {
		query.Must(es.TermQuery("reviewed_ids", options.ReviewedID.Value()))
	}
	if options.ReviewRequestedID.Has() {
		query.Must(es.TermQuery("review_requested_ids", options.ReviewRequestedID.Value()))
	}

	if options.SubscriberID.Has() {
		query.Must(es.TermQuery("subscriber_ids", options.SubscriberID.Value()))
	}

	if options.UpdatedAfterUnix.Has() || options.UpdatedBeforeUnix.Has() {
		q := es.NewRangeQuery("updated_unix")
		if options.UpdatedAfterUnix.Has() {
			q.Gte(options.UpdatedAfterUnix.Value())
		}
		if options.UpdatedBeforeUnix.Has() {
			q.Lte(options.UpdatedBeforeUnix.Value())
		}
		query.Must(q)
	}

	if options.SortBy == "" {
		options.SortBy = internal.SortByCreatedAsc
	}
	sortBy := []es.SortField{
		parseSortBy(options.SortBy),
		{Field: "id", Desc: true},
	}

	// See https://stackoverflow.com/questions/35206409/elasticsearch-2-1-result-window-is-too-large-index-max-result-window/35221900
	// TODO: make it configurable since it's configurable in elasticsearch
	const maxPageSize = 10000

	skip, limit := indexer_internal.ParsePaginator(options.Paginator, maxPageSize)
	resp, err := b.Indexer.Search(ctx, es.SearchRequest{
		Query:      query,
		Sort:       sortBy,
		From:       skip,
		Size:       limit,
		TrackTotal: true,
	})
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		id, _ := strconv.ParseInt(hit.ID, 10, 64)
		hits = append(hits, internal.Match{ID: id})
	}

	return &internal.SearchResult{
		Total: resp.Total,
		Hits:  hits,
	}, nil
}

func parseSortBy(sortBy internal.SortBy) es.SortField {
	field, desc := strings.CutPrefix(string(sortBy), "-")
	return es.SortField{Field: field, Desc: desc}
}
