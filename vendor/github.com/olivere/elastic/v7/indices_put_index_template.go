// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesPutIndexTemplateService creates or updates index templates.
//
// Index templates have changed during in 7.8 update of Elasticsearch.
// This service implements the new version (7.8 or higher) for managing
// index templates. If you want the v1/legacy version, please see e.g.
// IndicesPutTemplateService and friends.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/indices-put-template.html
// for more details on this API.
type IndicesPutIndexTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          string
	create        *bool
	cause         string
	masterTimeout string

	bodyJson   interface{}
	bodyString string
}

// NewIndicesPutIndexTemplateService creates a new IndicesPutIndexTemplateService.
func NewIndicesPutIndexTemplateService(client *Client) *IndicesPutIndexTemplateService {
	return &IndicesPutIndexTemplateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesPutIndexTemplateService) Pretty(pretty bool) *IndicesPutIndexTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesPutIndexTemplateService) Human(human bool) *IndicesPutIndexTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesPutIndexTemplateService) ErrorTrace(errorTrace bool) *IndicesPutIndexTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesPutIndexTemplateService) FilterPath(filterPath ...string) *IndicesPutIndexTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesPutIndexTemplateService) Header(name string, value string) *IndicesPutIndexTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesPutIndexTemplateService) Headers(headers http.Header) *IndicesPutIndexTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the index template.
func (s *IndicesPutIndexTemplateService) Name(name string) *IndicesPutIndexTemplateService {
	s.name = name
	return s
}

// Create indicates whether the index template should only be added if
// new or can also replace an existing one.
func (s *IndicesPutIndexTemplateService) Create(create bool) *IndicesPutIndexTemplateService {
	s.create = &create
	return s
}

// Cause is the user-defined reason for creating/updating the the index template.
func (s *IndicesPutIndexTemplateService) Cause(cause string) *IndicesPutIndexTemplateService {
	s.cause = cause
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesPutIndexTemplateService) MasterTimeout(masterTimeout string) *IndicesPutIndexTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// BodyJson is the index template definition as a JSON serializable
// type, e.g. map[string]interface{}.
func (s *IndicesPutIndexTemplateService) BodyJson(body interface{}) *IndicesPutIndexTemplateService {
	s.bodyJson = body
	return s
}

// BodyString is the index template definition as a raw string.
func (s *IndicesPutIndexTemplateService) BodyString(body string) *IndicesPutIndexTemplateService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesPutIndexTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_index_template/{name}", map[string]string{
		"name": s.name,
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
	if s.create != nil {
		params.Set("create", fmt.Sprint(*s.create))
	}
	if s.cause != "" {
		params.Set("cause", s.cause)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesPutIndexTemplateService) Validate() error {
	var invalid []string
	if s.name == "" {
		invalid = append(invalid, "Name")
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
func (s *IndicesPutIndexTemplateService) Do(ctx context.Context) (*IndicesPutIndexTemplateResponse, error) {
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
	ret := new(IndicesPutIndexTemplateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesPutIndexTemplateResponse is the response of IndicesPutIndexTemplateService.Do.
type IndicesPutIndexTemplateResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
