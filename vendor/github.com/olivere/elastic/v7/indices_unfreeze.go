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

// IndicesUnfreezeService unfreezes an index.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/unfreeze-index-api.html
// and https://www.elastic.co/blog/creating-frozen-indices-with-the-elasticsearch-freeze-index-api
// for details.
type IndicesUnfreezeService struct {
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

// NewIndicesUnfreezeService creates a new IndicesUnfreezeService.
func NewIndicesUnfreezeService(client *Client) *IndicesUnfreezeService {
	return &IndicesUnfreezeService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesUnfreezeService) Pretty(pretty bool) *IndicesUnfreezeService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesUnfreezeService) Human(human bool) *IndicesUnfreezeService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesUnfreezeService) ErrorTrace(errorTrace bool) *IndicesUnfreezeService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesUnfreezeService) FilterPath(filterPath ...string) *IndicesUnfreezeService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesUnfreezeService) Header(name string, value string) *IndicesUnfreezeService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesUnfreezeService) Headers(headers http.Header) *IndicesUnfreezeService {
	s.headers = headers
	return s
}

// Index is the name of the index to unfreeze.
func (s *IndicesUnfreezeService) Index(index string) *IndicesUnfreezeService {
	s.index = index
	return s
}

// Timeout allows to specify an explicit timeout.
func (s *IndicesUnfreezeService) Timeout(timeout string) *IndicesUnfreezeService {
	s.timeout = timeout
	return s
}

// MasterTimeout allows to specify a timeout for connection to master.
func (s *IndicesUnfreezeService) MasterTimeout(masterTimeout string) *IndicesUnfreezeService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesUnfreezeService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesUnfreezeService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or when
// no indices have been specified).
func (s *IndicesUnfreezeService) AllowNoIndices(allowNoIndices bool) *IndicesUnfreezeService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards specifies whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesUnfreezeService) ExpandWildcards(expandWildcards string) *IndicesUnfreezeService {
	s.expandWildcards = expandWildcards
	return s
}

// WaitForActiveShards sets the number of active shards to wait for
// before the operation returns.
func (s *IndicesUnfreezeService) WaitForActiveShards(numShards string) *IndicesUnfreezeService {
	s.waitForActiveShards = numShards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesUnfreezeService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/_unfreeze", map[string]string{
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
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
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
func (s *IndicesUnfreezeService) Validate() error {
	var invalid []string
	if s.index == "" {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the service.
func (s *IndicesUnfreezeService) Do(ctx context.Context) (*IndicesUnfreezeResponse, error) {
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
	ret := new(IndicesUnfreezeResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesUnfreezeResponse is the outcome of freezing an index.
type IndicesUnfreezeResponse struct {
	Shards *ShardsInfo `json:"_shards"`
}
