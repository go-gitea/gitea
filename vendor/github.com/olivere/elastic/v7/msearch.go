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
)

// MultiSearch executes one or more searches in one roundtrip.
type MultiSearchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	requests              []*SearchRequest
	indices               []string
	maxConcurrentRequests *int
	preFilterShardSize    *int
}

func NewMultiSearchService(client *Client) *MultiSearchService {
	builder := &MultiSearchService{
		client: client,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *MultiSearchService) Pretty(pretty bool) *MultiSearchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *MultiSearchService) Human(human bool) *MultiSearchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *MultiSearchService) ErrorTrace(errorTrace bool) *MultiSearchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *MultiSearchService) FilterPath(filterPath ...string) *MultiSearchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *MultiSearchService) Header(name string, value string) *MultiSearchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *MultiSearchService) Headers(headers http.Header) *MultiSearchService {
	s.headers = headers
	return s
}

func (s *MultiSearchService) Add(requests ...*SearchRequest) *MultiSearchService {
	s.requests = append(s.requests, requests...)
	return s
}

func (s *MultiSearchService) Index(indices ...string) *MultiSearchService {
	s.indices = append(s.indices, indices...)
	return s
}

func (s *MultiSearchService) MaxConcurrentSearches(max int) *MultiSearchService {
	s.maxConcurrentRequests = &max
	return s
}

func (s *MultiSearchService) PreFilterShardSize(size int) *MultiSearchService {
	s.preFilterShardSize = &size
	return s
}

func (s *MultiSearchService) Do(ctx context.Context) (*MultiSearchResult, error) {
	// Build url
	path := "/_msearch"

	// Parameters
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
	if v := s.maxConcurrentRequests; v != nil {
		params.Set("max_concurrent_searches", fmt.Sprintf("%v", *v))
	}
	if v := s.preFilterShardSize; v != nil {
		params.Set("pre_filter_shard_size", fmt.Sprintf("%v", *v))
	}

	// Set body
	var lines []string
	for _, sr := range s.requests {
		// Set default indices if not specified in the request
		if !sr.HasIndices() && len(s.indices) > 0 {
			sr = sr.Index(s.indices...)
		}

		header, err := json.Marshal(sr.header())
		if err != nil {
			return nil, err
		}
		body, err := sr.Body()
		if err != nil {
			return nil, err
		}
		lines = append(lines, string(header))
		lines = append(lines, body)
	}
	body := strings.Join(lines, "\n") + "\n" // add trailing \n

	// Get response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return result
	ret := new(MultiSearchResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// MultiSearchResult is the outcome of running a multi-search operation.
type MultiSearchResult struct {
	TookInMillis int64           `json:"took,omitempty"` // search time in milliseconds
	Responses    []*SearchResult `json:"responses,omitempty"`
}
