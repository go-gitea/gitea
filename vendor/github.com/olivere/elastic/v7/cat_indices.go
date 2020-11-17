// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// CatIndicesService returns the list of indices plus some additional
// information about them.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cat-indices.html
// for details.
type CatIndicesService struct {
	client *Client

	pretty     *bool    // pretty format the returned JSON response
	human      *bool    // return human readable values for statistics
	errorTrace *bool    // include the stack trace of returned errors
	filterPath []string // list of filters used to reduce the response

	index         string
	bytes         string // b, k, m, or g
	local         *bool
	masterTimeout string
	columns       []string
	health        string   // green, yellow, or red
	primaryOnly   *bool    // true for primary shards only
	sort          []string // list of columns for sort order
	headers       http.Header
}

// NewCatIndicesService creates a new CatIndicesService.
func NewCatIndicesService(client *Client) *CatIndicesService {
	return &CatIndicesService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *CatIndicesService) Pretty(pretty bool) *CatIndicesService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *CatIndicesService) Human(human bool) *CatIndicesService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *CatIndicesService) ErrorTrace(errorTrace bool) *CatIndicesService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *CatIndicesService) FilterPath(filterPath ...string) *CatIndicesService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *CatIndicesService) Header(name string, value string) *CatIndicesService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *CatIndicesService) Headers(headers http.Header) *CatIndicesService {
	s.headers = headers
	return s
}

// Index is the name of the index to list (by default all indices are returned).
func (s *CatIndicesService) Index(index string) *CatIndicesService {
	s.index = index
	return s
}

// Bytes represents the unit in which to display byte values.
// Valid values are: "b", "k", "m", or "g".
func (s *CatIndicesService) Bytes(bytes string) *CatIndicesService {
	s.bytes = bytes
	return s
}

// Local indicates to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *CatIndicesService) Local(local bool) *CatIndicesService {
	s.local = &local
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *CatIndicesService) MasterTimeout(masterTimeout string) *CatIndicesService {
	s.masterTimeout = masterTimeout
	return s
}

// Columns to return in the response.
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/indices?help'
//
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatIndicesService) Columns(columns ...string) *CatIndicesService {
	s.columns = columns
	return s
}

// Health filters indices by their health status.
// Valid values are: "green", "yellow", or "red".
func (s *CatIndicesService) Health(healthState string) *CatIndicesService {
	s.health = healthState
	return s
}

// PrimaryOnly when set to true returns stats only for primary shards (default: false).
func (s *CatIndicesService) PrimaryOnly(primaryOnly bool) *CatIndicesService {
	s.primaryOnly = &primaryOnly
	return s
}

// Sort is a list of fields to sort by.
func (s *CatIndicesService) Sort(fields ...string) *CatIndicesService {
	s.sort = fields
	return s
}

