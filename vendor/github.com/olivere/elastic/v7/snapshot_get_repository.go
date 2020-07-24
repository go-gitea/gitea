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

// SnapshotGetRepositoryService reads a snapshot repository.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
// for details.
type SnapshotGetRepositoryService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository    []string
	local         *bool
	masterTimeout string
}

// NewSnapshotGetRepositoryService creates a new SnapshotGetRepositoryService.
func NewSnapshotGetRepositoryService(client *Client) *SnapshotGetRepositoryService {
	return &SnapshotGetRepositoryService{
		client:     client,
		repository: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotGetRepositoryService) Pretty(pretty bool) *SnapshotGetRepositoryService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotGetRepositoryService) Human(human bool) *SnapshotGetRepositoryService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotGetRepositoryService) ErrorTrace(errorTrace bool) *SnapshotGetRepositoryService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotGetRepositoryService) FilterPath(filterPath ...string) *SnapshotGetRepositoryService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotGetRepositoryService) Header(name string, value string) *SnapshotGetRepositoryService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotGetRepositoryService) Headers(headers http.Header) *SnapshotGetRepositoryService {
	s.headers = headers
	return s
}

// Repository is the list of repository names.
func (s *SnapshotGetRepositoryService) Repository(repositories ...string) *SnapshotGetRepositoryService {
	s.repository = append(s.repository, repositories...)
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve the state from master node (default: false).
func (s *SnapshotGetRepositoryService) Local(local bool) *SnapshotGetRepositoryService {
	s.local = &local
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *SnapshotGetRepositoryService) MasterTimeout(masterTimeout string) *SnapshotGetRepositoryService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *SnapshotGetRepositoryService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.repository) > 0 {
		path, err = uritemplates.Expand("/_snapshot/{repository}", map[string]string{
			"repository": strings.Join(s.repository, ","),
		})
	} else {
		path = "/_snapshot"
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
	if v := s.local; v != nil {
		params.Set("local", fmt.Sprint(*v))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *SnapshotGetRepositoryService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *SnapshotGetRepositoryService) Do(ctx context.Context) (SnapshotGetRepositoryResponse, error) {
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
	var ret SnapshotGetRepositoryResponse
	if err := json.Unmarshal(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// SnapshotGetRepositoryResponse is the response of SnapshotGetRepositoryService.Do.
type SnapshotGetRepositoryResponse map[string]*SnapshotRepositoryMetaData

// SnapshotRepositoryMetaData contains all information about
// a single snapshot repository.
type SnapshotRepositoryMetaData struct {
	Type     string                 `json:"type"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}
