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

// IndicesGetTemplateService returns an index template (v1).
//
// Index templates have changed during in 7.8 update of Elasticsearch.
// This service implements the legacy version (7.7 or lower). If you want
// the new version, please use the IndicesGetIndexTemplateService.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.9/indices-get-template-v1.html
// for more details.
type IndicesGetTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name         []string
	flatSettings *bool
	local        *bool
}

// NewIndicesGetTemplateService creates a new IndicesGetTemplateService.
func NewIndicesGetTemplateService(client *Client) *IndicesGetTemplateService {
	return &IndicesGetTemplateService{
		client: client,
		name:   make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetTemplateService) Pretty(pretty bool) *IndicesGetTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetTemplateService) Human(human bool) *IndicesGetTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetTemplateService) ErrorTrace(errorTrace bool) *IndicesGetTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetTemplateService) FilterPath(filterPath ...string) *IndicesGetTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetTemplateService) Header(name string, value string) *IndicesGetTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetTemplateService) Headers(headers http.Header) *IndicesGetTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the index template.
func (s *IndicesGetTemplateService) Name(name ...string) *IndicesGetTemplateService {
	s.name = append(s.name, name...)
	return s
}

// FlatSettings is returns settings in flat format (default: false).
func (s *IndicesGetTemplateService) FlatSettings(flatSettings bool) *IndicesGetTemplateService {
	s.flatSettings = &flatSettings
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *IndicesGetTemplateService) Local(local bool) *IndicesGetTemplateService {
	s.local = &local
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.name) > 0 {
		path, err = uritemplates.Expand("/_template/{name}", map[string]string{
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetTemplateService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesGetTemplateService) Do(ctx context.Context) (map[string]*IndicesGetTemplateResponse, error) {
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
	var ret map[string]*IndicesGetTemplateResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesGetTemplateResponse is the response of IndicesGetTemplateService.Do.
type IndicesGetTemplateResponse struct {
	Order         int                    `json:"order,omitempty"`
	Version       int                    `json:"version,omitempty"`
	IndexPatterns []string               `json:"index_patterns,omitempty"`
	Settings      map[string]interface{} `json:"settings,omitempty"`
	Mappings      map[string]interface{} `json:"mappings,omitempty"`
	Aliases       map[string]interface{} `json:"aliases,omitempty"`
}