// buildURL builds the URL for the operation.
func (s *CatIndicesService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if s.index != "" {
		path, err = uritemplates.Expand("/_cat/indices/{index}", map[string]string{
			"index": s.index,
		})
	} else {
		path = "/_cat/indices"
	}
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{
		"format": []string{"json"}, // always returns as JSON
	}
	if v := s.pretty; v != nil {
		params.Set("pretty", fmt.Sprint(*v))
	}
	if v := s.human; v != nil {
		params.Set("human", fmt.Sprint(*v))
	}
	if v := s.errorTrace; v != nil {
		params.Set("error_trace", fmt.Sprint(*v))
	}
	if len(s.filterPath) > 0 {
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.bytes != "" {
		params.Set("bytes", s.bytes)
	}
	if v := s.local; v != nil {
		params.Set("local", fmt.Sprint(*v))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if len(s.columns) > 0 {
		// loop through all columns and apply alias if needed
		for i, column := range s.columns {
			if fullValueRaw, isAliased := catIndicesResponseRowAliasesMap[column]; isAliased {
				// alias can be translated to multiple fields,
				// so if translated value contains a comma, than replace the first value
				// and append the others
				if strings.Contains(fullValueRaw, ",") {
					fullValues := strings.Split(fullValueRaw, ",")
					s.columns[i] = fullValues[0]
					s.columns = append(s.columns, fullValues[1:]...)
				} else {
					s.columns[i] = fullValueRaw
				}
			}
		}

		params.Set("h", strings.Join(s.columns, ","))
	}
	if s.health != "" {
		params.Set("health", s.health)
	}
	if v := s.primaryOnly; v != nil {
		params.Set("pri", fmt.Sprint(*v))
	}
	if len(s.sort) > 0 {
		params.Set("s", strings.Join(s.sort, ","))
	}
	return path, params, nil
}

// Do executes the operation.
func (s *CatIndicesService) Do(ctx context.Context) (CatIndicesResponse, error) {
	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	var ret CatIndicesResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatIndicesResponse is the outcome of CatIndicesService.Do.
type CatIndicesResponse []CatIndicesResponseRow

// CatIndicesResponseRow specifies the data returned for one index
// of a CatIndicesResponse. Notice that not all of these fields might
// be filled; that depends on the number of columns chose in the
// request (see CatIndicesService.Columns).
type CatIndicesResponseRow struct {
	Health                       string `json:"health"`                              // "green", "yellow", or "red"
	Status                       string `json:"status"`                              // "open" or "closed"
	Index                        string `json:"index"`                               // index name
	UUID                         string `json:"uuid"`                                // index uuid
	Pri                          int    `json:"pri,string"`                          // number of primary shards
	Rep                          int    `json:"rep,string"`                          // number of replica shards
	DocsCount                    int    `json:"docs.count,string"`                   // number of available documents
	DocsDeleted                  int    `json:"docs.deleted,string"`                 // number of deleted documents
	CreationDate                 int64  `json:"creation.date,string"`                // index creation date (millisecond value), e.g. 1527077221644
	CreationDateString           string `json:"creation.date.string"`                // index creation date (as string), e.g. "2018-05-23T12:07:01.644Z"
	StoreSize                    string `json:"store.size"`                          // store size of primaries & replicas, e.g. "4.6kb"
	PriStoreSize                 string `json:"pri.store.size"`                      // store size of primaries, e.g. "230b"
	CompletionSize               string `json:"completion.size"`                     // size of completion on primaries & replicas
	PriCompletionSize            string `json:"pri.completion.size"`                 // size of completion on primaries
	FielddataMemorySize          string `json:"fielddata.memory_size"`               // used fielddata cache on primaries & replicas
	PriFielddataMemorySize       string `json:"pri.fielddata.memory_size"`           // used fielddata cache on primaries
	FielddataEvictions           int    `json:"fielddata.evictions,string"`          // fielddata evictions on primaries & replicas
	PriFielddataEvictions        int    `json:"pri.fielddata.evictions,string"`      // fielddata evictions on primaries
	QueryCacheMemorySize         string `json:"query_cache.memory_size"`             // used query cache on primaries & replicas
	PriQueryCacheMemorySize      string `json:"pri.query_cache.memory_size"`         // used query cache on primaries
	QueryCacheEvictions          int    `json:"query_cache.evictions,string"`        // query cache evictions on primaries & replicas
	PriQueryCacheEvictions       int    `json:"pri.query_cache.evictions,string"`    // query cache evictions on primaries
	RequestCacheMemorySize       string `json:"request_cache.memory_size"`           // used request cache on primaries & replicas
	PriRequestCacheMemorySize    string `json:"pri.request_cache.memory_size"`       // used request cache on primaries
	RequestCacheEvictions        int    `json:"request_cache.evictions,string"`      // request cache evictions on primaries & replicas
	PriRequestCacheEvictions     int    `json:"pri.request_cache.evictions,string"`  // request cache evictions on primaries
	RequestCacheHitCount         int    `json:"request_cache.hit_count,string"`      // request cache hit count on primaries & replicas
	PriRequestCacheHitCount      int    `json:"pri.request_cache.hit_count,string"`  // request cache hit count on primaries
	RequestCacheMissCount        int    `json:"request_cache.miss_count,string"`     // request cache miss count on primaries & replicas
	PriRequestCacheMissCount     int    `json:"pri.request_cache.miss_count,string"` // request cache miss count on primaries
	FlushTotal                   int    `json:"flush.total,string"`                  // number of flushes on primaries & replicas
	PriFlushTotal                int    `json:"pri.flush.total,string"`              // number of flushes on primaries
	FlushTotalTime               string `json:"flush.total_time"`                    // time spent in flush on primaries & replicas
	PriFlushTotalTime            string `json:"pri.flush.total_time"`                // time spent in flush on primaries
	GetCurrent                   int    `json:"get.current,string"`                  // number of current get ops on primaries & replicas
	PriGetCurrent                int    `json:"pri.get.current,string"`              // number of current get ops on primaries
	GetTime                      string `json:"get.time"`                            // time spent in get on primaries & replicas
	PriGetTime                   string `json:"pri.get.time"`                        // time spent in get on primaries
	GetTotal                     int    `json:"get.total,string"`                    // number of get ops on primaries & replicas
	PriGetTotal                  int    `json:"pri.get.total,string"`                // number of get ops on primaries
	GetExistsTime                string `json:"get.exists_time"`                     // time spent in successful gets on primaries & replicas
	PriGetExistsTime             string `json:"pri.get.exists_time"`                 // time spent in successful gets on primaries
	GetExistsTotal               int    `json:"get.exists_total,string"`             // number of successful gets on primaries & replicas
	PriGetExistsTotal            int    `json:"pri.get.exists_total,string"`         // number of successful gets on primaries
	GetMissingTime               string `json:"get.missing_time"`                    // time spent in failed gets on primaries & replicas
	PriGetMissingTime            string `json:"pri.get.missing_time"`                // time spent in failed gets on primaries
	GetMissingTotal              int    `json:"get.missing_total,string"`            // number of failed gets on primaries & replicas
	PriGetMissingTotal           int    `json:"pri.get.missing_total,string"`        // number of failed gets on primaries
	IndexingDeleteCurrent        int    `json:"indexing.delete_current,string"`      // number of current deletions on primaries & replicas
	PriIndexingDeleteCurrent     int    `json:"pri.indexing.delete_current,string"`  // number of current deletions on primaries
	IndexingDeleteTime           string `json:"indexing.delete_time"`                // time spent in deletions on primaries & replicas
	PriIndexingDeleteTime        string `json:"pri.indexing.delete_time"`            // time spent in deletions on primaries
	IndexingDeleteTotal          int    `json:"indexing.delete_total,string"`        // number of delete ops on primaries & replicas
	PriIndexingDeleteTotal       int    `json:"pri.indexing.delete_total,string"`    // number of delete ops on primaries
	IndexingIndexCurrent         int    `json:"indexing.index_current,string"`       // number of current indexing on primaries & replicas
	PriIndexingIndexCurrent      int    `json:"pri.indexing.index_current,string"`   // number of current indexing on primaries
	IndexingIndexTime            string `json:"indexing.index_time"`                 // time spent in indexing on primaries & replicas
	PriIndexingIndexTime         string `json:"pri.indexing.index_time"`             // time spent in indexing on primaries
	IndexingIndexTotal           int    `json:"indexing.index_total,string"`         // number of index ops on primaries & replicas
	PriIndexingIndexTotal        int    `json:"pri.indexing.index_total,string"`     // number of index ops on primaries
	IndexingIndexFailed          int    `json:"indexing.index_failed,string"`        // number of failed indexing ops on primaries & replicas
	PriIndexingIndexFailed       int    `json:"pri.indexing.index_failed,string"`    // number of failed indexing ops on primaries
	MergesCurrent                int    `json:"merges.current,string"`               // number of current merges on primaries & replicas
	PriMergesCurrent             int    `json:"pri.merges.current,string"`           // number of current merges on primaries
	MergesCurrentDocs            int    `json:"merges.current_docs,string"`          // number of current merging docs on primaries & replicas
	PriMergesCurrentDocs         int    `json:"pri.merges.current_docs,string"`      // number of current merging docs on primaries
	MergesCurrentSize            string `json:"merges.current_size"`                 // size of current merges on primaries & replicas
	PriMergesCurrentSize         string `json:"pri.merges.current_size"`             // size of current merges on primaries
	MergesTotal                  int    `json:"merges.total,string"`                 // number of completed merge ops on primaries & replicas
	PriMergesTotal               int    `json:"pri.merges.total,string"`             // number of completed merge ops on primaries
	MergesTotalDocs              int    `json:"merges.total_docs,string"`            // docs merged on primaries & replicas
	PriMergesTotalDocs           int    `json:"pri.merges.total_docs,string"`        // docs merged on primaries
	MergesTotalSize              string `json:"merges.total_size"`                   // size merged on primaries & replicas
	PriMergesTotalSize           string `json:"pri.merges.total_size"`               // size merged on primaries
	MergesTotalTime              string `json:"merges.total_time"`                   // time spent in merges on primaries & replicas
	PriMergesTotalTime           string `json:"pri.merges.total_time"`               // time spent in merges on primaries
	RefreshTotal                 int    `json:"refresh.total,string"`                // total refreshes on primaries & replicas
	PriRefreshTotal              int    `json:"pri.refresh.total,string"`            // total refreshes on primaries
	RefreshExternalTotal         int    `json:"refresh.external_total,string"`       // total external refreshes on primaries & replicas
	PriRefreshExternalTotal      int    `json:"pri.refresh.external_total,string"`   // total external refreshes on primaries
	RefreshTime                  string `json:"refresh.time"`                        // time spent in refreshes on primaries & replicas
	PriRefreshTime               string `json:"pri.refresh.time"`                    // time spent in refreshes on primaries
	RefreshExternalTime          string `json:"refresh.external_time"`               // external time spent in refreshes on primaries & replicas
	PriRefreshExternalTime       string `json:"pri.refresh.external_time"`           // external time spent in refreshes on primaries
	RefreshListeners             int    `json:"refresh.listeners,string"`            // number of pending refresh listeners on primaries & replicas
	PriRefreshListeners          int    `json:"pri.refresh.listeners,string"`        // number of pending refresh listeners on primaries
	SearchFetchCurrent           int    `json:"search.fetch_current,string"`         // current fetch phase ops on primaries & replicas
	PriSearchFetchCurrent        int    `json:"pri.search.fetch_current,string"`     // current fetch phase ops on primaries
	SearchFetchTime              string `json:"search.fetch_time"`                   // time spent in fetch phase on primaries & replicas
	PriSearchFetchTime           string `json:"pri.search.fetch_time"`               // time spent in fetch phase on primaries
	SearchFetchTotal             int    `json:"search.fetch_total,string"`           // total fetch ops on primaries & replicas
	PriSearchFetchTotal          int    `json:"pri.search.fetch_total,string"`       // total fetch ops on primaries
	SearchOpenContexts           int    `json:"search.open_contexts,string"`         // open search contexts on primaries & replicas
	PriSearchOpenContexts        int    `json:"pri.search.open_contexts,string"`     // open search contexts on primaries
	SearchQueryCurrent           int    `json:"search.query_current,string"`         // current query phase ops on primaries & replicas
	PriSearchQueryCurrent        int    `json:"pri.search.query_current,string"`     // current query phase ops on primaries
	SearchQueryTime              string `json:"search.query_time"`                   // time spent in query phase on primaries & replicas, e.g. "0s"
	PriSearchQueryTime           string `json:"pri.search.query_time"`               // time spent in query phase on primaries, e.g. "0s"
	SearchQueryTotal             int    `json:"search.query_total,string"`           // total query phase ops on primaries & replicas
	PriSearchQueryTotal          int    `json:"pri.search.query_total,string"`       // total query phase ops on primaries
	SearchScrollCurrent          int    `json:"search.scroll_current,string"`        // open scroll contexts on primaries & replicas
	PriSearchScrollCurrent       int    `json:"pri.search.scroll_current,string"`    // open scroll contexts on primaries
	SearchScrollTime             string `json:"search.scroll_time"`                  // time scroll contexts held open on primaries & replicas, e.g. "0s"
	PriSearchScrollTime          string `json:"pri.search.scroll_time"`              // time scroll contexts held open on primaries, e.g. "0s"
	SearchScrollTotal            int    `json:"search.scroll_total,string"`          // completed scroll contexts on primaries & replicas
	PriSearchScrollTotal         int    `json:"pri.search.scroll_total,string"`      // completed scroll contexts on primaries
	SearchThrottled              bool   `json:"search.throttled,string"`             // indicates if the index is search throttled
	SegmentsCount                int    `json:"segments.count,string"`               // number of segments on primaries & replicas
	PriSegmentsCount             int    `json:"pri.segments.count,string"`           // number of segments on primaries
	SegmentsMemory               string `json:"segments.memory"`                     // memory used by segments on primaries & replicas, e.g. "1.3kb"
	PriSegmentsMemory            string `json:"pri.segments.memory"`                 // memory used by segments on primaries, e.g. "1.3kb"
	SegmentsIndexWriterMemory    string `json:"segments.index_writer_memory"`        // memory used by index writer on primaries & replicas, e.g. "0b"
	PriSegmentsIndexWriterMemory string `json:"pri.segments.index_writer_memory"`    // memory used by index writer on primaries, e.g. "0b"
	SegmentsVersionMapMemory     string `json:"segments.version_map_memory"`         // memory used by version map on primaries & replicas, e.g. "0b"
	PriSegmentsVersionMapMemory  string `json:"pri.segments.version_map_memory"`     // memory used by version map on primaries, e.g. "0b"
	SegmentsFixedBitsetMemory    string `json:"segments.fixed_bitset_memory"`        // memory used by fixed bit sets for nested object field types and type filters for types referred in _parent fields on primaries & replicas, e.g. "0b"
	PriSegmentsFixedBitsetMemory string `json:"pri.segments.fixed_bitset_memory"`    // memory used by fixed bit sets for nested object field types and type filters for types referred in _parent fields on primaries, e.g. "0b"
	WarmerCurrent                int    `json:"warmer.current,string"`               // current warmer ops on primaries & replicas
	PriWarmerCurrent             int    `json:"pri.warmer.current,string"`           // current warmer ops on primaries
	WarmerTotal                  int    `json:"warmer.total,string"`                 // total warmer ops on primaries & replicas
	PriWarmerTotal               int    `json:"pri.warmer.total,string"`             // total warmer ops on primaries
	WarmerTotalTime              string `json:"warmer.total_time"`                   // time spent in warmers on primaries & replicas, e.g. "47s"
	PriWarmerTotalTime           string `json:"pri.warmer.total_time"`               // time spent in warmers on primaries, e.g. "47s"
	SuggestCurrent               int    `json:"suggest.current,string"`              // number of current suggest ops on primaries & replicas
	PriSuggestCurrent            int    `json:"pri.suggest.current,string"`          // number of current suggest ops on primaries
	SuggestTime                  string `json:"suggest.time"`                        // time spend in suggest on primaries & replicas, "31s"
	PriSuggestTime               string `json:"pri.suggest.time"`                    // time spend in suggest on primaries, e.g. "31s"
	SuggestTotal                 int    `json:"suggest.total,string"`                // number of suggest ops on primaries & replicas
	PriSuggestTotal              int    `json:"pri.suggest.total,string"`            // number of suggest ops on primaries
	MemoryTotal                  string `json:"memory.total"`                        // total user memory on primaries & replicas, e.g. "1.5kb"
	PriMemoryTotal               string `json:"pri.memory.total"`                    // total user memory on primaries, e.g. "1.5kb"
}

// catIndicesResponseRowAliasesMap holds the global map for columns aliases
// the map is used by CatIndicesService.buildURL
// for backwards compatibility some fields are able to have the same aliases
// that means that one alias can be translated to different columns (from different elastic versions)
// example for understanding: rto -> RefreshTotal, RefreshExternalTotal
var catIndicesResponseRowAliasesMap = map[string]string{
	"qce":                       "query_cache.evictions",
	"searchFetchTime":           "search.fetch_time",
	"memoryTotal":               "memory.total",
	"requestCacheEvictions":     "request_cache.evictions",
	"ftt":                       "flush.total_time",
	"iic":                       "indexing.index_current",
	"mtt":                       "merges.total_time",
	"scti":                      "search.scroll_time",
	"searchScrollTime":          "search.scroll_time",
	"segmentsCount":             "segments.count",
	"getTotal":                  "get.total",
	"sfti":                      "search.fetch_time",
	"searchScrollCurrent":       "search.scroll_current",
	"svmm":                      "segments.version_map_memory",
	"warmerTotalTime":           "warmer.total_time",
	"r":                         "rep",
	"indexingIndexTime":         "indexing.index_time",
	"refreshTotal":              "refresh.total,refresh.external_total",
	"scc":                       "search.scroll_current",
	"suggestTime":               "suggest.time",
	"idc":                       "indexing.delete_current",
	"rti":                       "refresh.time,refresh.external_time",
	"sfto":                      "search.fetch_total",
	"completionSize":            "completion.size",
	"mt":                        "merges.total",
	"segmentsVersionMapMemory":  "segments.version_map_memory",
	"rto":                       "refresh.total,refresh.external_total",
	"id":                        "uuid",
	"dd":                        "docs.deleted",
	"docsDeleted":               "docs.deleted",
	"fielddataMemory":           "fielddata.memory_size",
	"getTime":                   "get.time",
	"getExistsTime":             "get.exists_time",
	"mtd":                       "merges.total_docs",
	"rli":                       "refresh.listeners",
	"h":                         "health",
	"cds":                       "creation.date.string",
	"rcmc":                      "request_cache.miss_count",
	"iif":                       "indexing.index_failed",
	"warmerCurrent":             "warmer.current",
	"gti":                       "get.time",
	"indexingIndexFailed":       "indexing.index_failed",
	"mts":                       "merges.total_size",
	"sqti":                      "search.query_time",
	"segmentsIndexWriterMemory": "segments.index_writer_memory",
	"iiti":                      "indexing.index_time",
	"iito":                      "indexing.index_total",
	"cd":                        "creation.date",
	"gc":                        "get.current",
	"searchFetchTotal":          "search.fetch_total",
	"sqc":                       "search.query_current",
	"segmentsMemory":            "segments.memory",
	"dc":                        "docs.count",
	"qcm":                       "query_cache.memory_size",
	"queryCacheMemory":          "query_cache.memory_size",
	"mergesTotalDocs":           "merges.total_docs",
	"searchOpenContexts":        "search.open_contexts",
	"shards.primary":            "pri",
	"cs":                        "completion.size",
	"mergesTotalTIme":           "merges.total_time",
	"wtt":                       "warmer.total_time",
	"mergesCurrentSize":         "merges.current_size",
	"mergesTotal":               "merges.total",
	"refreshTime":               "refresh.time,refresh.external_time",
	"wc":                        "warmer.current",
	"p":                         "pri",
	"idti":                      "indexing.delete_time",
	"searchQueryCurrent":        "search.query_current",
	"warmerTotal":               "warmer.total",
	"suggestTotal":              "suggest.total",
	"tm":                        "memory.total",
	"ss":                        "store.size",
	"ft":                        "flush.total",
	"getExistsTotal":            "get.exists_total",
	"scto":                      "search.scroll_total",
	"s":                         "status",
	"queryCacheEvictions":       "query_cache.evictions",
	"rce":                       "request_cache.evictions",
	"geto":                      "get.exists_total",
	"refreshListeners":          "refresh.listeners",
	"suto":                      "suggest.total",
	"storeSize":                 "store.size",
	"gmti":                      "get.missing_time",
	"indexingIdexCurrent":       "indexing.index_current",
	"searchFetchCurrent":        "search.fetch_current",
	"idx":                       "index",
	"fm":                        "fielddata.memory_size",
	"geti":                      "get.exists_time",
	"indexingDeleteCurrent":     "indexing.delete_current",
	"mergesCurrentDocs":         "merges.current_docs",
	"sth":                       "search.throttled",
	"flushTotal":                "flush.total",
	"sfc":                       "search.fetch_current",
	"wto":                       "warmer.total",
	"suti":                      "suggest.time",
	"shardsReplica":             "rep",
	"mergesCurrent":             "merges.current",
	"mcs":                       "merges.current_size",
	"so":                        "search.open_contexts",
	"i":                         "index",
	"siwm":                      "segments.index_writer_memory",
	"sfbm":                      "segments.fixed_bitset_memory",
	"fe":                        "fielddata.evictions",
	"requestCacheMissCount":     "request_cache.miss_count",
	"idto":                      "indexing.delete_total",
	"mergesTotalSize":           "merges.total_size",
	"suc":                       "suggest.current",
	"suggestCurrent":            "suggest.current",
	"flushTotalTime":            "flush.total_time",
	"getMissingTotal":           "get.missing_total",
	"sqto":                      "search.query_total",
	"searchScrollTotal":         "search.scroll_total",
	"fixedBitsetMemory":         "segments.fixed_bitset_memory",
	"getMissingTime":            "get.missing_time",
	"indexingDeleteTotal":       "indexing.delete_total",
	"mcd":                       "merges.current_docs",
	"docsCount":                 "docs.count",
	"gto":                       "get.total",
	"mc":                        "merges.current",
	"fielddataEvictions":        "fielddata.evictions",
	"rcm":                       "request_cache.memory_size",
	"requestCacheHitCount":      "request_cache.hit_count",
	"gmto":                      "get.missing_total",
	"searchQueryTime":           "search.query_time",
	"shards.replica":            "rep",
	"requestCacheMemory":        "request_cache.memory_size",
	"rchc":                      "request_cache.hit_count",
	"getCurrent":                "get.current",
	"indexingIndexTotal":        "indexing.index_total",
	"sc":                        "segments.count,segments.memory",
	"shardsPrimary":             "pri",
	"indexingDeleteTime":        "indexing.delete_time",
	"searchQueryTotal":          "search.query_total",
}
