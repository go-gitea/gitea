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

// Flush allows to flush one or more indices. The flush process of an index
// basically frees memory from the index by flushing data to the index
// storage and clearing the internal transaction log.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-flush.html
// for details.
type IndicesFlushService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	force             *bool
	waitIfOngoing     *bool
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
}

// NewIndicesFlushService creates a new IndicesFlushService.
func NewIndicesFlushService(client *Client) *IndicesFlushService {
	return &IndicesFlushService{
		client: client,
		index:  make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesFlushService) Pretty(pretty bool) *IndicesFlushService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesFlushService) Human(human bool) *IndicesFlushService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesFlushService) ErrorTrace(errorTrace bool) *IndicesFlushService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesFlushService) FilterPath(filterPath ...string) *IndicesFlushService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesFlushService) Header(name string, value string) *IndicesFlushService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesFlushService) Headers(headers http.Header) *IndicesFlushService {
	s.headers = headers
	return s
}

// Index is a list of index names; use `_all` or empty string for all indices.
func (s *IndicesFlushService) Index(indices ...string) *IndicesFlushService {
	s.index = append(s.index, indices...)
	return s
}

// Force indicates whether a flush should be forced even if it is not
// necessarily needed ie. if no changes will be committed to the index.
// This is useful if transaction log IDs should be incremented even if
// no uncommitted changes are present. (This setting can be considered as internal).
func (s *IndicesFlushService) Force(force bool) *IndicesFlushService {
	s.force = &force
	return s
}

// WaitIfOngoing, if set to true, indicates that the flush operation will
// block until the flush can be executed if another flush operation is
// already executing. The default is false and will cause an exception
// to be thrown on the shard level if another flush operation is already running..
func (s *IndicesFlushService) WaitIfOngoing(waitIfOngoing bool) *IndicesFlushService {
	s.waitIfOngoing = &waitIfOngoing
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesFlushService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesFlushService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or when
// no indices have been specified).
func (s *IndicesFlushService) AllowNoIndices(allowNoIndices bool) *IndicesFlushService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards specifies whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesFlushService) ExpandWildcards(expandWildcards string) *IndicesFlushService {
	s.expandWildcards = expandWildcards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesFlushService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_flush", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_flush"
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
	if s.force != nil {
		params.Set("force", fmt.Sprintf("%v", *s.force))
	}
	if s.waitIfOngoing != nil {
		params.Set("wait_if_ongoing", fmt.Sprintf("%v", *s.waitIfOngoing))
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesFlushService) Validate() error {
	return nil
}

// Do executes the service.
func (s *IndicesFlushService) Do(ctx context.Context) (*IndicesFlushResponse, error) {
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
	ret := new(IndicesFlushResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a flush request.

type IndicesFlushResponse struct {
	Shards *ShardsInfo `json:"_shards"`
}
