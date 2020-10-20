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

// XPackSecurityPutUserService adds a user.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.4/security-api-put-user.html.
type XPackSecurityPutUserService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	username string
	refresh  string

	user *XPackSecurityPutUserRequest
	body interface{}
}

// NewXPackSecurityPutUserService creates a new XPackSecurityPutUserService.
func NewXPackSecurityPutUserService(client *Client) *XPackSecurityPutUserService {
	return &XPackSecurityPutUserService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackSecurityPutUserService) Pretty(pretty bool) *XPackSecurityPutUserService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackSecurityPutUserService) Human(human bool) *XPackSecurityPutUserService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackSecurityPutUserService) ErrorTrace(errorTrace bool) *XPackSecurityPutUserService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackSecurityPutUserService) FilterPath(filterPath ...string) *XPackSecurityPutUserService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackSecurityPutUserService) Header(name string, value string) *XPackSecurityPutUserService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackSecurityPutUserService) Headers(headers http.Header) *XPackSecurityPutUserService {
	s.headers = headers
	return s
}

// Username is the name of the user to add.
func (s *XPackSecurityPutUserService) Username(username string) *XPackSecurityPutUserService {
	s.username = username
	return s
}

// User specifies the data of the new user.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.4/security-api-put-user.html
// for details.
func (s *XPackSecurityPutUserService) User(user *XPackSecurityPutUserRequest) *XPackSecurityPutUserService {
	s.user = user
	return s
}

// Refresh specifies if and how to wait for refreshing the shards after the request.
// Possible values are "true" (default), "false" and "wait_for", all of type string.
func (s *XPackSecurityPutUserService) Refresh(refresh string) *XPackSecurityPutUserService {
	s.refresh = refresh
	return s
}

// Body specifies the user. Use a string or a type that will get serialized as JSON.
func (s *XPackSecurityPutUserService) Body(body interface{}) *XPackSecurityPutUserService {
	s.body = body
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackSecurityPutUserService) buildURL() (string, url.Values, error) {
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
func (s *XPackSecurityPutUserService) Validate() error {
	var invalid []string
	if s.username == "" {
		invalid = append(invalid, "Username")
	}
	if s.user == nil && s.body == nil {
		invalid = append(invalid, "User")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackSecurityPutUserService) Do(ctx context.Context) (*XPackSecurityPutUserResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	var body interface{}
	if s.user != nil {
		body = s.user
	} else {
		body = s.body
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method: "PUT",
		Path:   path,
		Params: params,
		Body:   body,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackSecurityPutUserResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackSecurityPutUserRequest specifies the data required/allowed to add
// a new user.
type XPackSecurityPutUserRequest struct {
	Enabled      bool                   `json:"enabled"`
	Email        string                 `json:"email,omitempty"`
	FullName     string                 `json:"full_name,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Password     string                 `json:"password,omitempty"`
	PasswordHash string                 `json:"password_hash,omitempty"`
	Roles        []string               `json:"roles"`
}

// XPackSecurityPutUserResponse is the response of XPackSecurityPutUserService.Do.
type XPackSecurityPutUserResponse struct {
	User XPackSecurityPutUser `json:"user"`
}

// XPackSecurityPutUser is the response containing the creation information
type XPackSecurityPutUser struct {
	Created bool `json:"created"`
}
