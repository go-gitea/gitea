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

// IngestPutPipelineService adds pipelines and updates existing pipelines in
// the cluster.
//
// It is documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/put-pipeline-api.html.
type IngestPutPipelineService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	masterTimeout string
	timeout       string
	bodyJson      interface{}
	bodyString    string
}

// NewIngestPutPipelineService creates a new IngestPutPipelineService.
func NewIngestPutPipelineService(client *Client) *IngestPutPipelineService {
	return &IngestPutPipelineService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IngestPutPipelineService) Pretty(pretty bool) *IngestPutPipelineService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IngestPutPipelineService) Human(human bool) *IngestPutPipelineService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IngestPutPipelineService) ErrorTrace(errorTrace bool) *IngestPutPipelineService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IngestPutPipelineService) FilterPath(filterPath ...string) *IngestPutPipelineService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IngestPutPipelineService) Header(name string, value string) *IngestPutPipelineService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IngestPutPipelineService) Headers(headers http.Header) *IngestPutPipelineService {
	s.headers = headers
	return s
}

// Id is the pipeline ID.
func (s *IngestPutPipelineService) Id(id string) *IngestPutPipelineService {
	s.id = id
	return s
}

// MasterTimeout is an explicit operation timeout for connection to master node.
func (s *IngestPutPipelineService) MasterTimeout(masterTimeout string) *IngestPutPipelineService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout specifies an explicit operation timeout.
func (s *IngestPutPipelineService) Timeout(timeout string) *IngestPutPipelineService {
	s.timeout = timeout
	return s
}

// BodyJson is the ingest definition, defined as a JSON-serializable document.
// Use e.g. a map[string]interface{} here.
func (s *IngestPutPipelineService) BodyJson(body interface{}) *IngestPutPipelineService {
	s.bodyJson = body
	return s
}

// BodyString is the ingest definition, specified as a string.
func (s *IngestPutPipelineService) BodyString(body string) *IngestPutPipelineService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IngestPutPipelineService) buildURL() (string, url.Values, error) {
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
func (s *IngestPutPipelineService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.bodyString == "" && s.bodyJson == nil {
		invalid = append(invalid, "BodyJson")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IngestPutPipelineService) Do(ctx context.Context) (*IngestPutPipelineResponse, error) {
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
	ret := new(IngestPutPipelineResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IngestPutPipelineResponse is the response of IngestPutPipelineService.Do.
type IngestPutPipelineResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
