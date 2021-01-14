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

// IndicesExistsService checks if an index or indices exist or not.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-exists.html
// for details.
type IndicesExistsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	local             *bool
}

// NewIndicesExistsService creates and initializes a new IndicesExistsService.
func NewIndicesExistsService(client *Client) *IndicesExistsService {
	return &IndicesExistsService{
		client: client,
		index:  make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesExistsService) Pretty(pretty bool) *IndicesExistsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesExistsService) Human(human bool) *IndicesExistsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesExistsService) ErrorTrace(errorTrace bool) *IndicesExistsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesExistsService) FilterPath(filterPath ...string) *IndicesExistsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesExistsService) Header(name string, value string) *IndicesExistsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesExistsService) Headers(headers http.Header) *IndicesExistsService {
	s.headers = headers
	return s
}

// Index is a list of one or more indices to check.
func (s *IndicesExistsService) Index(index []string) *IndicesExistsService {
	s.index = index
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or
// when no indices have been specified).
func (s *IndicesExistsService) AllowNoIndices(allowNoIndices bool) *IndicesExistsService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *IndicesExistsService) ExpandWildcards(expandWildcards string) *IndicesExistsService {
	s.expandWildcards = expandWildcards
	return s
}

// Local, when set, returns local information and does not retrieve the state
// from master node (default: false).
func (s *IndicesExistsService) Local(local bool) *IndicesExistsService {
	s.local = &local
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesExistsService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesExistsService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesExistsService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}", map[string]string{
		"index": strings.Join(s.index, ","),
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
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesExistsService) Validate() error {
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
func (s *IndicesExistsService) Do(ctx context.Context) (bool, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return false, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return false, err
	}

	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:       "HEAD",
		Path:         path,
		Params:       params,
		IgnoreErrors: []int{404},
		Headers:      s.headers,
	})
	if err != nil {
		return false, err
	}

	// Return operation response
	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("elastic: got HTTP code %d when it should have been either 200 or 404", res.StatusCode)
	}
}
