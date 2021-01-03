// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesSegmentsService provides low level segments information that a
// Lucene index (shard level) is built with. Allows to be used to provide
// more information on the state of a shard and an index, possibly
// optimization information, data "wasted" on deletes, and so on.
//
// Find further documentation at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-segments.html.
type IndicesSegmentsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index              []string
	allowNoIndices     *bool
	expandWildcards    string
	ignoreUnavailable  *bool
	operationThreading interface{}
	verbose            *bool
}

// NewIndicesSegmentsService creates a new IndicesSegmentsService.
func NewIndicesSegmentsService(client *Client) *IndicesSegmentsService {
	return &IndicesSegmentsService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesSegmentsService) Pretty(pretty bool) *IndicesSegmentsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesSegmentsService) Human(human bool) *IndicesSegmentsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesSegmentsService) ErrorTrace(errorTrace bool) *IndicesSegmentsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesSegmentsService) FilterPath(filterPath ...string) *IndicesSegmentsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesSegmentsService) Header(name string, value string) *IndicesSegmentsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesSegmentsService) Headers(headers http.Header) *IndicesSegmentsService {
	s.headers = headers
	return s
}

// Index is a comma-separated list of index names; use `_all` or empty string
// to perform the operation on all indices.
func (s *IndicesSegmentsService) Index(indices ...string) *IndicesSegmentsService {
	s.index = append(s.index, indices...)
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or when
// no indices have been specified).
func (s *IndicesSegmentsService) AllowNoIndices(allowNoIndices bool) *IndicesSegmentsService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to concrete indices
// that are open, closed or both.
func (s *IndicesSegmentsService) ExpandWildcards(expandWildcards string) *IndicesSegmentsService {
	s.expandWildcards = expandWildcards
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesSegmentsService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesSegmentsService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// OperationThreading is undocumented in Elasticsearch as of now.
func (s *IndicesSegmentsService) OperationThreading(operationThreading interface{}) *IndicesSegmentsService {
	s.operationThreading = operationThreading
	return s
}

// Verbose, when set to true, includes detailed memory usage by Lucene.
func (s *IndicesSegmentsService) Verbose(verbose bool) *IndicesSegmentsService {
	s.verbose = &verbose
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesSegmentsService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_segments", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_segments"
	}
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{}
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
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.operationThreading != nil {
		params.Set("operation_threading", fmt.Sprintf("%v", s.operationThreading))
	}
	if s.verbose != nil {
		params.Set("verbose", fmt.Sprintf("%v", *s.verbose))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesSegmentsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesSegmentsService) Do(ctx context.Context) (*IndicesSegmentsResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

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
	ret := new(IndicesSegmentsResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesSegmentsResponse is the response of IndicesSegmentsService.Do.
type IndicesSegmentsResponse struct {
	// Shards provides information returned from shards.
	Shards *ShardsInfo `json:"_shards"`

	// Indices provides a map into the stats of an index.
	// The key of the map is the index name.
	Indices map[string]*IndexSegments `json:"indices,omitempty"`
}

type IndexSegments struct {
	// Shards provides a map into the shard related information of an index.
	// The key of the map is the number of a specific shard.
	Shards map[string][]*IndexSegmentsShards `json:"shards,omitempty"`
}

type IndexSegmentsShards struct {
	Routing              *IndexSegmentsRouting `json:"routing,omitempty"`
	NumCommittedSegments int64                 `json:"num_committed_segments,omitempty"`
	NumSearchSegments    int64                 `json:"num_search_segments"`

	// Segments provides a map into the segment related information of a shard.
	// The key of the map is the specific lucene segment id.
	Segments map[string]*IndexSegmentsDetails `json:"segments,omitempty"`
}

type IndexSegmentsRouting struct {
	State          string `json:"state,omitempty"`
	Primary        bool   `json:"primary,omitempty"`
	Node           string `json:"node,omitempty"`
	RelocatingNode string `json:"relocating_node,omitempty"`
}

type IndexSegmentsDetails struct {
	Generation    int64                   `json:"generation,omitempty"`
	NumDocs       int64                   `json:"num_docs,omitempty"`
	DeletedDocs   int64                   `json:"deleted_docs,omitempty"`
	Size          string                  `json:"size,omitempty"`
	SizeInBytes   int64                   `json:"size_in_bytes,omitempty"`
	Memory        string                  `json:"memory,omitempty"`
	MemoryInBytes int64                   `json:"memory_in_bytes,omitempty"`
	Committed     bool                    `json:"committed,omitempty"`
	Search        bool                    `json:"search,omitempty"`
	Version       string                  `json:"version,omitempty"`
	Compound      bool                    `json:"compound,omitempty"`
	MergeId       string                  `json:"merge_id,omitempty"`
	Sort          []*IndexSegmentsSort    `json:"sort,omitempty"`
	RAMTree       []*IndexSegmentsRamTree `json:"ram_tree,omitempty"`
	Attributes    map[string]string       `json:"attributes,omitempty"`
}

type IndexSegmentsSort struct {
	Field   string      `json:"field,omitempty"`
	Mode    string      `json:"mode,omitempty"`
	Missing interface{} `json:"missing,omitempty"`
	Reverse bool        `json:"reverse,omitempty"`
}

type IndexSegmentsRamTree struct {
	Description string                  `json:"description,omitempty"`
	Size        string                  `json:"size,omitempty"`
	SizeInBytes int64                   `json:"size_in_bytes,omitempty"`
	Children    []*IndexSegmentsRamTree `json:"children,omitempty"`
}
