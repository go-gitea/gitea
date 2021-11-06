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

// CatFielddataService Returns the amount of heap memory currently used by
// the field data cache on every data node in the cluster.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.12/cat-fielddata.html
// for details.
type CatFielddataService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	fields  []string // list of fields used to limit returned information
	bytes   string   // b, k, m, or g
	columns []string
	sort    []string // list of columns for sort order
}

// NewCatFielddataService creates a new NewCatFielddataService.
func NewCatFielddataService(client *Client) *CatFielddataService {
	return &CatFielddataService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *CatFielddataService) Pretty(pretty bool) *CatFielddataService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *CatFielddataService) Human(human bool) *CatFielddataService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *CatFielddataService) ErrorTrace(errorTrace bool) *CatFielddataService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *CatFielddataService) FilterPath(filterPath ...string) *CatFielddataService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *CatFielddataService) Header(name string, value string) *CatFielddataService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *CatFielddataService) Headers(headers http.Header) *CatFielddataService {
	s.headers = headers
	return s
}

// Fielddata specifies one or more node IDs to for information should be returned.
func (s *CatFielddataService) Field(fields ...string) *CatFielddataService {
	s.fields = fields
	return s
}

// Bytes represents the unit in which to display byte values.
// Valid values are: "b", "k", "m", or "g".
func (s *CatFielddataService) Bytes(bytes string) *CatFielddataService {
	s.bytes = bytes
	return s
}

// Columns to return in the response.
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/fielddata?help'
//
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatFielddataService) Columns(columns ...string) *CatFielddataService {
	s.columns = columns
	return s
}

// Sort is a list of fields to sort by.
func (s *CatFielddataService) Sort(fields ...string) *CatFielddataService {
	s.sort = fields
	return s
}

func (s *CatFielddataService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if len(s.fields) > 0 {
		path, err = uritemplates.Expand("/_cat/fielddata/{field}", map[string]string{
			"field": strings.Join(s.fields, ","),
		})
	} else {
		path = "/_cat/fielddata/"
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
	if s.bytes != "" {
		params.Set("bytes", s.bytes)
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
func (s *CatFielddataService) Do(ctx context.Context) (CatFielddataResponse, error) {
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
	var ret CatFielddataResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatFielddataResponse is the outcome of CatFielddataService.Do.
type CatFielddataResponse []CatFielddataResponseRow

// CatFielddataResponseRow is a single row in a CatFielddataResponse.
// Notice that not all of these fields might be filled; that depends on
// the number of columns chose in the request (see CatFielddataService.Columns).
type CatFielddataResponseRow struct {
	// Id represents the id of the fielddata.
	Id string `json:"id"`
	// Host represents the hostname of the fielddata.
	Host string `json:"host"`
	// IP represents the IP address of the fielddata.
	IP string `json:"ip"`
	// Node represents the Node name of the fielddata.
	Node string `json:"node"`
	// Field represents the name of the fielddata.
	Field string `json:"field"`
	// Size represents the size of the fielddata, e.g. "53.2gb".
	Size string `json:"size"`
}
