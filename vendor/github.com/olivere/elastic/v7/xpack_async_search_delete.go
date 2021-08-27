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

// XPackAsyncSearchDelete allows removing an asynchronous search result,
// previously being started with XPackAsyncSearchSubmit service.
//
// For more details, see the documentation at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.9/async-search.html
type XPackAsyncSearchDelete struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	// ID of asynchronous search as returned by XPackAsyncSearchSubmit.Do.
	id string
}

// NewXPackAsyncSearchDelete creates a new XPackAsyncSearchDelete.
func NewXPackAsyncSearchDelete(client *Client) *XPackAsyncSearchDelete {
	return &XPackAsyncSearchDelete{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackAsyncSearchDelete) Pretty(pretty bool) *XPackAsyncSearchDelete {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackAsyncSearchDelete) Human(human bool) *XPackAsyncSearchDelete {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackAsyncSearchDelete) ErrorTrace(errorTrace bool) *XPackAsyncSearchDelete {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackAsyncSearchDelete) FilterPath(filterPath ...string) *XPackAsyncSearchDelete {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackAsyncSearchDelete) Header(name string, value string) *XPackAsyncSearchDelete {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackAsyncSearchDelete) Headers(headers http.Header) *XPackAsyncSearchDelete {
	s.headers = headers
	return s
}

// ID of the asynchronous search.
func (s *XPackAsyncSearchDelete) ID(id string) *XPackAsyncSearchDelete {
	s.id = id
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackAsyncSearchDelete) buildURL() (string, url.Values, error) {
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackAsyncSearchDelete) Validate() error {
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
func (s *XPackAsyncSearchDelete) Do(ctx context.Context) (*XPackAsyncSearchDeleteResponse, error) {
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
		Method:  "DELETE",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackAsyncSearchDeleteResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackAsyncSearchDeleteResponse is the outcome of calling XPackAsyncSearchDelete.Do.
type XPackAsyncSearchDeleteResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
