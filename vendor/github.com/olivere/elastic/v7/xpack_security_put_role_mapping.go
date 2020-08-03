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

// XPackSecurityPutRoleMappingService create or update a role mapping by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/security-api-put-role-mapping.html.
type XPackSecurityPutRoleMappingService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name string
	body interface{}
}

// NewXPackSecurityPutRoleMappingService creates a new XPackSecurityPutRoleMappingService.
func NewXPackSecurityPutRoleMappingService(client *Client) *XPackSecurityPutRoleMappingService {
	return &XPackSecurityPutRoleMappingService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityPutRoleMappingService) Pretty(pretty bool) *XPackSecurityPutRoleMappingService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityPutRoleMappingService) Human(human bool) *XPackSecurityPutRoleMappingService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityPutRoleMappingService) ErrorTrace(errorTrace bool) *XPackSecurityPutRoleMappingService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityPutRoleMappingService) FilterPath(filterPath ...string) *XPackSecurityPutRoleMappingService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityPutRoleMappingService) Header(name string, value string) *XPackSecurityPutRoleMappingService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityPutRoleMappingService) Headers(headers http.Header) *XPackSecurityPutRoleMappingService {
	s.headers = headers
	return s
}

// Name is name of the role mapping to create/update.
func (s *XPackSecurityPutRoleMappingService) Name(name string) *XPackSecurityPutRoleMappingService {
	s.name = name
	return s
}

// Body specifies the role mapping. Use a string or a type that will get serialized as JSON.
func (s *XPackSecurityPutRoleMappingService) Body(body interface{}) *XPackSecurityPutRoleMappingService {
	s.body = body
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityPutRoleMappingService) buildURL() (string, url.Values, error) {
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
func (s *XPackSecurityPutRoleMappingService) Validate() error {
	var invalid []string
	if s.name == "" {
		invalid = append(invalid, "Name")
	}
	if s.body == nil {
		invalid = append(invalid, "Body")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackSecurityPutRoleMappingService) Do(ctx context.Context) (*XPackSecurityPutRoleMappingResponse, error) {
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
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    s.body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackSecurityPutRoleMappingResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackSecurityPutRoleMappingResponse is the response of XPackSecurityPutRoleMappingService.Do.
type XPackSecurityPutRoleMappingResponse struct {
	Role_Mapping XPackSecurityPutRoleMapping
}

type XPackSecurityPutRoleMapping struct {
	Created bool `json:"created"`
}
