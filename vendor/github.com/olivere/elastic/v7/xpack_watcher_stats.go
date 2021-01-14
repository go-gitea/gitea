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

// XPackWatcherStatsService returns the current watcher metrics.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-stats.html.
type XPackWatcherStatsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	metric          string
	emitStacktraces *bool
}

// NewXPackWatcherStatsService creates a new XPackWatcherStatsService.
func NewXPackWatcherStatsService(client *Client) *XPackWatcherStatsService {
	return &XPackWatcherStatsService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherStatsService) Pretty(pretty bool) *XPackWatcherStatsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherStatsService) Human(human bool) *XPackWatcherStatsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherStatsService) ErrorTrace(errorTrace bool) *XPackWatcherStatsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherStatsService) FilterPath(filterPath ...string) *XPackWatcherStatsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherStatsService) Header(name string, value string) *XPackWatcherStatsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherStatsService) Headers(headers http.Header) *XPackWatcherStatsService {
	s.headers = headers
	return s
}

// Metric controls what additional stat metrics should be include in the response.
func (s *XPackWatcherStatsService) Metric(metric string) *XPackWatcherStatsService {
	s.metric = metric
	return s
}

// EmitStacktraces, if enabled, emits stack traces of currently running watches.
func (s *XPackWatcherStatsService) EmitStacktraces(emitStacktraces bool) *XPackWatcherStatsService {
	s.emitStacktraces = &emitStacktraces
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherStatsService) buildURL() (string, url.Values, error) {
	// Build URL
	path := "/_watcher/stats"

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
	if v := s.emitStacktraces; v != nil {
		params.Set("emit_stacktraces", fmt.Sprint(*v))
	}
	if s.metric != "" {
		params.Set("metric", s.metric)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackWatcherStatsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackWatcherStatsService) Do(ctx context.Context) (*XPackWatcherStatsResponse, error) {
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
	ret := new(XPackWatcherStatsResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherStatsResponse is the response of XPackWatcherStatsService.Do.
type XPackWatcherStatsResponse struct {
	Stats []XPackWatcherStats `json:"stats"`
}

// XPackWatcherStats represents the stats used in XPackWatcherStatsResponse.
type XPackWatcherStats struct {
	WatcherState        string                 `json:"watcher_state"`
	WatchCount          int                    `json:"watch_count"`
	ExecutionThreadPool map[string]interface{} `json:"execution_thread_pool"`
}
