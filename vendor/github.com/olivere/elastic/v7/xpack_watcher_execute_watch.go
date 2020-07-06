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

// XPackWatcherExecuteWatchService forces the execution of a stored watch.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-execute-watch.html.
type XPackWatcherExecuteWatchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id         string
	debug      *bool
	bodyJson   interface{}
	bodyString string
}

// NewXPackWatcherExecuteWatchService creates a new XPackWatcherExecuteWatchService.
func NewXPackWatcherExecuteWatchService(client *Client) *XPackWatcherExecuteWatchService {
	return &XPackWatcherExecuteWatchService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherExecuteWatchService) Pretty(pretty bool) *XPackWatcherExecuteWatchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherExecuteWatchService) Human(human bool) *XPackWatcherExecuteWatchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherExecuteWatchService) ErrorTrace(errorTrace bool) *XPackWatcherExecuteWatchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherExecuteWatchService) FilterPath(filterPath ...string) *XPackWatcherExecuteWatchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherExecuteWatchService) Header(name string, value string) *XPackWatcherExecuteWatchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherExecuteWatchService) Headers(headers http.Header) *XPackWatcherExecuteWatchService {
	s.headers = headers
	return s
}

// Id of the watch to execute on.
func (s *XPackWatcherExecuteWatchService) Id(id string) *XPackWatcherExecuteWatchService {
	s.id = id
	return s
}

// Debug indicates whether the watch should execute in debug mode.
func (s *XPackWatcherExecuteWatchService) Debug(debug bool) *XPackWatcherExecuteWatchService {
	s.debug = &debug
	return s
}

// BodyJson is documented as: Execution control.
func (s *XPackWatcherExecuteWatchService) BodyJson(body interface{}) *XPackWatcherExecuteWatchService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: Execution control.
func (s *XPackWatcherExecuteWatchService) BodyString(body string) *XPackWatcherExecuteWatchService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherExecuteWatchService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)
	if s.id != "" {
		path, err = uritemplates.Expand("/_watcher/watch/{id}/_execute", map[string]string{
			"id": s.id,
		})
	} else {
		path = "/_watcher/watch/_execute"
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
	if v := s.debug; v != nil {
		params.Set("debug", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackWatcherExecuteWatchService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackWatcherExecuteWatchService) Do(ctx context.Context) (*XPackWatcherExecuteWatchResponse, error) {
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
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else {
		body = s.bodyString
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackWatcherExecuteWatchResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherExecuteWatchResponse is the response of XPackWatcherExecuteWatchService.Do.
type XPackWatcherExecuteWatchResponse struct {
	Id          string            `json:"_id"`
	WatchRecord *XPackWatchRecord `json:"watch_record"`
}

type XPackWatchRecord struct {
	WatchId   string                            `json:"watch_id"`
	Node      string                            `json:"node"`
	Messages  []string                          `json:"messages"`
	State     string                            `json:"state"`
	Status    *XPackWatchRecordStatus           `json:"status"`
	Input     map[string]map[string]interface{} `json:"input"`
	Condition map[string]map[string]interface{} `json:"condition"`
	Result    map[string]interface{}            `json:"Result"`
}

type XPackWatchRecordStatus struct {
	Version          int                               `json:"version"`
	State            map[string]interface{}            `json:"state"`
	LastChecked      string                            `json:"last_checked"`
	LastMetCondition string                            `json:"last_met_condition"`
	Actions          map[string]map[string]interface{} `json:"actions"`
	ExecutionState   string                            `json:"execution_state"`
}
