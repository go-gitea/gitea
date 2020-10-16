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

// CatShardsService returns the list of shards plus some additional
// information about them.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.6/cat-shards.html
// for details.
type CatShardsService struct {
	client *Client

	pretty     *bool    // pretty format the returned JSON response
	human      *bool    // return human readable values for statistics
	errorTrace *bool    // include the stack trace of returned errors
	filterPath []string // list of filters used to reduce the response

	index         []string
	bytes         string // b, k, kb, m, mb, g, gb, t, tb, p, or pb
	local         *bool
	masterTimeout string
	columns       []string
	time          string   // d, h, m, s, ms, micros, or nanos
	sort          []string // list of columns for sort order
	headers       http.Header
}

// NewCatShardsService creates a new CatShardsService.
func NewCatShardsService(client *Client) *CatShardsService {
	return &CatShardsService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *CatShardsService) Pretty(pretty bool) *CatShardsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *CatShardsService) Human(human bool) *CatShardsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *CatShardsService) ErrorTrace(errorTrace bool) *CatShardsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *CatShardsService) FilterPath(filterPath ...string) *CatShardsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *CatShardsService) Header(name string, value string) *CatShardsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *CatShardsService) Headers(headers http.Header) *CatShardsService {
	s.headers = headers
	return s
}

// Index is the name of the index to list (by default all indices are returned).
func (s *CatShardsService) Index(index ...string) *CatShardsService {
	s.index = index
	return s
}

// Bytes represents the unit in which to display byte values.
// Valid values are: "b", "k", "kb", "m", "mb", "g", "gb", "t", "tb", "p" or "pb".
func (s *CatShardsService) Bytes(bytes string) *CatShardsService {
	s.bytes = bytes
	return s
}

// Local indicates to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *CatShardsService) Local(local bool) *CatShardsService {
	s.local = &local
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *CatShardsService) MasterTimeout(masterTimeout string) *CatShardsService {
	s.masterTimeout = masterTimeout
	return s
}

// Columns to return in the response.
//
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/shards?help'
//
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatShardsService) Columns(columns ...string) *CatShardsService {
	s.columns = columns
	return s
}

// Sort is a list of fields to sort by.
func (s *CatShardsService) Sort(fields ...string) *CatShardsService {
	s.sort = fields
	return s
}

// Time specifies the way that time values are formatted with.
func (s *CatShardsService) Time(time string) *CatShardsService {
	s.time = time
	return s
}

