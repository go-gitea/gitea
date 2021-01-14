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

// RefreshService explicitly refreshes one or more indices.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-refresh.html.
type RefreshService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index []string
}

// NewRefreshService creates a new instance of RefreshService.
func NewRefreshService(client *Client) *RefreshService {
	builder := &RefreshService{
		client: client,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *RefreshService) Pretty(pretty bool) *RefreshService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *RefreshService) Human(human bool) *RefreshService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *RefreshService) ErrorTrace(errorTrace bool) *RefreshService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *RefreshService) FilterPath(filterPath ...string) *RefreshService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *RefreshService) Header(name string, value string) *RefreshService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *RefreshService) Headers(headers http.Header) *RefreshService {
	s.headers = headers
	return s
}

// Index specifies the indices to refresh.
func (s *RefreshService) Index(index ...string) *RefreshService {
	s.index = append(s.index, index...)
	return s
}

// buildURL builds the URL for the operation.
func (s *RefreshService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_refresh", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_refresh"
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
	return path, params, nil
}

// Do executes the request.
func (s *RefreshService) Do(ctx context.Context) (*RefreshResult, error) {
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return result
	ret := new(RefreshResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a refresh request.

// RefreshResult is the outcome of RefreshService.Do.
type RefreshResult struct {
	Shards *ShardsInfo `json:"_shards,omitempty"`
}
