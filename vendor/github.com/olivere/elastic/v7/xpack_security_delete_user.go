// Copyright 2012-2019 Oliver Eilhard. All rights reserved.
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

// XPackSecurityDeleteUserService delete a user by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.4/security-api-delete-user.html.
type XPackSecurityDeleteUserService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	username string
	refresh  string
}

// NewXPackSecurityDeleteUserService creates a new XPackSecurityDeleteUserService.
func NewXPackSecurityDeleteUserService(client *Client) *XPackSecurityDeleteUserService {
	return &XPackSecurityDeleteUserService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityDeleteUserService) Pretty(pretty bool) *XPackSecurityDeleteUserService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityDeleteUserService) Human(human bool) *XPackSecurityDeleteUserService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityDeleteUserService) ErrorTrace(errorTrace bool) *XPackSecurityDeleteUserService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityDeleteUserService) FilterPath(filterPath ...string) *XPackSecurityDeleteUserService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityDeleteUserService) Header(name string, value string) *XPackSecurityDeleteUserService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityDeleteUserService) Headers(headers http.Header) *XPackSecurityDeleteUserService {
	s.headers = headers
	return s
}

// Username is name of the user to delete.
func (s *XPackSecurityDeleteUserService) Username(username string) *XPackSecurityDeleteUserService {
	s.username = username
	return s
}

// Refresh specifies if and how to wait for refreshing the shards after the request.
// Possible values are "true" (default), "false" and "wait_for", all of type string.
func (s *XPackSecurityDeleteUserService) Refresh(refresh string) *XPackSecurityDeleteUserService {
	s.refresh = refresh
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityDeleteUserService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_security/user/{username}", map[string]string{
		"username": s.username,
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
	if v := s.refresh; v != "" {
		params.Set("refresh", v)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackSecurityDeleteUserService) Validate() error {
	var invalid []string
	if s.username == "" {
		invalid = append(invalid, "Username")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackSecurityDeleteUserService) Do(ctx context.Context) (*XPackSecurityDeleteUserResponse, error) {
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
	ret := new(XPackSecurityDeleteUserResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackSecurityDeleteUserResponse is the response of XPackSecurityDeleteUserService.Do.
type XPackSecurityDeleteUserResponse struct {
	Found bool `json:"found"`
}
