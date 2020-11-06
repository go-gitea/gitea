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

// IndicesGetSettingsService allows to retrieve settings of one
// or more indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-get-settings.html
// for more details.
type IndicesGetSettingsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	name              []string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	flatSettings      *bool
	local             *bool
}

// NewIndicesGetSettingsService creates a new IndicesGetSettingsService.
func NewIndicesGetSettingsService(client *Client) *IndicesGetSettingsService {
	return &IndicesGetSettingsService{
		client: client,
		index:  make([]string, 0),
		name:   make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetSettingsService) Pretty(pretty bool) *IndicesGetSettingsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetSettingsService) Human(human bool) *IndicesGetSettingsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetSettingsService) ErrorTrace(errorTrace bool) *IndicesGetSettingsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetSettingsService) FilterPath(filterPath ...string) *IndicesGetSettingsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetSettingsService) Header(name string, value string) *IndicesGetSettingsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetSettingsService) Headers(headers http.Header) *IndicesGetSettingsService {
	s.headers = headers
	return s
}

// Index is a list of index names; use `_all` or empty string to perform
// the operation on all indices.
func (s *IndicesGetSettingsService) Index(indices ...string) *IndicesGetSettingsService {
	s.index = append(s.index, indices...)
	return s
}

// Name are the names of the settings that should be included.
func (s *IndicesGetSettingsService) Name(name ...string) *IndicesGetSettingsService {
	s.name = append(s.name, name...)
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should
// be ignored when unavailable (missing or closed).
func (s *IndicesGetSettingsService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesGetSettingsService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices.
// (This includes `_all` string or when no indices have been specified).
func (s *IndicesGetSettingsService) AllowNoIndices(allowNoIndices bool) *IndicesGetSettingsService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression
// to concrete indices that are open, closed or both.
// Options: open, closed, none, all. Default: open,closed.
func (s *IndicesGetSettingsService) ExpandWildcards(expandWildcards string) *IndicesGetSettingsService {
	s.expandWildcards = expandWildcards
	return s
}

// FlatSettings indicates whether to return settings in flat format (default: false).
func (s *IndicesGetSettingsService) FlatSettings(flatSettings bool) *IndicesGetSettingsService {
	s.flatSettings = &flatSettings
	return s
}

// Local indicates whether to return local information, do not retrieve
// the state from master node (default: false).
func (s *IndicesGetSettingsService) Local(local bool) *IndicesGetSettingsService {
	s.local = &local
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetSettingsService) buildURL() (string, url.Values, error) {
	var err error
	var path string
	var index []string

	if len(s.index) > 0 {
		index = s.index
	} else {
		index = []string{"_all"}
	}

	if len(s.name) > 0 {
		// Build URL
		path, err = uritemplates.Expand("/{index}/_settings/{name}", map[string]string{
			"index": strings.Join(index, ","),
			"name":  strings.Join(s.name, ","),
		})
	} else {
		// Build URL
		path, err = uritemplates.Expand("/{index}/_settings", map[string]string{
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
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetSettingsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesGetSettingsService) Do(ctx context.Context) (map[string]*IndicesGetSettingsResponse, error) {
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
	var ret map[string]*IndicesGetSettingsResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesGetSettingsResponse is the response of IndicesGetSettingsService.Do.
type IndicesGetSettingsResponse struct {
	Settings map[string]interface{} `json:"settings"`
}
