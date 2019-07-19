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

	"gopkg.in/olivere/elastic.v5/uritemplates"
)

// FieldCapsService allows retrieving the capabilities of fields among multiple indices.
//
// See http://www.elastic.co/guide/en/elasticsearch/reference/5.x/search-field-caps.html
// for details
type FieldCapsService struct {
	client            *Client
	pretty            bool
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

// Pretty indicates that the JSON response be indented and human readable.
func (s *FieldCapsService) Pretty(pretty bool) *FieldCapsService {
	s.pretty = pretty
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
	if s.pretty {
		params.Set("pretty", "true")
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
	res, err := s.client.PerformRequest(ctx, "POST", path, params, body, http.StatusNotFound)
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
	Fields map[string]FieldCaps `json:"fields,omitempty"`
}

// FieldCaps contains capabilities of an individual field.
type FieldCaps struct {
	Type                   string   `json:"type"`
	Searchable             bool     `json:"searchable"`
	Aggregatable           bool     `json:"aggregatable"`
	Indices                []string `json:"indices,omitempty"`
	NonSearchableIndices   []string `json:"non_searchable_indices,omitempty"`
	NonAggregatableIndices []string `json:"non_aggregatable_indices,omitempty"`
}
