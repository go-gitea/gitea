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

// See the documentation at
// https://www.elastic.co/guide/en/elasticsearch/reference/6.7/ilm-get-lifecycle.html.
type XPackIlmGetLifecycleService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	policy        []string
	timeout       string
	masterTimeout string
	flatSettings  *bool
	local         *bool
}

// NewXPackIlmGetLifecycleService creates a new XPackIlmGetLifecycleService.
func NewXPackIlmGetLifecycleService(client *Client) *XPackIlmGetLifecycleService {
	return &XPackIlmGetLifecycleService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackIlmGetLifecycleService) Pretty(pretty bool) *XPackIlmGetLifecycleService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackIlmGetLifecycleService) Human(human bool) *XPackIlmGetLifecycleService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackIlmGetLifecycleService) ErrorTrace(errorTrace bool) *XPackIlmGetLifecycleService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackIlmGetLifecycleService) FilterPath(filterPath ...string) *XPackIlmGetLifecycleService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackIlmGetLifecycleService) Header(name string, value string) *XPackIlmGetLifecycleService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackIlmGetLifecycleService) Headers(headers http.Header) *XPackIlmGetLifecycleService {
	s.headers = headers
	return s
}

// Policy is the name of the index lifecycle policy.
func (s *XPackIlmGetLifecycleService) Policy(policies ...string) *XPackIlmGetLifecycleService {
	s.policy = append(s.policy, policies...)
	return s
}

// Timeout is an explicit operation timeout.
func (s *XPackIlmGetLifecycleService) Timeout(timeout string) *XPackIlmGetLifecycleService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *XPackIlmGetLifecycleService) MasterTimeout(masterTimeout string) *XPackIlmGetLifecycleService {
	s.masterTimeout = masterTimeout
	return s
}

// FlatSettings is returns settings in flat format (default: false).
func (s *XPackIlmGetLifecycleService) FlatSettings(flatSettings bool) *XPackIlmGetLifecycleService {
	s.flatSettings = &flatSettings
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackIlmGetLifecycleService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.policy) > 0 {
		path, err = uritemplates.Expand("/_ilm/policy/{policy}", map[string]string{
			"policy": strings.Join(s.policy, ","),
		})
	} else {
		path = "/_ilm/policy"
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
	if v := s.flatSettings; v != nil {
		params.Set("flat_settings", fmt.Sprint(*v))
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if v := s.local; v != nil {
		params.Set("local", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackIlmGetLifecycleService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackIlmGetLifecycleService) Do(ctx context.Context) (map[string]*XPackIlmGetLifecycleResponse, error) {
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
	var ret map[string]*XPackIlmGetLifecycleResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackIlmGetLifecycleResponse is the response of XPackIlmGetLifecycleService.Do.
type XPackIlmGetLifecycleResponse struct {
	Version      int                    `json:"version,omitempty"`
	ModifiedDate string                 `json:"modified_date,omitempty"` // e.g. "2019-10-03T17:43:42.720Z"
	Policy       map[string]interface{} `json:"policy,omitempty"`
}
