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

// IndicesPutMappingService allows to register specific mapping definition
// for a specific type.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-put-mapping.html
// for details.
type IndicesPutMappingService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	masterTimeout     string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	updateAllTypes    *bool
	timeout           string
	bodyJson          map[string]interface{}
	bodyString        string
}

// NewPutMappingService is an alias for NewIndicesPutMappingService.
// Use NewIndicesPutMappingService.
func NewPutMappingService(client *Client) *IndicesPutMappingService {
	return NewIndicesPutMappingService(client)
}

// NewIndicesPutMappingService creates a new IndicesPutMappingService.
func NewIndicesPutMappingService(client *Client) *IndicesPutMappingService {
	return &IndicesPutMappingService{
		client: client,
		index:  make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesPutMappingService) Pretty(pretty bool) *IndicesPutMappingService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesPutMappingService) Human(human bool) *IndicesPutMappingService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesPutMappingService) ErrorTrace(errorTrace bool) *IndicesPutMappingService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesPutMappingService) FilterPath(filterPath ...string) *IndicesPutMappingService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesPutMappingService) Header(name string, value string) *IndicesPutMappingService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesPutMappingService) Headers(headers http.Header) *IndicesPutMappingService {
	s.headers = headers
	return s
}

// Index is a list of index names the mapping should be added to
// (supports wildcards); use `_all` or omit to add the mapping on all indices.
func (s *IndicesPutMappingService) Index(indices ...string) *IndicesPutMappingService {
	s.index = append(s.index, indices...)
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesPutMappingService) Timeout(timeout string) *IndicesPutMappingService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesPutMappingService) MasterTimeout(masterTimeout string) *IndicesPutMappingService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesPutMappingService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesPutMappingService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices.
// This includes `_all` string or when no indices have been specified.
func (s *IndicesPutMappingService) AllowNoIndices(allowNoIndices bool) *IndicesPutMappingService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *IndicesPutMappingService) ExpandWildcards(expandWildcards string) *IndicesPutMappingService {
	s.expandWildcards = expandWildcards
	return s
}

// UpdateAllTypes, if true, indicates that all fields that span multiple indices
// should be updated (default: false).
func (s *IndicesPutMappingService) UpdateAllTypes(updateAllTypes bool) *IndicesPutMappingService {
	s.updateAllTypes = &updateAllTypes
	return s
}

// BodyJson contains the mapping definition.
func (s *IndicesPutMappingService) BodyJson(mapping map[string]interface{}) *IndicesPutMappingService {
	s.bodyJson = mapping
	return s
}

// BodyString is the mapping definition serialized as a string.
func (s *IndicesPutMappingService) BodyString(mapping string) *IndicesPutMappingService {
	s.bodyString = mapping
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesPutMappingService) buildURL() (string, url.Values, error) {
	path, err := uritemplates.Expand("/{index}/_mapping", map[string]string{
		"index": strings.Join(s.index, ","),
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
	if s.updateAllTypes != nil {
		params.Set("update_all_types", fmt.Sprintf("%v", *s.updateAllTypes))
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesPutMappingService) Validate() error {
	var invalid []string
	if len(s.index) == 0 {
		invalid = append(invalid, "Index")
	}
	if s.bodyString == "" && s.bodyJson == nil {
		invalid = append(invalid, "BodyJson")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesPutMappingService) Do(ctx context.Context) (*PutMappingResponse, error) {
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
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(PutMappingResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// PutMappingResponse is the response of IndicesPutMappingService.Do.
type PutMappingResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
