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
// https://www.elastic.co/guide/en/elasticsearch/reference/6.7/ilm-put-lifecycle.html
type XPackIlmPutLifecycleService struct {
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
	bodyJson      interface{}
	bodyString    string
}

// NewXPackIlmPutLifecycleService creates a new XPackIlmPutLifecycleService.
func NewXPackIlmPutLifecycleService(client *Client) *XPackIlmPutLifecycleService {
	return &XPackIlmPutLifecycleService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackIlmPutLifecycleService) Pretty(pretty bool) *XPackIlmPutLifecycleService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackIlmPutLifecycleService) Human(human bool) *XPackIlmPutLifecycleService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackIlmPutLifecycleService) ErrorTrace(errorTrace bool) *XPackIlmPutLifecycleService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackIlmPutLifecycleService) FilterPath(filterPath ...string) *XPackIlmPutLifecycleService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackIlmPutLifecycleService) Header(name string, value string) *XPackIlmPutLifecycleService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackIlmPutLifecycleService) Headers(headers http.Header) *XPackIlmPutLifecycleService {
	s.headers = headers
	return s
}

// Policy is the name of the index lifecycle policy.
func (s *XPackIlmPutLifecycleService) Policy(policy string) *XPackIlmPutLifecycleService {
	s.policy = policy
	return s
}

// Timeout is an explicit operation timeout.
func (s *XPackIlmPutLifecycleService) Timeout(timeout string) *XPackIlmPutLifecycleService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *XPackIlmPutLifecycleService) MasterTimeout(masterTimeout string) *XPackIlmPutLifecycleService {
	s.masterTimeout = masterTimeout
	return s
}

// FlatSettings indicates whether to return settings in flat format (default: false).
func (s *XPackIlmPutLifecycleService) FlatSettings(flatSettings bool) *XPackIlmPutLifecycleService {
	s.flatSettings = &flatSettings
	return s
}

// BodyJson is documented as: The template definition.
func (s *XPackIlmPutLifecycleService) BodyJson(body interface{}) *XPackIlmPutLifecycleService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: The template definition.
func (s *XPackIlmPutLifecycleService) BodyString(body string) *XPackIlmPutLifecycleService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackIlmPutLifecycleService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_ilm/policy/{policy}", map[string]string{
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
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if v := s.flatSettings; v != nil {
		params.Set("flat_settings", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackIlmPutLifecycleService) Validate() error {
	var invalid []string
	if s.policy == "" {
		invalid = append(invalid, "Policy")
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
func (s *XPackIlmPutLifecycleService) Do(ctx context.Context) (*XPackIlmPutLifecycleResponse, error) {
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
	ret := new(XPackIlmPutLifecycleResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackIlmPutLifecycleSResponse is the response of XPackIlmPutLifecycleService.Do.
type XPackIlmPutLifecycleResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
