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

// IndicesExistsTemplateService checks if a given template exists.
// See http://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-templates.html#indices-templates-exists
// for documentation.
type IndicesExistsTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          string
	local         *bool
	masterTimeout string
}

// NewIndicesExistsTemplateService creates a new IndicesExistsTemplateService.
func NewIndicesExistsTemplateService(client *Client) *IndicesExistsTemplateService {
	return &IndicesExistsTemplateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesExistsTemplateService) Pretty(pretty bool) *IndicesExistsTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesExistsTemplateService) Human(human bool) *IndicesExistsTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesExistsTemplateService) ErrorTrace(errorTrace bool) *IndicesExistsTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesExistsTemplateService) FilterPath(filterPath ...string) *IndicesExistsTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesExistsTemplateService) Header(name string, value string) *IndicesExistsTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesExistsTemplateService) Headers(headers http.Header) *IndicesExistsTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the template.
func (s *IndicesExistsTemplateService) Name(name string) *IndicesExistsTemplateService {
	s.name = name
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *IndicesExistsTemplateService) Local(local bool) *IndicesExistsTemplateService {
	s.local = &local
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesExistsTemplateService) MasterTimeout(masterTimeout string) *IndicesExistsTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesExistsTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_template/{name}", map[string]string{
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
	if s.local != nil {
		params.Set("local", fmt.Sprint(*s.local))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesExistsTemplateService) Validate() error {
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
func (s *IndicesExistsTemplateService) Do(ctx context.Context) (bool, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return false, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return false, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:       "HEAD",
		Path:         path,
		Params:       params,
		IgnoreErrors: []int{404},
		Headers:      s.headers,
	})
	if err != nil {
		return false, err
	}

	// Return operation response
	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("elastic: got HTTP code %d when it should have been either 200 or 404", res.StatusCode)
	}
}
