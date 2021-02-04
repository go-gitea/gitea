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

// IndicesGetIndexTemplateService returns an index template.
//
// Index templates have changed during in 7.8 update of Elasticsearch.
// This service implements the new version (7.8 or later). If you want
// the old version, please use the IndicesGetTemplateService.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/indices-get-template.html
// for more details.
type IndicesGetIndexTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          []string
	masterTimeout string
	flatSettings  *bool
	local         *bool
}

// NewIndicesGetIndexTemplateService creates a new IndicesGetIndexTemplateService.
func NewIndicesGetIndexTemplateService(client *Client) *IndicesGetIndexTemplateService {
	return &IndicesGetIndexTemplateService{
		client: client,
		name:   make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetIndexTemplateService) Pretty(pretty bool) *IndicesGetIndexTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetIndexTemplateService) Human(human bool) *IndicesGetIndexTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetIndexTemplateService) ErrorTrace(errorTrace bool) *IndicesGetIndexTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetIndexTemplateService) FilterPath(filterPath ...string) *IndicesGetIndexTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetIndexTemplateService) Header(name string, value string) *IndicesGetIndexTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetIndexTemplateService) Headers(headers http.Header) *IndicesGetIndexTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the index template.
func (s *IndicesGetIndexTemplateService) Name(name ...string) *IndicesGetIndexTemplateService {
	s.name = append(s.name, name...)
	return s
}

// FlatSettings is returns settings in flat format (default: false).
func (s *IndicesGetIndexTemplateService) FlatSettings(flatSettings bool) *IndicesGetIndexTemplateService {
	s.flatSettings = &flatSettings
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *IndicesGetIndexTemplateService) Local(local bool) *IndicesGetIndexTemplateService {
	s.local = &local
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesGetIndexTemplateService) MasterTimeout(masterTimeout string) *IndicesGetIndexTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetIndexTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.name) > 0 {
		path, err = uritemplates.Expand("/_index_template/{name}", map[string]string{
			"name": strings.Join(s.name, ","),
		})
	} else {
		path = "/_template"
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
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetIndexTemplateService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesGetIndexTemplateService) Do(ctx context.Context) (*IndicesGetIndexTemplateResponse, error) {
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
	var ret *IndicesGetIndexTemplateResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesGetIndexTemplateResponse is the response of IndicesGetIndexTemplateService.Do.
type IndicesGetIndexTemplateResponse struct {
	IndexTemplates []IndicesGetIndexTemplates `json:"index_templates"`
}

type IndicesGetIndexTemplates struct {
	Name          string                   `json:"name"`
	IndexTemplate *IndicesGetIndexTemplate `json:"index_template"`
}

type IndicesGetIndexTemplate struct {
	IndexPatterns []string                     `json:"index_patterns,omitempty"`
	ComposedOf    []string                     `json:"composed_of,omitempty"`
	Priority      int                          `json:"priority,omitempty"`
	Version       int                          `json:"version,omitempty"`
	Template      *IndicesGetIndexTemplateData `json:"template,omitempty"`
}

type IndicesGetIndexTemplateData struct {
	Settings map[string]interface{} `json:"settings,omitempty"`
	Mappings map[string]interface{} `json:"mappings,omitempty"`
	Aliases  map[string]interface{} `json:"aliases,omitempty"`
}
