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
)

// XPackAsyncSearchGet allows retrieving an asynchronous search result,
// previously being started with XPackAsyncSearchSubmit service.
//
// For more details, see the documentation at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/async-search.html
type XPackAsyncSearchGet struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	// ID of asynchronous search as returned by XPackAsyncSearchSubmit.Do.
	id string
	// waitForCompletionTimeout is the duration the call should wait for a result
	// before timing out. The default is 1 second.
	waitForCompletionTimeout string
	// keepAlive asks Elasticsearch to keep the ID and its results even
	// after the search has been completed.
	keepAlive string
}

// NewXPackAsyncSearchGet creates a new XPackAsyncSearchGet.
func NewXPackAsyncSearchGet(client *Client) *XPackAsyncSearchGet {
	return &XPackAsyncSearchGet{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackAsyncSearchGet) Pretty(pretty bool) *XPackAsyncSearchGet {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackAsyncSearchGet) Human(human bool) *XPackAsyncSearchGet {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackAsyncSearchGet) ErrorTrace(errorTrace bool) *XPackAsyncSearchGet {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackAsyncSearchGet) FilterPath(filterPath ...string) *XPackAsyncSearchGet {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackAsyncSearchGet) Header(name string, value string) *XPackAsyncSearchGet {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackAsyncSearchGet) Headers(headers http.Header) *XPackAsyncSearchGet {
	s.headers = headers
	return s
}

// ID of the asynchronous search.
func (s *XPackAsyncSearchGet) ID(id string) *XPackAsyncSearchGet {
	s.id = id
	return s
}

// WaitForCompletionTimeout specifies the time the service waits for retrieving
// a complete result. If the timeout expires, you'll get the current results which
// might not be complete.
func (s *XPackAsyncSearchGet) WaitForCompletionTimeout(waitForCompletionTimeout string) *XPackAsyncSearchGet {
	s.waitForCompletionTimeout = waitForCompletionTimeout
	return s
}

// KeepAlive is the time the search results are kept by Elasticsearch before
// being garbage collected.
func (s *XPackAsyncSearchGet) KeepAlive(keepAlive string) *XPackAsyncSearchGet {
	s.keepAlive = keepAlive
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackAsyncSearchGet) buildURL() (string, url.Values, error) {
	path := fmt.Sprintf("/_async_search/%s", url.PathEscape(s.id))

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
	if s.waitForCompletionTimeout != "" {
		params.Set("wait_for_completion_timeout", s.waitForCompletionTimeout)
	}
	if s.keepAlive != "" {
		params.Set("keep_alive", s.keepAlive)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackAsyncSearchGet) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "ID")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackAsyncSearchGet) Do(ctx context.Context) (*XPackAsyncSearchResult, error) {
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
	ret := new(XPackAsyncSearchResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		ret.Header = res.Header
		return nil, err
	}
	ret.Header = res.Header
	return ret, nil
}
