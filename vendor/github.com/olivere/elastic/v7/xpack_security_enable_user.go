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

// XPackSecurityEnableUserService retrieves a user by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/security-api-get-user.html.
type XPackSecurityEnableUserService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	username string
	refresh  string
}

// NewXPackSecurityEnableUserService creates a new XPackSecurityEnableUserService.
func NewXPackSecurityEnableUserService(client *Client) *XPackSecurityEnableUserService {
	return &XPackSecurityEnableUserService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityEnableUserService) Pretty(pretty bool) *XPackSecurityEnableUserService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityEnableUserService) Human(human bool) *XPackSecurityEnableUserService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityEnableUserService) ErrorTrace(errorTrace bool) *XPackSecurityEnableUserService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityEnableUserService) FilterPath(filterPath ...string) *XPackSecurityEnableUserService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityEnableUserService) Header(name string, value string) *XPackSecurityEnableUserService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityEnableUserService) Headers(headers http.Header) *XPackSecurityEnableUserService {
	s.headers = headers
	return s
}

// Username is name of the user to enable.
func (s *XPackSecurityEnableUserService) Username(username string) *XPackSecurityEnableUserService {
	s.username = username
	return s
}

// Refresh specifies if and how to wait for refreshing the shards after the request.
// Possible values are "true" (default), "false" and "wait_for", all of type string.
func (s *XPackSecurityEnableUserService) Refresh(refresh string) *XPackSecurityEnableUserService {
	s.refresh = refresh
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityEnableUserService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_security/user/{username}/_enable", map[string]string{
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
func (s *XPackSecurityEnableUserService) Validate() error {
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
func (s *XPackSecurityEnableUserService) Do(ctx context.Context) (*XPackSecurityEnableUserResponse, error) {
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
		Method: "PUT",
		Path:   path,
		Params: params,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackSecurityEnableUserResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackSecurityEnableUserResponse is the response of XPackSecurityEnableUserService.Do.
type XPackSecurityEnableUserResponse struct {
}
