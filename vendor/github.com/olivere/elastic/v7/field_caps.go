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

// FieldCapsService allows retrieving the capabilities of fields among multiple indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-field-caps.html
// for details
type FieldCapsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	allowNoIndices    *bool
	expandWildcards   string
	fields            []string
	ignoreUnavailable *bool
	bodyJson          interface{}
	bodyString        string
}

// NewFieldCapsService creates a new FieldCapsService
func NewFieldCapsService(client *Client) *FieldCapsService {
	return &FieldCapsService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *FieldCapsService) Pretty(pretty bool) *FieldCapsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *FieldCapsService) Human(human bool) *FieldCapsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *FieldCapsService) ErrorTrace(errorTrace bool) *FieldCapsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *FieldCapsService) FilterPath(filterPath ...string) *FieldCapsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *FieldCapsService) Header(name string, value string) *FieldCapsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *FieldCapsService) Headers(headers http.Header) *FieldCapsService {
	s.headers = headers
	return s
}

// Index is a list of index names; use `_all` or empty string to perform
// the operation on all indices.
func (s *FieldCapsService) Index(index ...string) *FieldCapsService {
	s.index = append(s.index, index...)
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices.
// (This includes `_all` string or when no indices have been specified).
func (s *FieldCapsService) AllowNoIndices(allowNoIndices bool) *FieldCapsService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *FieldCapsService) ExpandWildcards(expandWildcards string) *FieldCapsService {
	s.expandWildcards = expandWildcards
	return s
}

// Fields is a list of fields for to get field capabilities.
func (s *FieldCapsService) Fields(fields ...string) *FieldCapsService {
	s.fields = append(s.fields, fields...)
	return s
}

// IgnoreUnavailable is documented as: Whether specified concrete indices should be ignored when unavailable (missing or closed).
func (s *FieldCapsService) IgnoreUnavailable(ignoreUnavailable bool) *FieldCapsService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// BodyJson is documented as: Field json objects containing the name and optionally a range to filter out indices result, that have results outside the defined bounds.
func (s *FieldCapsService) BodyJson(body interface{}) *FieldCapsService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: Field json objects containing the name and optionally a range to filter out indices result, that have results outside the defined bounds.
func (s *FieldCapsService) BodyString(body string) *FieldCapsService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *FieldCapsService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_field_caps", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_field_caps"
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
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if len(s.fields) > 0 {
		params.Set("fields", strings.Join(s.fields, ","))
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *FieldCapsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *FieldCapsService) Do(ctx context.Context) (*FieldCapsResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else {
		body = s.bodyString
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:       "POST",
		Path:         path,
		Params:       params,
		Body:         body,
		IgnoreErrors: []int{http.StatusNotFound},
		Headers:      s.headers,
	})
	if err != nil {
		return nil, err
	}

	// TODO(oe): Is 404 really a valid response here?
	if res.StatusCode == http.StatusNotFound {
		return &FieldCapsResponse{}, nil
	}

	// Return operation response
	ret := new(FieldCapsResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Request --

// FieldCapsRequest can be used to set up the body to be used in the
// Field Capabilities API.
type FieldCapsRequest struct {
	Fields []string `json:"fields"`
}

// -- Response --

// FieldCapsResponse contains field capabilities.
type FieldCapsResponse struct {
	Indices []string                 `json:"indices,omitempty"` // list of index names
	Fields  map[string]FieldCapsType `json:"fields,omitempty"`  // Name -> type -> caps
}

// FieldCapsType represents a mapping from type (e.g. keyword)
// to capabilities.
type FieldCapsType map[string]FieldCaps // type -> caps

// FieldCaps contains capabilities of an individual field.
type FieldCaps struct {
	Type                   string   `json:"type"`
	Searchable             bool     `json:"searchable"`
	Aggregatable           bool     `json:"aggregatable"`
	Indices                []string `json:"indices,omitempty"`
	NonSearchableIndices   []string `json:"non_searchable_indices,omitempty"`
	NonAggregatableIndices []string `json:"non_aggregatable_indices,omitempty"`
}