// buildURL builds the URL for the operation.
func (s *CatShardsService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/_cat/shards/{index}", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_cat/shards"
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
	if s.time != "" {
		params.Set("time", s.time)
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
			if fullValueRaw, isAliased := catShardsResponseRowAliasesMap[column]; isAliased {
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
	if len(s.sort) > 0 {
		params.Set("s", strings.Join(s.sort, ","))
	}
	return path, params, nil
}

// Do executes the operation.
func (s *CatShardsService) Do(ctx context.Context) (CatShardsResponse, error) {
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
	var ret CatShardsResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatShardsResponse is the outcome of CatShardsService.Do.
type CatShardsResponse []CatShardsResponseRow

// CatShardsResponseRow specifies the data returned for one index
// of a CatShardsResponse. Notice that not all of these fields might
// be filled; that depends on the number of columns chose in the
// request (see CatShardsService.Columns).
type CatShardsResponseRow struct {
	Index                     string `json:"index"`        // index name
	UUID                      string `json:"uuid"`         // index uuid
	Shard                     int    `json:"shard,string"` // shard number, e.g. 1
	Prirep                    string `json:"prirep"`       // "r" for replica, "p" for primary
	State                     string `json:"state"`        // STARTED, INITIALIZING, RELOCATING, or UNASSIGNED
	Docs                      int64  `json:"docs,string"`  // number of documents, e.g. 142847
	Store                     string `json:"store"`        // size, e.g. "40mb"
	IP                        string `json:"ip"`           // IP address
	ID                        string `json:"id"`
	Node                      string `json:"node"` // Node name
	SyncID                    string `json:"sync_id"`
	UnassignedReason          string `json:"unassigned.reason"`
	UnassignedAt              string `json:"unassigned.at"`
	UnassignedFor             string `json:"unassigned.for"`
	UnassignedDetails         string `json:"unassigned.details"`
	RecoverysourceType        string `json:"recoverysource.type"`
	CompletionSize            string `json:"completion.size"`                // size of completion on primaries & replicas
	FielddataMemorySize       string `json:"fielddata.memory_size"`          // used fielddata cache on primaries & replicas
	FielddataEvictions        int    `json:"fielddata.evictions,string"`     // fielddata evictions on primaries & replicas
	QueryCacheMemorySize      string `json:"query_cache.memory_size"`        // used query cache on primaries & replicas
	QueryCacheEvictions       int    `json:"query_cache.evictions,string"`   // query cache evictions on primaries & replicas
	FlushTotal                int    `json:"flush.total,string"`             // number of flushes on primaries & replicas
	FlushTotalTime            string `json:"flush.total_time"`               // time spent in flush on primaries & replicas
	GetCurrent                int    `json:"get.current,string"`             // number of current get ops on primaries & replicas
	GetTime                   string `json:"get.time"`                       // time spent in get on primaries & replicas
	GetTotal                  int    `json:"get.total,string"`               // number of get ops on primaries & replicas
	GetExistsTime             string `json:"get.exists_time"`                // time spent in successful gets on primaries & replicas
	GetExistsTotal            int    `json:"get.exists_total,string"`        // number of successful gets on primaries & replicas
	GetMissingTime            string `json:"get.missing_time"`               // time spent in failed gets on primaries & replicas
	GetMissingTotal           int    `json:"get.missing_total,string"`       // number of failed gets on primaries & replicas
	IndexingDeleteCurrent     int    `json:"indexing.delete_current,string"` // number of current deletions on primaries & replicas
	IndexingDeleteTime        string `json:"indexing.delete_time"`           // time spent in deletions on primaries & replicas
	IndexingDeleteTotal       int    `json:"indexing.delete_total,string"`   // number of delete ops on primaries & replicas
	IndexingIndexCurrent      int    `json:"indexing.index_current,string"`  // number of current indexing on primaries & replicas
	IndexingIndexTime         string `json:"indexing.index_time"`            // time spent in indexing on primaries & replicas
	IndexingIndexTotal        int    `json:"indexing.index_total,string"`    // number of index ops on primaries & replicas
	IndexingIndexFailed       int    `json:"indexing.index_failed,string"`   // number of failed indexing ops on primaries & replicas
	MergesCurrent             int    `json:"merges.current,string"`          // number of current merges on primaries & replicas
	MergesCurrentDocs         int    `json:"merges.current_docs,string"`     // number of current merging docs on primaries & replicas
	MergesCurrentSize         string `json:"merges.current_size"`            // size of current merges on primaries & replicas
	MergesTotal               int    `json:"merges.total,string"`            // number of completed merge ops on primaries & replicas
	MergesTotalDocs           int    `json:"merges.total_docs,string"`       // docs merged on primaries & replicas
	MergesTotalSize           string `json:"merges.total_size"`              // size merged on primaries & replicas
	MergesTotalTime           string `json:"merges.total_time"`              // time spent in merges on primaries & replicas
	RefreshTotal              int    `json:"refresh.total,string"`           // total refreshes on primaries & replicas
	RefreshExternalTotal      int    `json:"refresh.external_total,string"`  // total external refreshes on primaries & replicas
	RefreshTime               string `json:"refresh.time"`                   // time spent in refreshes on primaries & replicas
	RefreshExternalTime       string `json:"refresh.external_time"`          // external time spent in refreshes on primaries & replicas
	RefreshListeners          int    `json:"refresh.listeners,string"`       // number of pending refresh listeners on primaries & replicas
	SearchFetchCurrent        int    `json:"search.fetch_current,string"`    // current fetch phase ops on primaries & replicas
	SearchFetchTime           string `json:"search.fetch_time"`              // time spent in fetch phase on primaries & replicas
	SearchFetchTotal          int    `json:"search.fetch_total,string"`      // total fetch ops on primaries & replicas
	SearchOpenContexts        int    `json:"search.open_contexts,string"`    // open search contexts on primaries & replicas
	SearchQueryCurrent        int    `json:"search.query_current,string"`    // current query phase ops on primaries & replicas
	SearchQueryTime           string `json:"search.query_time"`              // time spent in query phase on primaries & replicas, e.g. "0s"
	SearchQueryTotal          int    `json:"search.query_total,string"`      // total query phase ops on primaries & replicas
	SearchScrollCurrent       int    `json:"search.scroll_current,string"`   // open scroll contexts on primaries & replicas
	SearchScrollTime          string `json:"search.scroll_time"`             // time scroll contexts held open on primaries & replicas, e.g. "0s"
	SearchScrollTotal         int    `json:"search.scroll_total,string"`     // completed scroll contexts on primaries & replicas
	SearchThrottled           bool   `json:"search.throttled,string"`        // indicates if the index is search throttled
	SegmentsCount             int    `json:"segments.count,string"`          // number of segments on primaries & replicas
	SegmentsMemory            string `json:"segments.memory"`                // memory used by segments on primaries & replicas, e.g. "1.3kb"
	SegmentsIndexWriterMemory string `json:"segments.index_writer_memory"`   // memory used by index writer on primaries & replicas, e.g. "0b"
	SegmentsVersionMapMemory  string `json:"segments.version_map_memory"`    // memory used by version map on primaries & replicas, e.g. "0b"
	SegmentsFixedBitsetMemory string `json:"segments.fixed_bitset_memory"`   // memory used by fixed bit sets for nested object field types and type filters for types referred in _parent fields on primaries & replicas, e.g. "0b"
	SeqNoMax                  int    `json:"seq_no.max,string"`
	SeqNoLocalCheckpoint      int    `json:"seq_no.local_checkpoint,string"`
	SeqNoGlobalCheckpoint     int    `json:"seq_no.global_checkpoint,string"`
	WarmerCurrent             int    `json:"warmer.current,string"` // current warmer ops on primaries & replicas
	WarmerTotal               int    `json:"warmer.total,string"`   // total warmer ops on primaries & replicas
	WarmerTotalTime           string `json:"warmer.total_time"`     // time spent in warmers on primaries & replicas, e.g. "47s"
	PathData                  string `json:"path.data"`
	PathState                 string `json:"path.state"`
}

// catShardsResponseRowAliasesMap holds the global map for columns aliases
// the map is used by CatShardsService.buildURL.
// For backwards compatibility some fields are able to have the same aliases
// that means that one alias can be translated to different columns (from different elastic versions)
// example for understanding: rto -> RefreshTotal, RefreshExternalTotal
var catShardsResponseRowAliasesMap = map[string]string{
	"sync_id": "sync_id",
	"ur":      "unassigned.reason",
	"ua":      "unassigned.at",
	"uf":      "unassigned.for",
	"ud":      "unassigned.details",
	"rs":      "recoverysource.type",
	"cs":      "completion.size",
	"fm":      "fielddata.memory_size",
	"fe":      "fielddata.evictions",
	"qcm":     "query_cache.memory_size",
	"qce":     "query_cache.evictions",
	"ft":      "flush.total",
	"ftt":     "flush.total_time",
	"gc":      "get.current",
	"gti":     "get.time",
	"gto":     "get.total",
	"geti":    "get.exists_time",
	"geto":    "get.exists_total",
	"gmti":    "get.missing_time",
	"gmto":    "get.missing_total",
	"idc":     "indexing.delete_current",
	"idti":    "indexing.delete_time",
	"idto":    "indexing.delete_total",
	"iic":     "indexing.index_current",
	"iiti":    "indexing.index_time",
	"iito":    "indexing.index_total",
	"iif":     "indexing.index_failed",
	"mc":      "merges.current",
	"mcd":     "merges.current_docs",
	"mcs":     "merges.current_size",
	"mt":      "merges.total",
	"mtd":     "merges.total_docs",
	"mts":     "merges.total_size",
	"mtt":     "merges.total_time",
	"rto":     "refresh.total",
	"rti":     "refresh.time",
	// "rto":     "refresh.external_total",
	// "rti":     "refresh.external_time",
	"rli":  "refresh.listeners",
	"sfc":  "search.fetch_current",
	"sfti": "search.fetch_time",
	"sfto": "search.fetch_total",
	"so":   "search.open_contexts",
	"sqc":  "search.query_current",
	"sqti": "search.query_time",
	"sqto": "search.query_total",
	"scc":  "search.scroll_current",
	"scti": "search.scroll_time",
	"scto": "search.scroll_total",
	"sc":   "segments.count",
	"sm":   "segments.memory",
	"siwm": "segments.index_writer_memory",
	"svmm": "segments.version_map_memory",
	"sfbm": "segments.fixed_bitset_memory",
	"sqm":  "seq_no.max",
	"sql":  "seq_no.local_checkpoint",
	"sqg":  "seq_no.global_checkpoint",
	"wc":   "warmer.current",
	"wto":  "warmer.total",
	"wtt":  "warmer.total_time",
}
