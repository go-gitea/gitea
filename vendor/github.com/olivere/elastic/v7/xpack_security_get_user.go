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

// XPackSecurityGetUserService retrieves a user by its name.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/security-api-get-user.html.
type XPackSecurityGetUserService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	usernames []string
}

// NewXPackSecurityGetUserService creates a new XPackSecurityGetUserService.
func NewXPackSecurityGetUserService(client *Client) *XPackSecurityGetUserService {
	return &XPackSecurityGetUserService{
		client: client,
	}
}

// Pretty indicates that the JSON response be indented and human readable.
func (s *XPackSecurityGetUserService) Pretty(pretty bool) *XPackSecurityGetUserService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityGetUserService) Human(human bool) *XPackSecurityGetUserService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityGetUserService) ErrorTrace(errorTrace bool) *XPackSecurityGetUserService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityGetUserService) FilterPath(filterPath ...string) *XPackSecurityGetUserService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityGetUserService) Header(name string, value string) *XPackSecurityGetUserService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityGetUserService) Headers(headers http.Header) *XPackSecurityGetUserService {
	s.headers = headers
	return s
}

// Usernames are the names of one or more users to retrieve.
func (s *XPackSecurityGetUserService) Usernames(usernames ...string) *XPackSecurityGetUserService {
	for _, username := range usernames {
		if v := strings.TrimSpace(username); v != "" {
			s.usernames = append(s.usernames, v)
		}
	}
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityGetUserService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)
	if len(s.usernames) > 0 {
		path, err = uritemplates.Expand("/_security/user/{username}", map[string]string{
			"username": strings.Join(s.usernames, ","),
		})
	} else {
		path = "/_security/user"
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

	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackSecurityGetUserService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *XPackSecurityGetUserService) Do(ctx context.Context) (*XPackSecurityGetUserResponse, error) {
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
	ret := XPackSecurityGetUserResponse{}
	if err := json.Unmarshal(res.Body, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// XPackSecurityGetUserResponse is the response of XPackSecurityGetUserService.Do.
type XPackSecurityGetUserResponse map[string]XPackSecurityUser

// XPackSecurityUser is the user object.
//
// The Java source for this struct is defined here:
// https://github.com/elastic/elasticsearch/blob/7.3/x-pack/plugin/core/src/main/java/org/elasticsearch/xpack/core/security/user/User.java
type XPackSecurityUser struct {
	Username string                 `json:"username"`
	Roles    []string               `json:"roles"`
	Fullname string                 `json:"full_name"`
	Email    string                 `json:"email"`
	Metadata map[string]interface{} `json:"metadata"`
	Enabled  bool                   `json:"enabled"`
}
