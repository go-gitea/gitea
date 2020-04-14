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

// IndicesGetFieldMappingService retrieves the mapping definitions for the fields in an index
//  or index/type.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-get-field-mapping.html
// for details.
type IndicesGetFieldMappingService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	typ               []string
	field             []string
	local             *bool
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
}

// NewGetFieldMappingService is an alias for NewIndicesGetFieldMappingService.
// Use NewIndicesGetFieldMappingService.
func NewGetFieldMappingService(client *Client) *IndicesGetFieldMappingService {
	return NewIndicesGetFieldMappingService(client)
}

// NewIndicesGetFieldMappingService creates a new IndicesGetFieldMappingService.
func NewIndicesGetFieldMappingService(client *Client) *IndicesGetFieldMappingService {
	return &IndicesGetFieldMappingService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetFieldMappingService) Pretty(pretty bool) *IndicesGetFieldMappingService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetFieldMappingService) Human(human bool) *IndicesGetFieldMappingService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetFieldMappingService) ErrorTrace(errorTrace bool) *IndicesGetFieldMappingService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetFieldMappingService) FilterPath(filterPath ...string) *IndicesGetFieldMappingService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetFieldMappingService) Header(name string, value string) *IndicesGetFieldMappingService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetFieldMappingService) Headers(headers http.Header) *IndicesGetFieldMappingService {
	s.headers = headers
	return s
}

// Index is a list of index names.
func (s *IndicesGetFieldMappingService) Index(indices ...string) *IndicesGetFieldMappingService {
	s.index = append(s.index, indices...)
	return s
}

// Type is a list of document types.
func (s *IndicesGetFieldMappingService) Type(types ...string) *IndicesGetFieldMappingService {
	s.typ = append(s.typ, types...)
	return s
}

// Field is a list of fields.
func (s *IndicesGetFieldMappingService) Field(fields ...string) *IndicesGetFieldMappingService {
	s.field = append(s.field, fields...)
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices.
// This includes `_all` string or when no indices have been specified.
func (s *IndicesGetFieldMappingService) AllowNoIndices(allowNoIndices bool) *IndicesGetFieldMappingService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesGetFieldMappingService) ExpandWildcards(expandWildcards string) *IndicesGetFieldMappingService {
	s.expandWildcards = expandWildcards
	return s
}

// Local indicates whether to return local information, do not retrieve
// the state from master node (default: false).
func (s *IndicesGetFieldMappingService) Local(local bool) *IndicesGetFieldMappingService {
	s.local = &local
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesGetFieldMappingService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesGetFieldMappingService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetFieldMappingService) buildURL() (string, url.Values, error) {
	var index, typ, field []string

	if len(s.index) > 0 {
		index = s.index
	} else {
		index = []string{"_all"}
	}

	if len(s.typ) > 0 {
		typ = s.typ
	} else {
		typ = []string{"_all"}
	}

	if len(s.field) > 0 {
		field = s.field
	} else {
		field = []string{"*"}
	}

	// Build URL
	path, err := uritemplates.Expand("/{index}/_mapping/{type}/field/{field}", map[string]string{
		"index": strings.Join(index, ","),
		"type":  strings.Join(typ, ","),
		"field": strings.Join(field, ","),
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
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetFieldMappingService) Validate() error {
	return nil
}

// Do executes the operation. It returns mapping definitions for an index
// or index/type.
func (s *IndicesGetFieldMappingService) Do(ctx context.Context) (map[string]interface{}, error) {
	var ret map[string]interface{}

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
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}
