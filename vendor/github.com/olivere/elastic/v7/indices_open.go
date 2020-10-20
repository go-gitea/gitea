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

// IndicesOpenService opens an index.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-open-close.html
// for details.
type IndicesOpenService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index               string
	timeout             string
	masterTimeout       string
	ignoreUnavailable   *bool
	allowNoIndices      *bool
	expandWildcards     string
	waitForActiveShards string
}

// NewIndicesOpenService creates and initializes a new IndicesOpenService.
func NewIndicesOpenService(client *Client) *IndicesOpenService {
	return &IndicesOpenService{client: client}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesOpenService) Pretty(pretty bool) *IndicesOpenService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesOpenService) Human(human bool) *IndicesOpenService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesOpenService) ErrorTrace(errorTrace bool) *IndicesOpenService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesOpenService) FilterPath(filterPath ...string) *IndicesOpenService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesOpenService) Header(name string, value string) *IndicesOpenService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesOpenService) Headers(headers http.Header) *IndicesOpenService {
	s.headers = headers
	return s
}

// Index is the name of the index to open.
func (s *IndicesOpenService) Index(index string) *IndicesOpenService {
	s.index = index
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesOpenService) Timeout(timeout string) *IndicesOpenService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesOpenService) MasterTimeout(masterTimeout string) *IndicesOpenService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should
// be ignored when unavailable (missing or closed).
func (s *IndicesOpenService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesOpenService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices.
// (This includes `_all` string or when no indices have been specified).
func (s *IndicesOpenService) AllowNoIndices(allowNoIndices bool) *IndicesOpenService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesOpenService) ExpandWildcards(expandWildcards string) *IndicesOpenService {
	s.expandWildcards = expandWildcards
	return s
}

// WaitForActiveShards specifies the number of shards that must be allocated
// before the Open operation returns. Valid values are "all" or an integer
// between 0 and number_of_replicas+1 (default: 0)
func (s *IndicesOpenService) WaitForActiveShards(waitForActiveShards string) *IndicesOpenService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesOpenService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/_open", map[string]string{
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
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
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
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}

	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesOpenService) Validate() error {
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
func (s *IndicesOpenService) Do(ctx context.Context) (*IndicesOpenResponse, error) {
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
	ret := new(IndicesOpenResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesOpenResponse is the response of IndicesOpenService.Do.
type IndicesOpenResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
