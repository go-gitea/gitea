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

// SnapshotDeleteRepositoryService deletes a snapshot repository.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
// for details.
type SnapshotDeleteRepositoryService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository    []string
	masterTimeout string
	timeout       string
}

// NewSnapshotDeleteRepositoryService creates a new SnapshotDeleteRepositoryService.
func NewSnapshotDeleteRepositoryService(client *Client) *SnapshotDeleteRepositoryService {
	return &SnapshotDeleteRepositoryService{
		client:     client,
		repository: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotDeleteRepositoryService) Pretty(pretty bool) *SnapshotDeleteRepositoryService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotDeleteRepositoryService) Human(human bool) *SnapshotDeleteRepositoryService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotDeleteRepositoryService) ErrorTrace(errorTrace bool) *SnapshotDeleteRepositoryService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotDeleteRepositoryService) FilterPath(filterPath ...string) *SnapshotDeleteRepositoryService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotDeleteRepositoryService) Header(name string, value string) *SnapshotDeleteRepositoryService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotDeleteRepositoryService) Headers(headers http.Header) *SnapshotDeleteRepositoryService {
	s.headers = headers
	return s
}

// Repository is the list of repository names.
func (s *SnapshotDeleteRepositoryService) Repository(repositories ...string) *SnapshotDeleteRepositoryService {
	s.repository = append(s.repository, repositories...)
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *SnapshotDeleteRepositoryService) MasterTimeout(masterTimeout string) *SnapshotDeleteRepositoryService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout is an explicit operation timeout.
func (s *SnapshotDeleteRepositoryService) Timeout(timeout string) *SnapshotDeleteRepositoryService {
	s.timeout = timeout
	return s
}

// buildURL builds the URL for the operation.
func (s *SnapshotDeleteRepositoryService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_snapshot/{repository}", map[string]string{
		"repository": strings.Join(s.repository, ","),
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
func (s *SnapshotDeleteRepositoryService) Validate() error {
	var invalid []string
	if len(s.repository) == 0 {
		invalid = append(invalid, "Repository")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *SnapshotDeleteRepositoryService) Do(ctx context.Context) (*SnapshotDeleteRepositoryResponse, error) {
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
	ret := new(SnapshotDeleteRepositoryResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// SnapshotDeleteRepositoryResponse is the response of SnapshotDeleteRepositoryService.Do.
type SnapshotDeleteRepositoryResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
