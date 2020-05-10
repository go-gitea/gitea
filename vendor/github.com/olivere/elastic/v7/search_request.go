// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"encoding/json"
	"strings"
)

// SearchRequest combines a search request and its
// query details (see SearchSource).
// It is used in combination with MultiSearch.
type SearchRequest struct {
	searchType                 string
	indices                    []string
	types                      []string
	routing                    *string
	preference                 *string
	requestCache               *bool
	allowPartialSearchResults  *bool
	ignoreUnavailable          *bool
	allowNoIndices             *bool
	expandWildcards            string
	scroll                     string
	source                     interface{}
	searchSource               *SearchSource
	batchedReduceSize          *int
	maxConcurrentShardRequests *int
	preFilterShardSize         *int
}

// NewSearchRequest creates a new search request.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		searchSource: NewSearchSource(),
	}
}

// SearchType must be one of "dfs_query_then_fetch", "dfs_query_and_fetch",
// "query_then_fetch", or "query_and_fetch".
func (r *SearchRequest) SearchType(searchType string) *SearchRequest {
	r.searchType = searchType
	return r
}

// SearchTypeDfsQueryThenFetch sets search type to "dfs_query_then_fetch".
func (r *SearchRequest) SearchTypeDfsQueryThenFetch() *SearchRequest {
	return r.SearchType("dfs_query_then_fetch")
}

// SearchTypeQueryThenFetch sets search type to "query_then_fetch".
func (r *SearchRequest) SearchTypeQueryThenFetch() *SearchRequest {
	return r.SearchType("query_then_fetch")
}

// Index specifies the indices to use in the request.
func (r *SearchRequest) Index(indices ...string) *SearchRequest {
	r.indices = append(r.indices, indices...)
	return r
}

// HasIndices returns true if there are indices used, false otherwise.
func (r *SearchRequest) HasIndices() bool {
	return len(r.indices) > 0
}

// Type specifies one or more types to be used.
//
// Deprecated: Types are in the process of being removed. Instead of using a type, prefer to
// filter on a field on the document.
func (r *SearchRequest) Type(types ...string) *SearchRequest {
	r.types = append(r.types, types...)
	return r
}

// Routing specifies the routing parameter. It is a comma-separated list.
func (r *SearchRequest) Routing(routing string) *SearchRequest {
	r.routing = &routing
	return r
}

// Routings to be used in the request.
func (r *SearchRequest) Routings(routings ...string) *SearchRequest {
	if routings != nil {
		routings := strings.Join(routings, ",")
		r.routing = &routings
	} else {
		r.routing = nil
	}
	return r
}

// Preference to execute the search. Defaults to randomize across shards.
// Can be set to "_local" to prefer local shards, "_primary" to execute
// only on primary shards, or a custom value, which guarantees that the
// same order will be used across different requests.
func (r *SearchRequest) Preference(preference string) *SearchRequest {
	r.preference = &preference
	return r
}

