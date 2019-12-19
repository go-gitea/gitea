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
type XPackIlmDeleteLifecycleService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	policy        string
	timeout       string
	masterTimeout string
	flatSettings  *bool
	local         *bool
}

// NewXPackIlmDeleteLifecycleService creates a new XPackIlmDeleteLifecycleService.
func NewXPackIlmDeleteLifecycleService(client *Client) *XPackIlmDeleteLifecycleService {
	return &XPackIlmDeleteLifecycleService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackIlmDeleteLifecycleService) Pretty(pretty bool) *XPackIlmDeleteLifecycleService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackIlmDeleteLifecycleService) Human(human bool) *XPackIlmDeleteLifecycleService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackIlmDeleteLifecycleService) ErrorTrace(errorTrace bool) *XPackIlmDeleteLifecycleService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackIlmDeleteLifecycleService) FilterPath(filterPath ...string) *XPackIlmDeleteLifecycleService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackIlmDeleteLifecycleService) Header(name string, value string) *XPackIlmDeleteLifecycleService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackIlmDeleteLifecycleService) Headers(headers http.Header) *XPackIlmDeleteLifecycleService {
	s.headers = headers
	return s
}

// Policy is the name of the index lifecycle policy.
func (s *XPackIlmDeleteLifecycleService) Policy(policy string) *XPackIlmDeleteLifecycleService {
	s.policy = policy
	return s
}

// Timeout is an explicit operation timeout.
func (s *XPackIlmDeleteLifecycleService) Timeout(timeout string) *XPackIlmDeleteLifecycleService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *XPackIlmDeleteLifecycleService) MasterTimeout(masterTimeout string) *XPackIlmDeleteLifecycleService {
	s.masterTimeout = masterTimeout
	return s
}

// FlatSettings is returns settings in flat format (default: false).
func (s *XPackIlmDeleteLifecycleService) FlatSettings(flatSettings bool) *XPackIlmDeleteLifecycleService {
	s.flatSettings = &flatSettings
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackIlmDeleteLifecycleService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	path, err = uritemplates.Expand("/_ilm/policy/{policy}", map[string]string{
		"policy": s.policy,
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
func (s *XPackIlmDeleteLifecycleService) Validate() error {
	var invalid []string
	if s.policy == "" {
		invalid = append(invalid, "Policy")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackIlmDeleteLifecycleService) Do(ctx context.Context) (*XPackIlmDeleteLifecycleResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Delete URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Delete HTTP response
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
	ret := new(XPackIlmDeleteLifecycleResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackIlmDeleteLifecycleResponse is the response of XPackIlmDeleteLifecycleService.Do.
type XPackIlmDeleteLifecycleResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
