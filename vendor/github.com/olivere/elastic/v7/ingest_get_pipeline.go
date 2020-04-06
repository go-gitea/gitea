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

// IngestGetPipelineService returns pipelines based on ID.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/get-pipeline-api.html
// for documentation.
type IngestGetPipelineService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            []string
	masterTimeout string
}

// NewIngestGetPipelineService creates a new IngestGetPipelineService.
func NewIngestGetPipelineService(client *Client) *IngestGetPipelineService {
	return &IngestGetPipelineService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IngestGetPipelineService) Pretty(pretty bool) *IngestGetPipelineService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IngestGetPipelineService) Human(human bool) *IngestGetPipelineService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IngestGetPipelineService) ErrorTrace(errorTrace bool) *IngestGetPipelineService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IngestGetPipelineService) FilterPath(filterPath ...string) *IngestGetPipelineService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IngestGetPipelineService) Header(name string, value string) *IngestGetPipelineService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IngestGetPipelineService) Headers(headers http.Header) *IngestGetPipelineService {
	s.headers = headers
	return s
}

// Id is a list of pipeline ids. Wildcards supported.
func (s *IngestGetPipelineService) Id(id ...string) *IngestGetPipelineService {
	s.id = append(s.id, id...)
	return s
}

// MasterTimeout is an explicit operation timeout for connection to master node.
func (s *IngestGetPipelineService) MasterTimeout(masterTimeout string) *IngestGetPipelineService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IngestGetPipelineService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	// Build URL
	if len(s.id) > 0 {
		path, err = uritemplates.Expand("/_ingest/pipeline/{id}", map[string]string{
			"id": strings.Join(s.id, ","),
		})
	} else {
		path = "/_ingest/pipeline"
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
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IngestGetPipelineService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IngestGetPipelineService) Do(ctx context.Context) (IngestGetPipelineResponse, error) {
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
	var ret IngestGetPipelineResponse
	if err := json.Unmarshal(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IngestGetPipelineResponse is the response of IngestGetPipelineService.Do.
type IngestGetPipelineResponse map[string]*IngestGetPipeline

// IngestGetPipeline describes a specific ingest pipeline, its
// processors etc.
type IngestGetPipeline struct {
	Description string                   `json:"description"`
	Processors  []map[string]interface{} `json:"processors"`
	Version     int64                    `json:"version,omitempty"`
	OnFailure   []map[string]interface{} `json:"on_failure,omitempty"`
}