// RequestCache specifies if this request should use the request cache
// or not, assuming that it can. By default, will default to the index
// level setting if request cache is enabled or not.
func (r *SearchRequest) RequestCache(requestCache bool) *SearchRequest {
	r.requestCache = &requestCache
	return r
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *SearchRequest) IgnoreUnavailable(ignoreUnavailable bool) *SearchRequest {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all` string or when no indices have been specified).
func (s *SearchRequest) AllowNoIndices(allowNoIndices bool) *SearchRequest {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *SearchRequest) ExpandWildcards(expandWildcards string) *SearchRequest {
	s.expandWildcards = expandWildcards
	return s
}

// Scroll, if set, will enable scrolling of the search request.
// Pass a timeout value, e.g. "2m" or "30s" as a value.
func (r *SearchRequest) Scroll(scroll string) *SearchRequest {
	r.scroll = scroll
	return r
}

// SearchSource allows passing your own SearchSource, overriding
// all values set on the request (except Source).
func (r *SearchRequest) SearchSource(searchSource *SearchSource) *SearchRequest {
	if searchSource == nil {
		r.searchSource = NewSearchSource()
		return r
	}
	r.searchSource = searchSource
	return r
}

// Source allows passing your own request body. It will have preference over
// all other properties set on the request.
func (r *SearchRequest) Source(source interface{}) *SearchRequest {
	r.source = source
	return r
}

// Timeout value for the request, e.g. "30s" or "2m".
func (r *SearchRequest) Timeout(timeout string) *SearchRequest {
	r.searchSource = r.searchSource.Timeout(timeout)
	return r
}

// TerminateAfter, when set, specifies an optional document count,
// upon collecting which the search query will terminate early.
func (r *SearchRequest) TerminateAfter(docs int) *SearchRequest {
	r.searchSource = r.searchSource.TerminateAfter(docs)
	return r
}

// Query for the search.
func (r *SearchRequest) Query(query Query) *SearchRequest {
	r.searchSource = r.searchSource.Query(query)
	return r
}

// PostFilter is a filter that will be executed after the query
// has been executed and only has affect on the search hits
// (not aggregations). This filter is always executed as last
// filtering mechanism.
func (r *SearchRequest) PostFilter(filter Query) *SearchRequest {
	r.searchSource = r.searchSource.PostFilter(filter)
	return r
}

// MinScore below which documents are filtered out.
func (r *SearchRequest) MinScore(minScore float64) *SearchRequest {
	r.searchSource = r.searchSource.MinScore(minScore)
	return r
}

// From index to start search from (default is 0).
func (r *SearchRequest) From(from int) *SearchRequest {
	r.searchSource = r.searchSource.From(from)
	return r
}

// Size is the number of search hits to return (default is 10).
func (r *SearchRequest) Size(size int) *SearchRequest {
	r.searchSource = r.searchSource.Size(size)
	return r
}

// Explain indicates whether to return an explanation for each hit.
func (r *SearchRequest) Explain(explain bool) *SearchRequest {
	r.searchSource = r.searchSource.Explain(explain)
	return r
}

// Version indicates whether each hit should be returned with
// its version.
func (r *SearchRequest) Version(version bool) *SearchRequest {
	r.searchSource = r.searchSource.Version(version)
	return r
}

// IndexBoost sets a boost a specific index will receive when
// the query is executed against it.
func (r *SearchRequest) IndexBoost(index string, boost float64) *SearchRequest {
	r.searchSource = r.searchSource.IndexBoost(index, boost)
	return r
}

// Stats groups that this request will be aggregated under.
func (r *SearchRequest) Stats(statsGroup ...string) *SearchRequest {
	r.searchSource = r.searchSource.Stats(statsGroup...)
	return r
}

// FetchSource indicates whether the response should contain the stored
// _source for every hit.
func (r *SearchRequest) FetchSource(fetchSource bool) *SearchRequest {
	r.searchSource = r.searchSource.FetchSource(fetchSource)
	return r
}

// FetchSourceIncludeExclude specifies that _source should be returned
// with each hit, where "include" and "exclude" serve as a simple wildcard
// matcher that gets applied to its fields
// (e.g. include := []string{"obj1.*","obj2.*"}, exclude := []string{"description.*"}).
func (r *SearchRequest) FetchSourceIncludeExclude(include, exclude []string) *SearchRequest {
	r.searchSource = r.searchSource.FetchSourceIncludeExclude(include, exclude)
	return r
}

// FetchSourceContext indicates how the _source should be fetched.
func (r *SearchRequest) FetchSourceContext(fsc *FetchSourceContext) *SearchRequest {
	r.searchSource = r.searchSource.FetchSourceContext(fsc)
	return r
}

// DocValueField adds a docvalue based field to load and return.
// The field does not have to be stored, but it's recommended to use
// non analyzed or numeric fields.
func (r *SearchRequest) DocValueField(field string) *SearchRequest {
	r.searchSource = r.searchSource.DocvalueField(field)
	return r
}

// DocValueFieldWithFormat adds a docvalue based field to load and return.
// The field does not have to be stored, but it's recommended to use
// non analyzed or numeric fields.
func (r *SearchRequest) DocValueFieldWithFormat(field DocvalueField) *SearchRequest {
	r.searchSource = r.searchSource.DocvalueFieldWithFormat(field)
	return r
}

// DocValueFields adds one or more docvalue based field to load and return.
// The fields do not have to be stored, but it's recommended to use
// non analyzed or numeric fields.
func (r *SearchRequest) DocValueFields(fields ...string) *SearchRequest {
	r.searchSource = r.searchSource.DocvalueFields(fields...)
	return r
}

// DocValueFieldsWithFormat adds one or more docvalue based field to load and return.
// The fields do not have to be stored, but it's recommended to use
// non analyzed or numeric fields.
func (r *SearchRequest) DocValueFieldsWithFormat(fields ...DocvalueField) *SearchRequest {
	r.searchSource = r.searchSource.DocvalueFieldsWithFormat(fields...)
	return r
}

// StoredField adds a stored field to load and return
// (note, it must be stored) as part of the search request.
func (r *SearchRequest) StoredField(field string) *SearchRequest {
	r.searchSource = r.searchSource.StoredField(field)
	return r
}

// NoStoredFields indicates that no fields should be loaded,
// resulting in only id and type to be returned per field.
func (r *SearchRequest) NoStoredFields() *SearchRequest {
	r.searchSource = r.searchSource.NoStoredFields()
	return r
}

// StoredFields adds one or more stored field to load and return
// (note, they must be stored) as part of the search request.
func (r *SearchRequest) StoredFields(fields ...string) *SearchRequest {
	r.searchSource = r.searchSource.StoredFields(fields...)
	return r
}

// ScriptField adds a script based field to load and return.
// The field does not have to be stored, but it's recommended
// to use non analyzed or numeric fields.
func (r *SearchRequest) ScriptField(field *ScriptField) *SearchRequest {
	r.searchSource = r.searchSource.ScriptField(field)
	return r
}

// ScriptFields adds one or more script based field to load and return.
// The fields do not have to be stored, but it's recommended
// to use non analyzed or numeric fields.
func (r *SearchRequest) ScriptFields(fields ...*ScriptField) *SearchRequest {
	r.searchSource = r.searchSource.ScriptFields(fields...)
	return r
}

// Sort adds a sort order.
func (r *SearchRequest) Sort(field string, ascending bool) *SearchRequest {
	r.searchSource = r.searchSource.Sort(field, ascending)
	return r
}

// SortWithInfo adds a sort order.
func (r *SearchRequest) SortWithInfo(info SortInfo) *SearchRequest {
	r.searchSource = r.searchSource.SortWithInfo(info)
	return r
}

// SortBy adds a sort order.
func (r *SearchRequest) SortBy(sorter ...Sorter) *SearchRequest {
	r.searchSource = r.searchSource.SortBy(sorter...)
	return r
}

// SearchAfter sets the sort values that indicates which docs this
// request should "search after".
func (r *SearchRequest) SearchAfter(sortValues ...interface{}) *SearchRequest {
	r.searchSource = r.searchSource.SearchAfter(sortValues...)
	return r
}

// Slice allows partitioning the documents in multiple slices.
// It is e.g. used to slice a scroll operation, supported in
// Elasticsearch 5.0 or later.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-scroll.html#sliced-scroll
// for details.
func (r *SearchRequest) Slice(sliceQuery Query) *SearchRequest {
	r.searchSource = r.searchSource.Slice(sliceQuery)
	return r
}

// TrackScores is applied when sorting and controls if scores will be
// tracked as well. Defaults to false.
func (r *SearchRequest) TrackScores(trackScores bool) *SearchRequest {
	r.searchSource = r.searchSource.TrackScores(trackScores)
	return r
}

// TrackTotalHits indicates if the total hit count for the query should be tracked.
// Defaults to true.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-track-total-hits.html
// for details.
func (r *SearchRequest) TrackTotalHits(trackTotalHits interface{}) *SearchRequest {
	r.searchSource = r.searchSource.TrackTotalHits(trackTotalHits)
	return r
}

// Aggregation adds an aggreation to perform as part of the search.
func (r *SearchRequest) Aggregation(name string, aggregation Aggregation) *SearchRequest {
	r.searchSource = r.searchSource.Aggregation(name, aggregation)
	return r
}

// Highlight adds highlighting to the search.
func (r *SearchRequest) Highlight(highlight *Highlight) *SearchRequest {
	r.searchSource = r.searchSource.Highlight(highlight)
	return r
}

// Suggester adds a suggester to the search.
func (r *SearchRequest) Suggester(suggester Suggester) *SearchRequest {
	r.searchSource = r.searchSource.Suggester(suggester)
	return r
}

// Rescorer adds a rescorer to the search.
func (r *SearchRequest) Rescorer(rescore *Rescore) *SearchRequest {
	r.searchSource = r.searchSource.Rescorer(rescore)
	return r
}

// ClearRescorers removes all rescorers from the search.
func (r *SearchRequest) ClearRescorers() *SearchRequest {
	r.searchSource = r.searchSource.ClearRescorers()
	return r
}

// Profile specifies that this search source should activate the
// Profile API for queries made on it.
func (r *SearchRequest) Profile(profile bool) *SearchRequest {
	r.searchSource = r.searchSource.Profile(profile)
	return r
}

// Collapse adds field collapsing.
func (r *SearchRequest) Collapse(collapse *CollapseBuilder) *SearchRequest {
	r.searchSource = r.searchSource.Collapse(collapse)
	return r
}

// AllowPartialSearchResults indicates if this request should allow partial
// results. (If method is not called, will default to the cluster level
// setting).
func (r *SearchRequest) AllowPartialSearchResults(allow bool) *SearchRequest {
	r.allowPartialSearchResults = &allow
	return r
}

// BatchedReduceSize specifies the number of shard results that should be
// reduced at once on the coordinating node. This value should be used
// as a protection mechanism to reduce the memory overhead per search request
// if the potential number of shards in the request can be large.
func (r *SearchRequest) BatchedReduceSize(size int) *SearchRequest {
	r.batchedReduceSize = &size
	return r
}

// MaxConcurrentShardRequests sets the number of shard requests that should
// be executed concurrently. This value should be used as a protection
// mechanism to reduce the number of shard requests fired per high level
// search request. Searches that hit the entire cluster can be throttled
// with this number to reduce the cluster load. The default grows with
// the number of nodes in the cluster but is at most 256.
func (r *SearchRequest) MaxConcurrentShardRequests(size int) *SearchRequest {
	r.maxConcurrentShardRequests = &size
	return r
}

// PreFilterShardSize sets a threshold that enforces a pre-filter roundtrip
// to pre-filter search shards based on query rewriting if the number of
// shards the search request expands to exceeds the threshold.
// This filter roundtrip can limit the number of shards significantly if for
// instance a shard can not match any documents based on it's rewrite
// method ie. if date filters are mandatory to match but the shard
// bounds and the query are disjoint. The default is 128.
func (r *SearchRequest) PreFilterShardSize(size int) *SearchRequest {
	r.preFilterShardSize = &size
	return r
}

// header is used e.g. by MultiSearch to get information about the search header
// of one SearchRequest.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-multi-search.html
func (r *SearchRequest) header() interface{} {
	h := make(map[string]interface{})
	if r.searchType != "" {
		h["search_type"] = r.searchType
	}

	switch len(r.indices) {
	case 0:
	case 1:
		h["index"] = r.indices[0]
	default:
		h["indices"] = r.indices
	}

	switch len(r.types) {
	case 0:
	case 1:
		h["type"] = r.types[0]
	default:
		h["types"] = r.types
	}

	if r.routing != nil && *r.routing != "" {
		h["routing"] = *r.routing
	}
	if r.preference != nil && *r.preference != "" {
		h["preference"] = *r.preference
	}
	if r.requestCache != nil {
		h["request_cache"] = *r.requestCache
	}
	if r.ignoreUnavailable != nil {
		h["ignore_unavailable"] = *r.ignoreUnavailable
	}
	if r.allowNoIndices != nil {
		h["allow_no_indices"] = *r.allowNoIndices
	}
	if r.expandWildcards != "" {
		h["expand_wildcards"] = r.expandWildcards
	}
	if v := r.allowPartialSearchResults; v != nil {
		h["allow_partial_search_results"] = *v
	}
	if r.scroll != "" {
		h["scroll"] = r.scroll
	}

	return h
}

// Body allows to access the search body of the request, as generated by the DSL.
// Notice that Body is read-only. You must not change the request body.
//
// Body is used e.g. by MultiSearch to get information about the search body
// of one SearchRequest.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-multi-search.html
func (r *SearchRequest) Body() (string, error) {
	if r.source == nil {
		// Default: No custom source specified
		src, err := r.searchSource.Source()
		if err != nil {
			return "", err
		}
		body, err := json.Marshal(src)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
	switch t := r.source.(type) {
	default:
		body, err := json.Marshal(r.source)
		if err != nil {
			return "", err
		}
		return string(body), nil
	case *SearchSource:
		src, err := t.Source()
		if err != nil {
			return "", err
		}
		body, err := json.Marshal(src)
		if err != nil {
			return "", err
		}
		return string(body), nil
	case json.RawMessage:
		return string(t), nil
	case *json.RawMessage:
		return string(*t), nil
	case string:
		return t, nil
	case *string:
		if t != nil {
			return *t, nil
		}
		return "{}", nil
	}
}

// source returns the search source. It is used by Reindex.
func (r *SearchRequest) sourceAsMap() (interface{}, error) {
	if r.source == nil {
		// Default: No custom source specified
		return r.searchSource.Source()
	}
	switch t := r.source.(type) {
	default:
		body, err := json.Marshal(r.source)
		if err != nil {
			return "", err
		}
		return RawStringQuery(body), nil
	case *SearchSource:
		return t.Source()
	case json.RawMessage:
		return RawStringQuery(string(t)), nil
	case *json.RawMessage:
		return RawStringQuery(string(*t)), nil
	case string:
		return RawStringQuery(t), nil
	case *string:
		if t != nil {
			return RawStringQuery(*t), nil
		}
		return RawStringQuery("{}"), nil
	}
}
