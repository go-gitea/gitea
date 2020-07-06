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

// IndicesCloseService closes an index.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-open-close.html
// for details.
type IndicesCloseService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             string
	timeout           string
	masterTimeout     string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
}

// NewIndicesCloseService creates and initializes a new IndicesCloseService.
func NewIndicesCloseService(client *Client) *IndicesCloseService {
	return &IndicesCloseService{client: client}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesCloseService) Pretty(pretty bool) *IndicesCloseService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesCloseService) Human(human bool) *IndicesCloseService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesCloseService) ErrorTrace(errorTrace bool) *IndicesCloseService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesCloseService) FilterPath(filterPath ...string) *IndicesCloseService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesCloseService) Header(name string, value string) *IndicesCloseService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesCloseService) Headers(headers http.Header) *IndicesCloseService {
	s.headers = headers
	return s
}

// Index is the name of the index to close.
func (s *IndicesCloseService) Index(index string) *IndicesCloseService {
	s.index = index
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesCloseService) Timeout(timeout string) *IndicesCloseService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesCloseService) MasterTimeout(masterTimeout string) *IndicesCloseService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesCloseService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesCloseService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all` string or when no indices have been specified).
func (s *IndicesCloseService) AllowNoIndices(allowNoIndices bool) *IndicesCloseService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *IndicesCloseService) ExpandWildcards(expandWildcards string) *IndicesCloseService {
	s.expandWildcards = expandWildcards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesCloseService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/_close", map[string]string{
		"index": s.index,
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
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}

	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesCloseService) Validate() error {
	var invalid []string
	if s.index == "" {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesCloseService) Do(ctx context.Context) (*IndicesCloseResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(IndicesCloseResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesCloseResponse is the response of IndicesCloseService.Do.
type IndicesCloseResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
