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

// XPackSecurityGetRoleMappingService retrieves a role mapping by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/security-api-get-role-mapping.html.
type XPackSecurityGetRoleMappingService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name string
}

// NewXPackSecurityGetRoleMappingService creates a new XPackSecurityGetRoleMappingService.
func NewXPackSecurityGetRoleMappingService(client *Client) *XPackSecurityGetRoleMappingService {
	return &XPackSecurityGetRoleMappingService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityGetRoleMappingService) Pretty(pretty bool) *XPackSecurityGetRoleMappingService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityGetRoleMappingService) Human(human bool) *XPackSecurityGetRoleMappingService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityGetRoleMappingService) ErrorTrace(errorTrace bool) *XPackSecurityGetRoleMappingService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityGetRoleMappingService) FilterPath(filterPath ...string) *XPackSecurityGetRoleMappingService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityGetRoleMappingService) Header(name string, value string) *XPackSecurityGetRoleMappingService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityGetRoleMappingService) Headers(headers http.Header) *XPackSecurityGetRoleMappingService {
	s.headers = headers
	return s
}

// Name is name of the role mapping to retrieve.
func (s *XPackSecurityGetRoleMappingService) Name(name string) *XPackSecurityGetRoleMappingService {
	s.name = name
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityGetRoleMappingService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_security/role_mapping/{name}", map[string]string{
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackSecurityGetRoleMappingService) Validate() error {
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
func (s *XPackSecurityGetRoleMappingService) Do(ctx context.Context) (*XPackSecurityGetRoleMappingResponse, error) {
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
	ret := XPackSecurityGetRoleMappingResponse{}
	if err := json.Unmarshal(res.Body, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// XPackSecurityGetRoleMappingResponse is the response of XPackSecurityGetRoleMappingService.Do.
type XPackSecurityGetRoleMappingResponse map[string]XPackSecurityRoleMapping

// XPackSecurityRoleMapping is the role mapping object
type XPackSecurityRoleMapping struct {
	Enabled  bool                   `json:"enabled"`
	Roles    []string               `json:"roles"`
	Rules    map[string]interface{} `json:"rules"`
	Metadata interface{}            `json:"metadata"`
}
