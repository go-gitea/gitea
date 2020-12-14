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

// CatCountService provides quick access to the document count of the entire cluster,
// or individual indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cat-count.html
// for details.
type CatCountService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index         []string
	local         *bool
	masterTimeout string
	columns       []string
	sort          []string // list of columns for sort order
}

// NewCatCountService creates a new CatCountService.
func NewCatCountService(client *Client) *CatCountService {
	return &CatCountService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *CatCountService) Pretty(pretty bool) *CatCountService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *CatCountService) Human(human bool) *CatCountService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *CatCountService) ErrorTrace(errorTrace bool) *CatCountService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *CatCountService) FilterPath(filterPath ...string) *CatCountService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *CatCountService) Header(name string, value string) *CatCountService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *CatCountService) Headers(headers http.Header) *CatCountService {
	s.headers = headers
	return s
}

// Index specifies zero or more indices for which to return counts
// (by default counts for all indices are returned).
func (s *CatCountService) Index(index ...string) *CatCountService {
	s.index = index
	return s
}

// Local indicates to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *CatCountService) Local(local bool) *CatCountService {
	s.local = &local
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *CatCountService) MasterTimeout(masterTimeout string) *CatCountService {
	s.masterTimeout = masterTimeout
	return s
}

// Columns to return in the response.
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/count?help'
//
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatCountService) Columns(columns ...string) *CatCountService {
	s.columns = columns
	return s
}

// Sort is a list of fields to sort by.
func (s *CatCountService) Sort(fields ...string) *CatCountService {
	s.sort = fields
	return s
}

// buildURL builds the URL for the operation.
func (s *CatCountService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/_cat/count/{index}", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_cat/count"
	}
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{
		"format": []string{"json"}, // always returns as JSON
	}
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
	if v := s.local; v != nil {
		params.Set("local", fmt.Sprint(*v))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if len(s.sort) > 0 {
		params.Set("s", strings.Join(s.sort, ","))
	}
	if len(s.columns) > 0 {
		params.Set("h", strings.Join(s.columns, ","))
	}
	return path, params, nil
}

// Do executes the operation.
func (s *CatCountService) Do(ctx context.Context) (CatCountResponse, error) {
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
	var ret CatCountResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatCountResponse is the outcome of CatCountService.Do.
type CatCountResponse []CatCountResponseRow

// CatCountResponseRow specifies the data returned for one index
// of a CatCountResponse. Notice that not all of these fields might
// be filled; that depends on the number of columns chose in the
// request (see CatCountService.Columns).
type CatCountResponseRow struct {
	Epoch     int64  `json:"epoch,string"` // e.g. 1527077996
	Timestamp string `json:"timestamp"`    // e.g. "12:19:56"
	Count     int    `json:"count,string"` // number of documents
}
