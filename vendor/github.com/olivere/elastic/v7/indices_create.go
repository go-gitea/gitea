// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesCreateService creates a new index.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-create-index.html
// for details.
type IndicesCreateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index         string
	timeout       string
	masterTimeout string
	bodyJson      interface{}
	bodyString    string
}

// NewIndicesCreateService returns a new IndicesCreateService.
func NewIndicesCreateService(client *Client) *IndicesCreateService {
	return &IndicesCreateService{client: client}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesCreateService) Pretty(pretty bool) *IndicesCreateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesCreateService) Human(human bool) *IndicesCreateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesCreateService) ErrorTrace(errorTrace bool) *IndicesCreateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesCreateService) FilterPath(filterPath ...string) *IndicesCreateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesCreateService) Header(name string, value string) *IndicesCreateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesCreateService) Headers(headers http.Header) *IndicesCreateService {
	s.headers = headers
	return s
}

// Index is the name of the index to create.
func (s *IndicesCreateService) Index(index string) *IndicesCreateService {
	s.index = index
	return s
}

// Timeout the explicit operation timeout, e.g. "5s".
func (s *IndicesCreateService) Timeout(timeout string) *IndicesCreateService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesCreateService) MasterTimeout(masterTimeout string) *IndicesCreateService {
	s.masterTimeout = masterTimeout
	return s
}

// Body specifies the configuration of the index as a string.
// It is an alias for BodyString.
func (s *IndicesCreateService) Body(body string) *IndicesCreateService {
	s.bodyString = body
	return s
}

// BodyString specifies the configuration of the index as a string.
func (s *IndicesCreateService) BodyString(body string) *IndicesCreateService {
	s.bodyString = body
	return s
}

// BodyJson specifies the configuration of the index. The interface{} will
// be serializes as a JSON document, so use a map[string]interface{}.
func (s *IndicesCreateService) BodyJson(body interface{}) *IndicesCreateService {
	s.bodyJson = body
	return s
}

// Do executes the operation.
func (s *IndicesCreateService) Do(ctx context.Context) (*IndicesCreateResult, error) {
	if s.index == "" {
		return nil, errors.New("missing index name")
	}

	// Build url
	path, err := uritemplates.Expand("/{index}", map[string]string{
		"index": s.index,
	})
	if err != nil {
		return nil, err
	}

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

	// Setup HTTP request body
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else {
		body = s.bodyString
	}

	// Get response
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

	ret := new(IndicesCreateResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a create index request.

// IndicesCreateResult is the outcome of creating a new index.
type IndicesCreateResult struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
