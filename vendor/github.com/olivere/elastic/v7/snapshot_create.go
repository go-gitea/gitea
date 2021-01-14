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

// SnapshotCreateService is documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html.
type SnapshotCreateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository        string
	snapshot          string
	masterTimeout     string
	waitForCompletion *bool
	bodyJson          interface{}
	bodyString        string
}

// NewSnapshotCreateService creates a new SnapshotCreateService.
func NewSnapshotCreateService(client *Client) *SnapshotCreateService {
	return &SnapshotCreateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotCreateService) Pretty(pretty bool) *SnapshotCreateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotCreateService) Human(human bool) *SnapshotCreateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotCreateService) ErrorTrace(errorTrace bool) *SnapshotCreateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotCreateService) FilterPath(filterPath ...string) *SnapshotCreateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotCreateService) Header(name string, value string) *SnapshotCreateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotCreateService) Headers(headers http.Header) *SnapshotCreateService {
	s.headers = headers
	return s
}

// Repository is the repository name.
func (s *SnapshotCreateService) Repository(repository string) *SnapshotCreateService {
	s.repository = repository
	return s
}

// Snapshot is the snapshot name.
func (s *SnapshotCreateService) Snapshot(snapshot string) *SnapshotCreateService {
	s.snapshot = snapshot
	return s
}

// MasterTimeout is documented as: Explicit operation timeout for connection to master node.
func (s *SnapshotCreateService) MasterTimeout(masterTimeout string) *SnapshotCreateService {
	s.masterTimeout = masterTimeout
	return s
}

// WaitForCompletion is documented as: Should this request wait until the operation has completed before returning.
func (s *SnapshotCreateService) WaitForCompletion(waitForCompletion bool) *SnapshotCreateService {
	s.waitForCompletion = &waitForCompletion
	return s
}

// BodyJson is documented as: The snapshot definition.
func (s *SnapshotCreateService) BodyJson(body interface{}) *SnapshotCreateService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: The snapshot definition.
func (s *SnapshotCreateService) BodyString(body string) *SnapshotCreateService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *SnapshotCreateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_snapshot/{repository}/{snapshot}", map[string]string{
		"snapshot":   s.snapshot,
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
	if v := s.waitForCompletion; v != nil {
		params.Set("wait_for_completion", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *SnapshotCreateService) Validate() error {
	var invalid []string
	if s.repository == "" {
		invalid = append(invalid, "Repository")
	}
	if s.snapshot == "" {
		invalid = append(invalid, "Snapshot")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *SnapshotCreateService) Do(ctx context.Context) (*SnapshotCreateResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else {
		body = s.bodyString
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(SnapshotCreateResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// SnapshotShardFailure stores information about failures that occurred during shard snapshotting process.
type SnapshotShardFailure struct {
	Index     string `json:"index"`
	IndexUUID string `json:"index_uuid"`
	ShardID   int    `json:"shard_id"`
	Reason    string `json:"reason"`
	NodeID    string `json:"node_id"`
	Status    string `json:"status"`
}

// SnapshotCreateResponse is the response of SnapshotCreateService.Do.
type SnapshotCreateResponse struct {
	// Accepted indicates whether the request was accepted by elasticsearch.
	// It's available when waitForCompletion is false.
	Accepted *bool `json:"accepted"`

	// Snapshot is available when waitForCompletion is true.
	Snapshot *Snapshot `json:"snapshot"`
}
