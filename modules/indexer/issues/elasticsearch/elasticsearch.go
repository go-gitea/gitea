// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_elasticsearch "code.gitea.io/gitea/modules/indexer/internal/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/json"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/bulk"
	"github.com/elastic/go-elasticsearch/v8/typedapi/some"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/textquerytype"
)

const (
	issueIndexerLatestVersion = 1
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface
type Indexer struct {
	inner                    *inner_elasticsearch.Indexer
	indexer_internal.Indexer // do not composite inner_elasticsearch.Indexer directly to avoid exposing too much
}

// NewIndexer creates a new elasticsearch indexer
func NewIndexer(url, indexerName string) *Indexer {
	inner := inner_elasticsearch.NewIndexer(url, indexerName, issueIndexerLatestVersion, defaultMapping)
	indexer := &Indexer{
		inner:   inner,
		Indexer: inner,
	}
	return indexer
}

var defaultMapping = &types.TypeMapping{
	Properties: map[string]types.Property{
		"id":        types.NewIntegerNumberProperty(),
		"repo_id":   types.NewIntegerNumberProperty(),
		"is_public": types.NewBooleanProperty(),

		"title":    types.NewTextProperty(),
		"content":  types.NewTextProperty(),
		"comments": types.NewTextProperty(),

		"is_pull":              types.NewBooleanProperty(),
		"is_closed":            types.NewBooleanProperty(),
		"label_ids":            types.NewIntegerNumberProperty(),
		"no_label":             types.NewBooleanProperty(),
		"milestone_id":         types.NewIntegerNumberProperty(),
		"project_id":           types.NewIntegerNumberProperty(),
		"project_board_id":     types.NewIntegerNumberProperty(),
		"poster_id":            types.NewIntegerNumberProperty(),
		"assignee_id":          types.NewIntegerNumberProperty(),
		"mention_ids":          types.NewIntegerNumberProperty(),
		"reviewed_ids":         types.NewIntegerNumberProperty(),
		"review_requested_ids": types.NewIntegerNumberProperty(),
		"subscriber_ids":       types.NewIntegerNumberProperty(),
		"updated_unix":         types.NewIntegerNumberProperty(),
		"created_unix":         types.NewIntegerNumberProperty(),
		"deadline_unix":        types.NewIntegerNumberProperty(),
		"comment_count":        types.NewIntegerNumberProperty(),
	},
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, issues ...*internal.IndexerData) error {
	if len(issues) == 0 {
		return nil
	} else if len(issues) == 1 {
		issue := issues[0]

		raw, err := json.Marshal(issue)
		if err != nil {
			return err
		}

		_, err = b.inner.Client.Index(b.inner.VersionedIndexName()).
			Id(fmt.Sprintf("%d", issue.ID)).
			Raw(bytes.NewBuffer(raw)).
			Do(ctx)
		return err
	}

	reqs := make(bulk.Request, 0)
	for _, issue := range issues {
		reqs = append(reqs, issue)
	}

	_, err := b.inner.Client.Bulk().
		Index(b.inner.VersionedIndexName()).
		Request(&reqs).
		Do(graceful.GetManager().HammerContext())
	return err
}

// Delete deletes indexes by ids
func (b *Indexer) Delete(ctx context.Context, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}
	if len(ids) == 1 {
		_, err := b.inner.Client.Delete(
			b.inner.VersionedIndexName(),
			fmt.Sprintf("%d", ids[0]),
		).Do(ctx)
		return err
	}

	bulkIndexer, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client: &elasticsearch.Client{
			BaseClient: elasticsearch.BaseClient{
				Transport: b.inner.Client.Transport,
			},
		},
		Index: b.inner.VersionedIndexName(),
	})
	if err != nil {
		return err
	}

	for _, id := range ids {
		bulkIndexer.Add(ctx, esutil.BulkIndexerItem{
			Action:     "delete",
			Index:      b.inner.VersionedIndexName(),
			DocumentID: fmt.Sprintf("%d", id),
		})
	}

	if err := bulkIndexer.Close(context.Background()); err != nil {
		return err
	}
	return nil
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *Indexer) Search(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
	query := &types.Query{
		Bool: &types.BoolQuery{
			Must: make([]types.Query, 0),
		},
	}

	if options.Keyword != "" {
		searchType := &textquerytype.Phraseprefix
		if options.IsFuzzyKeyword {
			searchType = &textquerytype.Bestfields
		}

		query.Bool.Must = append(query.Bool.Must, types.Query{
			MultiMatch: &types.MultiMatchQuery{
				Query:  options.Keyword,
				Fields: []string{"title", "content", "comments"},
				Type:   searchType,
			},
		})
	}

	if len(options.RepoIDs) > 0 {
		q := types.Query{
			Bool: &types.BoolQuery{
				Should: make([]types.Query, 0),
			},
		}
		if options.AllPublic {
			q.Bool.Should = append(q.Bool.Should, types.Query{
				Term: map[string]types.TermQuery{
					"is_public": {Value: true},
				},
			})
		}
		query.Bool.Must = append(query.Bool.Must, q)
	}

	if options.IsPull.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"is_pull": {Value: options.IsPull.Value()},
			},
		})
	}
	if options.IsClosed.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"is_closed": {Value: options.IsClosed.Value()},
			},
		})
	}

	if options.NoLabelOnly {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"no_label": {Value: true},
			},
		})
	} else {
		if len(options.IncludedLabelIDs) > 0 {
			q := types.Query{
				Bool: &types.BoolQuery{
					Must: make([]types.Query, 0),
				},
			}
			for _, labelID := range options.IncludedLabelIDs {
				q.Bool.Must = append(q.Bool.Must, types.Query{
					Term: map[string]types.TermQuery{
						"label_ids": {Value: labelID},
					},
				})
			}
			query.Bool.Must = append(query.Bool.Must, q)
		} else if len(options.IncludedAnyLabelIDs) > 0 {
			query.Bool.Must = append(query.Bool.Must, types.Query{
				Terms: &types.TermsQuery{
					TermsQuery: map[string]types.TermsQueryField{
						"label_ids": toAnySlice(options.IncludedAnyLabelIDs),
					},
				},
			})
		}
		if len(options.ExcludedLabelIDs) > 0 {
			q := types.Query{
				Bool: &types.BoolQuery{
					MustNot: make([]types.Query, 0),
				},
			}
			for _, labelID := range options.ExcludedLabelIDs {
				q.Bool.MustNot = append(q.Bool.MustNot, types.Query{
					Term: map[string]types.TermQuery{
						"label_ids": {Value: labelID},
					},
				})
			}
			query.Bool.Must = append(query.Bool.Must, q)
		}
	}

	if len(options.MilestoneIDs) > 0 {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Terms: &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					"milestone_id": toAnySlice(options.MilestoneIDs),
				},
			},
		})
	}

	if options.ProjectID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"project_id": {Value: options.ProjectID.Value()},
			},
		})
	}
	if options.ProjectBoardID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"project_board_id": {Value: options.ProjectBoardID.Value()},
			},
		})
	}

	if options.PosterID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"poster_id": {Value: options.PosterID.Value()},
			},
		})
	}

	if options.AssigneeID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"assignee_id": {Value: options.AssigneeID.Value()},
			},
		})
	}

	if options.MentionID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"mention_ids": {Value: options.MentionID.Value()},
			},
		})
	}

	if options.ReviewedID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"reviewed_ids": {Value: options.ReviewedID.Value()},
			},
		})
	}

	if options.ReviewRequestedID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"review_requested_ids": {Value: options.ReviewRequestedID.Value()},
			},
		})
	}

	if options.SubscriberID.Has() {
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Term: map[string]types.TermQuery{
				"subscriber_ids": {Value: options.SubscriberID.Value()},
			},
		})
	}

	if options.UpdatedAfterUnix.Has() || options.UpdatedBeforeUnix.Has() {
		rangeQuery := types.NumberRangeQuery{}
		if options.UpdatedAfterUnix.Has() {
			rangeQuery.Gte = some.Float64(float64(options.UpdatedAfterUnix.Value()))
		}
		if options.UpdatedBeforeUnix.Has() {
			rangeQuery.Lte = some.Float64(float64(options.UpdatedBeforeUnix.Value()))
		}
		query.Bool.Must = append(query.Bool.Must, types.Query{
			Range: map[string]types.RangeQuery{
				"updated_unix": rangeQuery,
			},
		})
	}

	if options.SortBy == "" {
		options.SortBy = internal.SortByCreatedAsc
	}
	field, fieldSort := parseSortBy(options.SortBy)
	sort := []types.SortCombinations{
		&types.SortOptions{SortOptions: map[string]types.FieldSort{
			field: fieldSort,
			"id":  {Order: &sortorder.Desc},
		}},
	}

	// See https://stackoverflow.com/questions/35206409/elasticsearch-2-1-result-window-is-too-large-index-max-result-window/35221900
	// TODO: make it configurable since it's configurable in elasticsearch
	const maxPageSize = 10000

	skip, limit := indexer_internal.ParsePaginator(options.Paginator, maxPageSize)
	searchResult, err := b.inner.Client.Search().
		Index(b.inner.VersionedIndexName()).
		Query(query).
		Sort(sort...).
		From(skip).Size(limit).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := make([]internal.Match, 0, limit)
	for _, hit := range searchResult.Hits.Hits {
		id, _ := strconv.ParseInt(hit.Id_, 10, 64)
		hits = append(hits, internal.Match{
			ID: id,
		})
	}

	return &internal.SearchResult{
		Total: searchResult.Hits.Total.Value,
		Hits:  hits,
	}, nil
}

func toAnySlice[T any](s []T) []any {
	ret := make([]any, 0, len(s))
	for _, item := range s {
		ret = append(ret, item)
	}
	return ret
}

func parseSortBy(sortBy internal.SortBy) (string, types.FieldSort) {
	field := strings.TrimPrefix(string(sortBy), "-")
	sort := types.FieldSort{
		Order: &sortorder.Asc,
	}
	if strings.HasPrefix(string(sortBy), "-") {
		sort.Order = &sortorder.Desc
	}

	return field, sort
}
