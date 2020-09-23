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

// IngestSimulatePipelineService executes a specific pipeline against the set of
// documents provided in the body of the request.
//
// The API is documented at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/simulate-pipeline-api.html.
type IngestSimulatePipelineService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id         string
	verbose    *bool
	bodyJson   interface{}
	bodyString string
}

// NewIngestSimulatePipelineService creates a new IngestSimulatePipeline.
func NewIngestSimulatePipelineService(client *Client) *IngestSimulatePipelineService {
	return &IngestSimulatePipelineService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IngestSimulatePipelineService) Pretty(pretty bool) *IngestSimulatePipelineService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IngestSimulatePipelineService) Human(human bool) *IngestSimulatePipelineService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IngestSimulatePipelineService) ErrorTrace(errorTrace bool) *IngestSimulatePipelineService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IngestSimulatePipelineService) FilterPath(filterPath ...string) *IngestSimulatePipelineService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IngestSimulatePipelineService) Header(name string, value string) *IngestSimulatePipelineService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IngestSimulatePipelineService) Headers(headers http.Header) *IngestSimulatePipelineService {
	s.headers = headers
	return s
}

// Id specifies the pipeline ID.
func (s *IngestSimulatePipelineService) Id(id string) *IngestSimulatePipelineService {
	s.id = id
	return s
}

// Verbose mode. Display data output for each processor in executed pipeline.
func (s *IngestSimulatePipelineService) Verbose(verbose bool) *IngestSimulatePipelineService {
	s.verbose = &verbose
	return s
}

// BodyJson is the ingest definition, defined as a JSON-serializable simulate
// definition. Use e.g. a map[string]interface{} here.
func (s *IngestSimulatePipelineService) BodyJson(body interface{}) *IngestSimulatePipelineService {
	s.bodyJson = body
	return s
}

// BodyString is the simulate definition, defined as a string.
func (s *IngestSimulatePipelineService) BodyString(body string) *IngestSimulatePipelineService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IngestSimulatePipelineService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	// Build URL
	if s.id != "" {
		path, err = uritemplates.Expand("/_ingest/pipeline/{id}/_simulate", map[string]string{
			"id": s.id,
		})
	} else {
		path = "/_ingest/pipeline/_simulate"
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
	if v := s.verbose; v != nil {
		params.Set("verbose", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IngestSimulatePipelineService) Validate() error {
	var invalid []string
	if s.bodyString == "" && s.bodyJson == nil {
		invalid = append(invalid, "BodyJson")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IngestSimulatePipelineService) Do(ctx context.Context) (*IngestSimulatePipelineResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(IngestSimulatePipelineResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IngestSimulatePipelineResponse is the response of IngestSimulatePipeline.Do.
type IngestSimulatePipelineResponse struct {
	Docs []*IngestSimulateDocumentResult `json:"docs"`
}

type IngestSimulateDocumentResult struct {
	Doc              map[string]interface{}           `json:"doc"`
	ProcessorResults []*IngestSimulateProcessorResult `json:"processor_results"`
}

type IngestSimulateProcessorResult struct {
	ProcessorTag string                 `json:"tag"`
	Doc          map[string]interface{} `json:"doc"`
}
