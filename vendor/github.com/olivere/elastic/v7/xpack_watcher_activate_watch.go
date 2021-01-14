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

	"github.com/olivere/elastic/v7/uritemplates"
)

// XPackWatcherActivateWatchService enables you to activate a currently inactive watch.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-activate-watch.html.
type XPackWatcherActivateWatchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	watchId       string
	masterTimeout string
}

// NewXPackWatcherActivateWatchService creates a new XPackWatcherActivateWatchService.
func NewXPackWatcherActivateWatchService(client *Client) *XPackWatcherActivateWatchService {
	return &XPackWatcherActivateWatchService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherActivateWatchService) Pretty(pretty bool) *XPackWatcherActivateWatchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherActivateWatchService) Human(human bool) *XPackWatcherActivateWatchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherActivateWatchService) ErrorTrace(errorTrace bool) *XPackWatcherActivateWatchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherActivateWatchService) FilterPath(filterPath ...string) *XPackWatcherActivateWatchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherActivateWatchService) Header(name string, value string) *XPackWatcherActivateWatchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherActivateWatchService) Headers(headers http.Header) *XPackWatcherActivateWatchService {
	s.headers = headers
	return s
}

// WatchId is the ID of the watch to activate.
func (s *XPackWatcherActivateWatchService) WatchId(watchId string) *XPackWatcherActivateWatchService {
	s.watchId = watchId
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *XPackWatcherActivateWatchService) MasterTimeout(masterTimeout string) *XPackWatcherActivateWatchService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherActivateWatchService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_watcher/watch/{watch_id}/_activate", map[string]string{
		"watch_id": s.watchId,
	})
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
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackWatcherActivateWatchService) Validate() error {
	var invalid []string
	if s.watchId == "" {
		invalid = append(invalid, "WatchId")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackWatcherActivateWatchService) Do(ctx context.Context) (*XPackWatcherActivateWatchResponse, error) {
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
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackWatcherActivateWatchResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherActivateWatchResponse is the response of XPackWatcherActivateWatchService.Do.
type XPackWatcherActivateWatchResponse struct {
	Status *XPackWatchStatus `json:"status"`
}
