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

// IndicesPutTemplateService creates or updates templates.
//
// Index templates have changed during in 7.8 update of Elasticsearch.
// This service implements the legacy version (7.7 or lower). If you want
// the new version, please use the IndicesPutIndexTemplateService.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/indices-templates-v1.html
// for more details.
type IndicesPutTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          string
	cause         string
	order         interface{}
	version       *int
	create        *bool
	timeout       string
	masterTimeout string
	flatSettings  *bool
	bodyJson      interface{}
	bodyString    string
}

// NewIndicesPutTemplateService creates a new IndicesPutTemplateService.
func NewIndicesPutTemplateService(client *Client) *IndicesPutTemplateService {
	return &IndicesPutTemplateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesPutTemplateService) Pretty(pretty bool) *IndicesPutTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesPutTemplateService) Human(human bool) *IndicesPutTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesPutTemplateService) ErrorTrace(errorTrace bool) *IndicesPutTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesPutTemplateService) FilterPath(filterPath ...string) *IndicesPutTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesPutTemplateService) Header(name string, value string) *IndicesPutTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesPutTemplateService) Headers(headers http.Header) *IndicesPutTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the index template.
func (s *IndicesPutTemplateService) Name(name string) *IndicesPutTemplateService {
	s.name = name
	return s
}

// Cause describes the cause for this index template creation. This is currently
// undocumented, but part of the Java source.
func (s *IndicesPutTemplateService) Cause(cause string) *IndicesPutTemplateService {
	s.cause = cause
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesPutTemplateService) Timeout(timeout string) *IndicesPutTemplateService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesPutTemplateService) MasterTimeout(masterTimeout string) *IndicesPutTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// FlatSettings indicates whether to return settings in flat format (default: false).
func (s *IndicesPutTemplateService) FlatSettings(flatSettings bool) *IndicesPutTemplateService {
	s.flatSettings = &flatSettings
	return s
}

// Order is the order for this template when merging multiple matching ones
// (higher numbers are merged later, overriding the lower numbers).
func (s *IndicesPutTemplateService) Order(order interface{}) *IndicesPutTemplateService {
	s.order = order
	return s
}

// Version sets the version number for this template.
func (s *IndicesPutTemplateService) Version(version int) *IndicesPutTemplateService {
	s.version = &version
	return s
}

// Create indicates whether the index template should only be added if
// new or can also replace an existing one.
func (s *IndicesPutTemplateService) Create(create bool) *IndicesPutTemplateService {
	s.create = &create
	return s
}

// BodyJson is documented as: The template definition.
func (s *IndicesPutTemplateService) BodyJson(body interface{}) *IndicesPutTemplateService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: The template definition.
func (s *IndicesPutTemplateService) BodyString(body string) *IndicesPutTemplateService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesPutTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_template/{name}", map[string]string{
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
	if s.order != nil {
		params.Set("order", fmt.Sprintf("%v", s.order))
	}
	if s.version != nil {
		params.Set("version", fmt.Sprintf("%v", *s.version))
	}
	if s.create != nil {
		params.Set("create", fmt.Sprintf("%v", *s.create))
	}
	if s.cause != "" {
		params.Set("cause", s.cause)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesPutTemplateService) Validate() error {
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
func (s *IndicesPutTemplateService) Do(ctx context.Context) (*IndicesPutTemplateResponse, error) {
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
	ret := new(IndicesPutTemplateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesPutTemplateResponse is the response of IndicesPutTemplateService.Do.
type IndicesPutTemplateResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
