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

// IndicesPutSettingsService changes specific index level settings in
// real time.
//
// See the documentation at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-update-settings.html.
type IndicesPutSettingsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	allowNoIndices    *bool
	expandWildcards   string
	flatSettings      *bool
	ignoreUnavailable *bool
	masterTimeout     string
	bodyJson          interface{}
	bodyString        string
}

// NewIndicesPutSettingsService creates a new IndicesPutSettingsService.
func NewIndicesPutSettingsService(client *Client) *IndicesPutSettingsService {
	return &IndicesPutSettingsService{
		client: client,
		index:  make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesPutSettingsService) Pretty(pretty bool) *IndicesPutSettingsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesPutSettingsService) Human(human bool) *IndicesPutSettingsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesPutSettingsService) ErrorTrace(errorTrace bool) *IndicesPutSettingsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesPutSettingsService) FilterPath(filterPath ...string) *IndicesPutSettingsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesPutSettingsService) Header(name string, value string) *IndicesPutSettingsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesPutSettingsService) Headers(headers http.Header) *IndicesPutSettingsService {
	s.headers = headers
	return s
}

// Index is a list of index names the mapping should be added to
// (supports wildcards); use `_all` or omit to add the mapping on all indices.
func (s *IndicesPutSettingsService) Index(indices ...string) *IndicesPutSettingsService {
	s.index = append(s.index, indices...)
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all`
// string or when no indices have been specified).
func (s *IndicesPutSettingsService) AllowNoIndices(allowNoIndices bool) *IndicesPutSettingsService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards specifies whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *IndicesPutSettingsService) ExpandWildcards(expandWildcards string) *IndicesPutSettingsService {
	s.expandWildcards = expandWildcards
	return s
}

// FlatSettings indicates whether to return settings in flat format (default: false).
func (s *IndicesPutSettingsService) FlatSettings(flatSettings bool) *IndicesPutSettingsService {
	s.flatSettings = &flatSettings
	return s
}

// IgnoreUnavailable specifies whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesPutSettingsService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesPutSettingsService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// MasterTimeout is the timeout for connection to master.
func (s *IndicesPutSettingsService) MasterTimeout(masterTimeout string) *IndicesPutSettingsService {
	s.masterTimeout = masterTimeout
	return s
}

// BodyJson is documented as: The index settings to be updated.
func (s *IndicesPutSettingsService) BodyJson(body interface{}) *IndicesPutSettingsService {
	s.bodyJson = body
	return s
}

// BodyString is documented as: The index settings to be updated.
func (s *IndicesPutSettingsService) BodyString(body string) *IndicesPutSettingsService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesPutSettingsService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_settings", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_settings"
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
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesPutSettingsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesPutSettingsService) Do(ctx context.Context) (*IndicesPutSettingsResponse, error) {
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
	ret := new(IndicesPutSettingsResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesPutSettingsResponse is the response of IndicesPutSettingsService.Do.
type IndicesPutSettingsResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
