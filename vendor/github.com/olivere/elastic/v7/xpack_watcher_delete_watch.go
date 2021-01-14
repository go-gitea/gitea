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

// XPackWatcherDeleteWatchService removes a watch.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-delete-watch.html.
type XPackWatcherDeleteWatchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	masterTimeout string
}

// NewXPackWatcherDeleteWatchService creates a new XPackWatcherDeleteWatchService.
func NewXPackWatcherDeleteWatchService(client *Client) *XPackWatcherDeleteWatchService {
	return &XPackWatcherDeleteWatchService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherDeleteWatchService) Pretty(pretty bool) *XPackWatcherDeleteWatchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherDeleteWatchService) Human(human bool) *XPackWatcherDeleteWatchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherDeleteWatchService) ErrorTrace(errorTrace bool) *XPackWatcherDeleteWatchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherDeleteWatchService) FilterPath(filterPath ...string) *XPackWatcherDeleteWatchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherDeleteWatchService) Header(name string, value string) *XPackWatcherDeleteWatchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherDeleteWatchService) Headers(headers http.Header) *XPackWatcherDeleteWatchService {
	s.headers = headers
	return s
}

// Id of the watch to delete.
func (s *XPackWatcherDeleteWatchService) Id(id string) *XPackWatcherDeleteWatchService {
	s.id = id
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *XPackWatcherDeleteWatchService) MasterTimeout(masterTimeout string) *XPackWatcherDeleteWatchService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherDeleteWatchService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_watcher/watch/{id}", map[string]string{
		"id": s.id,
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
func (s *XPackWatcherDeleteWatchService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackWatcherDeleteWatchService) Do(ctx context.Context) (*XPackWatcherDeleteWatchResponse, error) {
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
		Method:  "DELETE",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackWatcherDeleteWatchResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherDeleteWatchResponse is the response of XPackWatcherDeleteWatchService.Do.
type XPackWatcherDeleteWatchResponse struct {
	Found   bool   `json:"found"`
	Id      string `json:"_id"`
	Version int    `json:"_version"`
}
