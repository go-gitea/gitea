// Copyright 2012-2018 Oliver Eilhard. All rights reserved.
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

// XPackWatcherStartService starts the watcher service if it is not already running.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-start.html.
type XPackWatcherStartService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers
}

// NewXPackWatcherStartService creates a new XPackWatcherStartService.
func NewXPackWatcherStartService(client *Client) *XPackWatcherStartService {
	return &XPackWatcherStartService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherStartService) Pretty(pretty bool) *XPackWatcherStartService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherStartService) Human(human bool) *XPackWatcherStartService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherStartService) ErrorTrace(errorTrace bool) *XPackWatcherStartService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherStartService) FilterPath(filterPath ...string) *XPackWatcherStartService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherStartService) Header(name string, value string) *XPackWatcherStartService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherStartService) Headers(headers http.Header) *XPackWatcherStartService {
	s.headers = headers
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherStartService) buildURL() (string, url.Values, error) {
	// Build URL path
	path := "/_watcher/_start"

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
func (s *XPackWatcherStartService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackWatcherStartService) Do(ctx context.Context) (*XPackWatcherStartResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackWatcherStartResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherStartResponse is the response of XPackWatcherStartService.Do.
type XPackWatcherStartResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
