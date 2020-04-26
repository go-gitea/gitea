// Copyright 2012-present Oliver Eilhard. All rights reserved.
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

// SnapshotVerifyRepositoryService verifies a snapshop repository.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
// for details.
type SnapshotVerifyRepositoryService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository    string
	masterTimeout string
	timeout       string
}

// NewSnapshotVerifyRepositoryService creates a new SnapshotVerifyRepositoryService.
func NewSnapshotVerifyRepositoryService(client *Client) *SnapshotVerifyRepositoryService {
	return &SnapshotVerifyRepositoryService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotVerifyRepositoryService) Pretty(pretty bool) *SnapshotVerifyRepositoryService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotVerifyRepositoryService) Human(human bool) *SnapshotVerifyRepositoryService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotVerifyRepositoryService) ErrorTrace(errorTrace bool) *SnapshotVerifyRepositoryService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotVerifyRepositoryService) FilterPath(filterPath ...string) *SnapshotVerifyRepositoryService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotVerifyRepositoryService) Header(name string, value string) *SnapshotVerifyRepositoryService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotVerifyRepositoryService) Headers(headers http.Header) *SnapshotVerifyRepositoryService {
	s.headers = headers
	return s
}

// Repository specifies the repository name.
func (s *SnapshotVerifyRepositoryService) Repository(repository string) *SnapshotVerifyRepositoryService {
	s.repository = repository
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *SnapshotVerifyRepositoryService) MasterTimeout(masterTimeout string) *SnapshotVerifyRepositoryService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout is an explicit operation timeout.
func (s *SnapshotVerifyRepositoryService) Timeout(timeout string) *SnapshotVerifyRepositoryService {
	s.timeout = timeout
	return s
}

// buildURL builds the URL for the operation.
func (s *SnapshotVerifyRepositoryService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_snapshot/{repository}/_verify", map[string]string{
		"repository": s.repository,
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
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *SnapshotVerifyRepositoryService) Validate() error {
	var invalid []string
	if s.repository == "" {
		invalid = append(invalid, "Repository")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *SnapshotVerifyRepositoryService) Do(ctx context.Context) (*SnapshotVerifyRepositoryResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(SnapshotVerifyRepositoryResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// SnapshotVerifyRepositoryResponse is the response of SnapshotVerifyRepositoryService.Do.
type SnapshotVerifyRepositoryResponse struct {
	Nodes map[string]*SnapshotVerifyRepositoryNode `json:"nodes"`
}

type SnapshotVerifyRepositoryNode struct {
	Name string `json:"name"`
}
