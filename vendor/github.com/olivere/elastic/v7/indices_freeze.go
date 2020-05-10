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

// IndicesFreezeService freezes an index.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/freeze-index-api.html
// and https://www.elastic.co/blog/creating-frozen-indices-with-the-elasticsearch-freeze-index-api
// for details.
type IndicesFreezeService struct {
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

// NewIndicesFreezeService creates a new IndicesFreezeService.
func NewIndicesFreezeService(client *Client) *IndicesFreezeService {
	return &IndicesFreezeService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesFreezeService) Pretty(pretty bool) *IndicesFreezeService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesFreezeService) Human(human bool) *IndicesFreezeService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesFreezeService) ErrorTrace(errorTrace bool) *IndicesFreezeService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesFreezeService) FilterPath(filterPath ...string) *IndicesFreezeService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesFreezeService) Header(name string, value string) *IndicesFreezeService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesFreezeService) Headers(headers http.Header) *IndicesFreezeService {
	s.headers = headers
	return s
}

// Index is the name of the index to freeze.
func (s *IndicesFreezeService) Index(index string) *IndicesFreezeService {
	s.index = index
	return s
}

// Timeout allows to specify an explicit timeout.
func (s *IndicesFreezeService) Timeout(timeout string) *IndicesFreezeService {
	s.timeout = timeout
	return s
}

// MasterTimeout allows to specify a timeout for connection to master.
func (s *IndicesFreezeService) MasterTimeout(masterTimeout string) *IndicesFreezeService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesFreezeService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesFreezeService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or when
// no indices have been specified).
func (s *IndicesFreezeService) AllowNoIndices(allowNoIndices bool) *IndicesFreezeService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards specifies whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesFreezeService) ExpandWildcards(expandWildcards string) *IndicesFreezeService {
	s.expandWildcards = expandWildcards
	return s
}

// WaitForActiveShards sets the number of active shards to wait for
// before the operation returns.
func (s *IndicesFreezeService) WaitForActiveShards(numShards string) *IndicesFreezeService {
	s.waitForActiveShards = numShards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesFreezeService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/_freeze", map[string]string{
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
func (s *IndicesFreezeService) Validate() error {
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
func (s *IndicesFreezeService) Do(ctx context.Context) (*IndicesFreezeResponse, error) {
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
	ret := new(IndicesFreezeResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesFreezeResponse is the outcome of freezing an index.
type IndicesFreezeResponse struct {
	Shards *ShardsInfo `json:"_shards"`
}
