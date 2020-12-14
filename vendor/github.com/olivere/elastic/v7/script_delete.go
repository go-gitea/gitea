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

	"github.com/olivere/elastic/v7/uritemplates"
)

// DeleteScriptService removes a stored script in Elasticsearch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-scripting.html
// for details.
type DeleteScriptService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	timeout       string
	masterTimeout string
}

// NewDeleteScriptService creates a new DeleteScriptService.
func NewDeleteScriptService(client *Client) *DeleteScriptService {
	return &DeleteScriptService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *DeleteScriptService) Pretty(pretty bool) *DeleteScriptService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *DeleteScriptService) Human(human bool) *DeleteScriptService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *DeleteScriptService) ErrorTrace(errorTrace bool) *DeleteScriptService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *DeleteScriptService) FilterPath(filterPath ...string) *DeleteScriptService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *DeleteScriptService) Header(name string, value string) *DeleteScriptService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *DeleteScriptService) Headers(headers http.Header) *DeleteScriptService {
	s.headers = headers
	return s
}

// Id is the script ID.
func (s *DeleteScriptService) Id(id string) *DeleteScriptService {
	s.id = id
	return s
}

// Timeout is an explicit operation timeout.
func (s *DeleteScriptService) Timeout(timeout string) *DeleteScriptService {
	s.timeout = timeout
	return s
}

// MasterTimeout is the timeout for connecting to master.
func (s *DeleteScriptService) MasterTimeout(masterTimeout string) *DeleteScriptService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *DeleteScriptService) buildURL() (string, string, url.Values, error) {
	var (
		err    error
		method = "DELETE"
		path   string
	)

	path, err = uritemplates.Expand("/_scripts/{id}", map[string]string{
		"id": s.id,
	})
	if err != nil {
		return "", "", url.Values{}, err
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
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timestamp", s.masterTimeout)
	}
	return method, path, params, nil
}

// Validate checks if the operation is valid.
func (s *DeleteScriptService) Validate() error {
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
func (s *DeleteScriptService) Do(ctx context.Context) (*DeleteScriptResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	method, path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  method,
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(DeleteScriptResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// DeleteScriptResponse is the result of deleting a stored script
// in Elasticsearch.
type DeleteScriptResponse struct {
	AcknowledgedResponse
}
