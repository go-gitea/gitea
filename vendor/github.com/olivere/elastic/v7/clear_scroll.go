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
)

// ClearScrollService clears one or more scroll contexts by their ids.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-scroll.html#_clear_scroll_api
// for details.
type ClearScrollService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	scrollId []string
}

// NewClearScrollService creates a new ClearScrollService.
func NewClearScrollService(client *Client) *ClearScrollService {
	return &ClearScrollService{
		client:   client,
		scrollId: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ClearScrollService) Pretty(pretty bool) *ClearScrollService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ClearScrollService) Human(human bool) *ClearScrollService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ClearScrollService) ErrorTrace(errorTrace bool) *ClearScrollService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ClearScrollService) FilterPath(filterPath ...string) *ClearScrollService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ClearScrollService) Header(name string, value string) *ClearScrollService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ClearScrollService) Headers(headers http.Header) *ClearScrollService {
	s.headers = headers
	return s
}

// ScrollId is a list of scroll IDs to clear.
// Use _all to clear all search contexts.
func (s *ClearScrollService) ScrollId(scrollIds ...string) *ClearScrollService {
	s.scrollId = append(s.scrollId, scrollIds...)
	return s
}

// buildURL builds the URL for the operation.
func (s *ClearScrollService) buildURL() (string, url.Values, error) {
	// Build URL
	path := "/_search/scroll/"

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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ClearScrollService) Validate() error {
	var invalid []string
	if len(s.scrollId) == 0 {
		invalid = append(invalid, "ScrollId")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *ClearScrollService) Do(ctx context.Context) (*ClearScrollResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	body := map[string][]string{
		"scroll_id": s.scrollId,
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "DELETE",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(ClearScrollResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ClearScrollResponse is the response of ClearScrollService.Do.
type ClearScrollResponse struct {
	Succeeded bool `json:"succeeded,omitempty"`
	NumFreed  int  `json:"num_freed,omitempty"`
}
