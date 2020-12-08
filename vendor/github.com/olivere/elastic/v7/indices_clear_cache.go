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

// IndicesClearCacheService allows to clear either all caches or specific cached associated
// with one or more indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.6/indices-clearcache.html
// for details.
type IndicesClearCacheService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	fieldData         *bool
	fields            string
	query             *bool
	request           *bool
}

// NewIndicesClearCacheService initializes a new instance of
// IndicesClearCacheService.
func NewIndicesClearCacheService(client *Client) *IndicesClearCacheService {
	return &IndicesClearCacheService{client: client}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesClearCacheService) Pretty(pretty bool) *IndicesClearCacheService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesClearCacheService) Human(human bool) *IndicesClearCacheService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesClearCacheService) ErrorTrace(errorTrace bool) *IndicesClearCacheService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesClearCacheService) FilterPath(filterPath ...string) *IndicesClearCacheService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesClearCacheService) Header(name string, value string) *IndicesClearCacheService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesClearCacheService) Headers(headers http.Header) *IndicesClearCacheService {
	s.headers = headers
	return s
}

// Index is the comma-separated list or wildcard expression of index names used to clear cache.
func (s *IndicesClearCacheService) Index(indices ...string) *IndicesClearCacheService {
	s.index = append(s.index, indices...)
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesClearCacheService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesClearCacheService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all` string or when no indices
// have been specified).
func (s *IndicesClearCacheService) AllowNoIndices(allowNoIndices bool) *IndicesClearCacheService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *IndicesClearCacheService) ExpandWildcards(expandWildcards string) *IndicesClearCacheService {
	s.expandWildcards = expandWildcards
	return s
}

// FieldData indicates whether to clear the fields cache.
// Use the fields parameter to clear the cache of specific fields only.
func (s *IndicesClearCacheService) FieldData(fieldData bool) *IndicesClearCacheService {
	s.fieldData = &fieldData
	return s
}

// Fields indicates comma-separated list of field names used to limit the fielddata parameter.
// Defaults to all fields.
func (s *IndicesClearCacheService) Fields(fields string) *IndicesClearCacheService {
	s.fields = fields
	return s
}

// Query indicates whether to clear only query cache.
func (s *IndicesClearCacheService) Query(queryCache bool) *IndicesClearCacheService {
	s.query = &queryCache
	return s
}

// Request indicates whether to clear only request cache.
func (s *IndicesClearCacheService) Request(requestCache bool) *IndicesClearCacheService {
	s.request = &requestCache
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesClearCacheService) buildURL() (string, url.Values, error) {
	// Build URL
	var path string
	var err error

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_cache/clear", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_cache/clear"
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
	if v := s.allowNoIndices; v != nil {
		params.Set("allow_no_indices", fmt.Sprint(*v))
	}
	if v := s.expandWildcards; v != "" {
		params.Set("expand_wildcards", v)
	}
	if v := s.ignoreUnavailable; v != nil {
		params.Set("ignore_unavailable", fmt.Sprint(*v))
	}
	if len(s.index) > 0 {
		params.Set("index", fmt.Sprintf("%v", s.index))
	}
	if v := s.ignoreUnavailable; v != nil {
		params.Set("fielddata", fmt.Sprint(*v))
	}
	if len(s.fields) > 0 {
		params.Set("fields", fmt.Sprintf("%v", s.fields))
	}
	if v := s.query; v != nil {
		params.Set("query", fmt.Sprint(*v))
	}
	if s.request != nil {
		params.Set("request", fmt.Sprintf("%v", *s.request))
	}

	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesClearCacheService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesClearCacheService) Do(ctx context.Context) (*IndicesClearCacheResponse, error) {
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
	ret := new(IndicesClearCacheResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesClearCacheResponse is the response of IndicesClearCacheService.Do.
type IndicesClearCacheResponse struct {
	Shards *ShardsInfo `json:"_shards"`
}
