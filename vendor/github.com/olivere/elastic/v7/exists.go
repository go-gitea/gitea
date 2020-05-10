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

// ExistsService checks for the existence of a document using HEAD.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-get.html
// for details.
type ExistsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id         string
	index      string
	typ        string
	preference string
	realtime   *bool
	refresh    string
	routing    string
	parent     string
}

// NewExistsService creates a new ExistsService.
func NewExistsService(client *Client) *ExistsService {
	return &ExistsService{
		client: client,
		typ:    "_doc",
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ExistsService) Pretty(pretty bool) *ExistsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ExistsService) Human(human bool) *ExistsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ExistsService) ErrorTrace(errorTrace bool) *ExistsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ExistsService) FilterPath(filterPath ...string) *ExistsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ExistsService) Header(name string, value string) *ExistsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ExistsService) Headers(headers http.Header) *ExistsService {
	s.headers = headers
	return s
}

// Id is the document ID.
func (s *ExistsService) Id(id string) *ExistsService {
	s.id = id
	return s
}

// Index is the name of the index.
func (s *ExistsService) Index(index string) *ExistsService {
	s.index = index
	return s
}

// Type is the type of the document (use `_all` to fetch the first document
// matching the ID across all types).
func (s *ExistsService) Type(typ string) *ExistsService {
	s.typ = typ
	return s
}

// Preference specifies the node or shard the operation should be performed on (default: random).
func (s *ExistsService) Preference(preference string) *ExistsService {
	s.preference = preference
	return s
}

// Realtime specifies whether to perform the operation in realtime or search mode.
func (s *ExistsService) Realtime(realtime bool) *ExistsService {
	s.realtime = &realtime
	return s
}

// Refresh the shard containing the document before performing the operation.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *ExistsService) Refresh(refresh string) *ExistsService {
	s.refresh = refresh
	return s
}

// Routing is a specific routing value.
func (s *ExistsService) Routing(routing string) *ExistsService {
	s.routing = routing
	return s
}

// Parent is the ID of the parent document.
func (s *ExistsService) Parent(parent string) *ExistsService {
	s.parent = parent
	return s
}

// buildURL builds the URL for the operation.
func (s *ExistsService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/{type}/{id}", map[string]string{
		"id":    s.id,
		"index": s.index,
		"type":  s.typ,
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
	if s.realtime != nil {
		params.Set("realtime", fmt.Sprint(*s.realtime))
	}
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if s.parent != "" {
		params.Set("parent", s.parent)
	}
	if s.preference != "" {
		params.Set("preference", s.preference)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ExistsService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.index == "" {
		invalid = append(invalid, "Index")
	}
	if s.typ == "" {
		invalid = append(invalid, "Type")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *ExistsService) Do(ctx context.Context) (bool, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return false, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return false, err
	}

	// Get HTTP response
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
