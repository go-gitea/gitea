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

// IngestDeletePipelineService deletes pipelines by ID.
// It is documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/delete-pipeline-api.html.
type IngestDeletePipelineService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	masterTimeout string
	timeout       string
}

// NewIngestDeletePipelineService creates a new IngestDeletePipelineService.
func NewIngestDeletePipelineService(client *Client) *IngestDeletePipelineService {
	return &IngestDeletePipelineService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IngestDeletePipelineService) Pretty(pretty bool) *IngestDeletePipelineService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IngestDeletePipelineService) Human(human bool) *IngestDeletePipelineService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IngestDeletePipelineService) ErrorTrace(errorTrace bool) *IngestDeletePipelineService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IngestDeletePipelineService) FilterPath(filterPath ...string) *IngestDeletePipelineService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IngestDeletePipelineService) Header(name string, value string) *IngestDeletePipelineService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IngestDeletePipelineService) Headers(headers http.Header) *IngestDeletePipelineService {
	s.headers = headers
	return s
}

// Id is documented as: Pipeline ID.
func (s *IngestDeletePipelineService) Id(id string) *IngestDeletePipelineService {
	s.id = id
	return s
}

// MasterTimeout is documented as: Explicit operation timeout for connection to master node.
func (s *IngestDeletePipelineService) MasterTimeout(masterTimeout string) *IngestDeletePipelineService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout is documented as: Explicit operation timeout.
func (s *IngestDeletePipelineService) Timeout(timeout string) *IngestDeletePipelineService {
	s.timeout = timeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IngestDeletePipelineService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_ingest/pipeline/{id}", map[string]string{
		"id": s.id,
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
func (s *IngestDeletePipelineService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IngestDeletePipelineService) Do(ctx context.Context) (*IngestDeletePipelineResponse, error) {
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
	ret := new(IngestDeletePipelineResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IngestDeletePipelineResponse is the response of IngestDeletePipelineService.Do.
type IngestDeletePipelineResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
