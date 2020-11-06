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

// XPackWatcherStopService stops the watcher service if it is running.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-stop.html.
type XPackWatcherStopService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers
}

// NewXPackWatcherStopService creates a new XPackWatcherStopService.
func NewXPackWatcherStopService(client *Client) *XPackWatcherStopService {
	return &XPackWatcherStopService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherStopService) Pretty(pretty bool) *XPackWatcherStopService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherStopService) Human(human bool) *XPackWatcherStopService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherStopService) ErrorTrace(errorTrace bool) *XPackWatcherStopService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherStopService) FilterPath(filterPath ...string) *XPackWatcherStopService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherStopService) Header(name string, value string) *XPackWatcherStopService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherStopService) Headers(headers http.Header) *XPackWatcherStopService {
	s.headers = headers
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherStopService) buildURL() (string, url.Values, error) {
	// Build URL path
	path := "/_watcher/_stop"

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
func (s *XPackWatcherStopService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackWatcherStopService) Do(ctx context.Context) (*XPackWatcherStopResponse, error) {
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
	ret := new(XPackWatcherStopResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherStopResponse is the response of XPackWatcherStopService.Do.
type XPackWatcherStopResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
