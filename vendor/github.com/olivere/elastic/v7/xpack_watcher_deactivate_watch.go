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

// XPackWatcherDeactivateWatchService enables you to deactivate a currently active watch.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-deactivate-watch.html.
type XPackWatcherDeactivateWatchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	watchId       string
	masterTimeout string
}

// NewXPackWatcherDeactivateWatchService creates a new XPackWatcherDeactivateWatchService.
func NewXPackWatcherDeactivateWatchService(client *Client) *XPackWatcherDeactivateWatchService {
	return &XPackWatcherDeactivateWatchService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherDeactivateWatchService) Pretty(pretty bool) *XPackWatcherDeactivateWatchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherDeactivateWatchService) Human(human bool) *XPackWatcherDeactivateWatchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherDeactivateWatchService) ErrorTrace(errorTrace bool) *XPackWatcherDeactivateWatchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherDeactivateWatchService) FilterPath(filterPath ...string) *XPackWatcherDeactivateWatchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherDeactivateWatchService) Header(name string, value string) *XPackWatcherDeactivateWatchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherDeactivateWatchService) Headers(headers http.Header) *XPackWatcherDeactivateWatchService {
	s.headers = headers
	return s
}

// WatchId is the ID of the watch to deactivate.
func (s *XPackWatcherDeactivateWatchService) WatchId(watchId string) *XPackWatcherDeactivateWatchService {
	s.watchId = watchId
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *XPackWatcherDeactivateWatchService) MasterTimeout(masterTimeout string) *XPackWatcherDeactivateWatchService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherDeactivateWatchService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_watcher/watch/{watch_id}/_deactivate", map[string]string{
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
func (s *XPackWatcherDeactivateWatchService) Validate() error {
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
func (s *XPackWatcherDeactivateWatchService) Do(ctx context.Context) (*XPackWatcherDeactivateWatchResponse, error) {
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
	ret := new(XPackWatcherDeactivateWatchResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherDeactivateWatchResponse is the response of XPackWatcherDeactivateWatchService.Do.
type XPackWatcherDeactivateWatchResponse struct {
	Status *XPackWatchStatus `json:"status"`
}
