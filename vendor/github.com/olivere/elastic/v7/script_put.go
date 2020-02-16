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

// PutScriptService adds or updates a stored script in Elasticsearch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-scripting.html
// for details.
type PutScriptService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	context       string
	timeout       string
	masterTimeout string
	bodyJson      interface{}
	bodyString    string
}

// NewPutScriptService creates a new PutScriptService.
func NewPutScriptService(client *Client) *PutScriptService {
	return &PutScriptService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *PutScriptService) Pretty(pretty bool) *PutScriptService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *PutScriptService) Human(human bool) *PutScriptService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *PutScriptService) ErrorTrace(errorTrace bool) *PutScriptService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *PutScriptService) FilterPath(filterPath ...string) *PutScriptService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *PutScriptService) Header(name string, value string) *PutScriptService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *PutScriptService) Headers(headers http.Header) *PutScriptService {
	s.headers = headers
	return s
}

// Id is the script ID.
func (s *PutScriptService) Id(id string) *PutScriptService {
	s.id = id
	return s
}

// Context specifies the script context (optional).
func (s *PutScriptService) Context(context string) *PutScriptService {
	s.context = context
	return s
}

// Timeout is an explicit operation timeout.
func (s *PutScriptService) Timeout(timeout string) *PutScriptService {
	s.timeout = timeout
	return s
}

// MasterTimeout is the timeout for connecting to master.
func (s *PutScriptService) MasterTimeout(masterTimeout string) *PutScriptService {
	s.masterTimeout = masterTimeout
	return s
}

// BodyJson is the document as a serializable JSON interface.
func (s *PutScriptService) BodyJson(body interface{}) *PutScriptService {
	s.bodyJson = body
	return s
}

// BodyString is the document encoded as a string.
func (s *PutScriptService) BodyString(body string) *PutScriptService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *PutScriptService) buildURL() (string, string, url.Values, error) {
	var (
		err    error
		method = "PUT"
		path   string
	)

	if s.context != "" {
		path, err = uritemplates.Expand("/_scripts/{id}/{context}", map[string]string{
			"id":      s.id,
			"context": s.context,
		})
	} else {
		path, err = uritemplates.Expand("/_scripts/{id}", map[string]string{
			"id": s.id,
		})
	}
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
func (s *PutScriptService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.bodyString == "" && s.bodyJson == nil {
		invalid = append(invalid, "BodyJson")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *PutScriptService) Do(ctx context.Context) (*PutScriptResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	method, path, params, err := s.buildURL()
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
		Method:  method,
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(PutScriptResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// PutScriptResponse is the result of saving a stored script
// in Elasticsearch.
type PutScriptResponse struct {
	AcknowledgedResponse
}
