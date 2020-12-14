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

// XPackSecurityDeleteRoleService delete a role by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/security-api-delete-role.html.
type XPackSecurityDeleteRoleService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name string
}

// NewXPackSecurityDeleteRoleService creates a new XPackSecurityDeleteRoleService.
func NewXPackSecurityDeleteRoleService(client *Client) *XPackSecurityDeleteRoleService {
	return &XPackSecurityDeleteRoleService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityDeleteRoleService) Pretty(pretty bool) *XPackSecurityDeleteRoleService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityDeleteRoleService) Human(human bool) *XPackSecurityDeleteRoleService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityDeleteRoleService) ErrorTrace(errorTrace bool) *XPackSecurityDeleteRoleService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityDeleteRoleService) FilterPath(filterPath ...string) *XPackSecurityDeleteRoleService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityDeleteRoleService) Header(name string, value string) *XPackSecurityDeleteRoleService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityDeleteRoleService) Headers(headers http.Header) *XPackSecurityDeleteRoleService {
	s.headers = headers
	return s
}

// Name is name of the role to delete.
func (s *XPackSecurityDeleteRoleService) Name(name string) *XPackSecurityDeleteRoleService {
	s.name = name
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityDeleteRoleService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_security/role/{name}", map[string]string{
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
func (s *XPackSecurityDeleteRoleService) Validate() error {
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
func (s *XPackSecurityDeleteRoleService) Do(ctx context.Context) (*XPackSecurityDeleteRoleResponse, error) {
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
	ret := new(XPackSecurityDeleteRoleResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackSecurityDeleteRoleResponse is the response of XPackSecurityDeleteRoleService.Do.
type XPackSecurityDeleteRoleResponse struct {
	Found bool `json:"found"`
}
