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

// IndicesDeleteIndexTemplateService deletes index templates.
//
// Index templates have changed during in 7.8 update of Elasticsearch.
// This service implements the new version (7.8 or later). If you want
// the old version, please use the IndicesDeleteTemplateService.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/indices-delete-template.html
// for more details.
type IndicesDeleteIndexTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          string
	timeout       string
	masterTimeout string
}

// NewIndicesDeleteIndexTemplateService creates a new IndicesDeleteIndexTemplateService.
func NewIndicesDeleteIndexTemplateService(client *Client) *IndicesDeleteIndexTemplateService {
	return &IndicesDeleteIndexTemplateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesDeleteIndexTemplateService) Pretty(pretty bool) *IndicesDeleteIndexTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesDeleteIndexTemplateService) Human(human bool) *IndicesDeleteIndexTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesDeleteIndexTemplateService) ErrorTrace(errorTrace bool) *IndicesDeleteIndexTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesDeleteIndexTemplateService) FilterPath(filterPath ...string) *IndicesDeleteIndexTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesDeleteIndexTemplateService) Header(name string, value string) *IndicesDeleteIndexTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesDeleteIndexTemplateService) Headers(headers http.Header) *IndicesDeleteIndexTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the template.
func (s *IndicesDeleteIndexTemplateService) Name(name string) *IndicesDeleteIndexTemplateService {
	s.name = name
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesDeleteIndexTemplateService) Timeout(timeout string) *IndicesDeleteIndexTemplateService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesDeleteIndexTemplateService) MasterTimeout(masterTimeout string) *IndicesDeleteIndexTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesDeleteIndexTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_index_template/{name}", map[string]string{
		"name": s.name,
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
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesDeleteIndexTemplateService) Validate() error {
	var invalid []string
	if s.name == "" {
		invalid = append(invalid, "Name")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesDeleteIndexTemplateService) Do(ctx context.Context) (*IndicesDeleteIndexTemplateResponse, error) {
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
	ret := new(IndicesDeleteIndexTemplateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesDeleteIndexTemplateResponse is the response of IndicesDeleteIndexTemplateService.Do.
type IndicesDeleteIndexTemplateResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
