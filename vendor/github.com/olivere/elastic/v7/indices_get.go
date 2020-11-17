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

// IndicesGetService retrieves information about one or more indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-get-index.html
// for more details.
type IndicesGetService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	feature           []string
	local             *bool
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	flatSettings      *bool
}

// NewIndicesGetService creates a new IndicesGetService.
func NewIndicesGetService(client *Client) *IndicesGetService {
	return &IndicesGetService{
		client:  client,
		index:   make([]string, 0),
		feature: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetService) Pretty(pretty bool) *IndicesGetService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetService) Human(human bool) *IndicesGetService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetService) ErrorTrace(errorTrace bool) *IndicesGetService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetService) FilterPath(filterPath ...string) *IndicesGetService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetService) Header(name string, value string) *IndicesGetService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetService) Headers(headers http.Header) *IndicesGetService {
	s.headers = headers
	return s
}

// Index is a list of index names.
func (s *IndicesGetService) Index(indices ...string) *IndicesGetService {
	s.index = append(s.index, indices...)
	return s
}

// Feature is a list of features.
func (s *IndicesGetService) Feature(features ...string) *IndicesGetService {
	s.feature = append(s.feature, features...)
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *IndicesGetService) Local(local bool) *IndicesGetService {
	s.local = &local
	return s
}

// IgnoreUnavailable indicates whether to ignore unavailable indexes (default: false).
func (s *IndicesGetService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesGetService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard expression
// resolves to no concrete indices (default: false).
func (s *IndicesGetService) AllowNoIndices(allowNoIndices bool) *IndicesGetService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether wildcard expressions should get
// expanded to open or closed indices (default: open).
func (s *IndicesGetService) ExpandWildcards(expandWildcards string) *IndicesGetService {
	s.expandWildcards = expandWildcards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetService) buildURL() (string, url.Values, error) {
	var err error
	var path string
	var index []string

	if len(s.index) > 0 {
		index = s.index
	} else {
		index = []string{"_all"}
	}

	if len(s.feature) > 0 {
		// Build URL
		path, err = uritemplates.Expand("/{index}/{feature}", map[string]string{
			"index":   strings.Join(index, ","),
			"feature": strings.Join(s.feature, ","),
		})
	} else {
		// Build URL
		path, err = uritemplates.Expand("/{index}", map[string]string{
			"index": strings.Join(index, ","),
		})
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
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetService) Validate() error {
	var invalid []string
	if len(s.index) == 0 {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesGetService) Do(ctx context.Context) (map[string]*IndicesGetResponse, error) {
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
	var ret map[string]*IndicesGetResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesGetResponse is part of the response of IndicesGetService.Do.
type IndicesGetResponse struct {
	Aliases  map[string]interface{} `json:"aliases"`
	Mappings map[string]interface{} `json:"mappings"`
	Settings map[string]interface{} `json:"settings"`
	Warmers  map[string]interface{} `json:"warmers"`
}
