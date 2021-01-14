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

// CatAliasesService shows information about currently configured aliases
// to indices including filter and routing infos.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cat-aliases.html
// for details.
type CatAliasesService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	local         *bool
	masterTimeout string
	aliases       []string
	columns       []string
	sort          []string // list of columns for sort order
}

// NewCatAliasesService creates a new CatAliasesService.
func NewCatAliasesService(client *Client) *CatAliasesService {
	return &CatAliasesService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *CatAliasesService) Pretty(pretty bool) *CatAliasesService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *CatAliasesService) Human(human bool) *CatAliasesService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *CatAliasesService) ErrorTrace(errorTrace bool) *CatAliasesService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *CatAliasesService) FilterPath(filterPath ...string) *CatAliasesService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *CatAliasesService) Header(name string, value string) *CatAliasesService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *CatAliasesService) Headers(headers http.Header) *CatAliasesService {
	s.headers = headers
	return s
}

// Alias specifies one or more aliases to which information should be returned.
func (s *CatAliasesService) Alias(alias ...string) *CatAliasesService {
	s.aliases = alias
	return s
}

// Local indicates to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *CatAliasesService) Local(local bool) *CatAliasesService {
	s.local = &local
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *CatAliasesService) MasterTimeout(masterTimeout string) *CatAliasesService {
	s.masterTimeout = masterTimeout
	return s
}

// Columns to return in the response.
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/aliases?help'
//
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatAliasesService) Columns(columns ...string) *CatAliasesService {
	s.columns = columns
	return s
}

// Sort is a list of fields to sort by.
func (s *CatAliasesService) Sort(fields ...string) *CatAliasesService {
	s.sort = fields
	return s
}

// buildURL builds the URL for the operation.
func (s *CatAliasesService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if len(s.aliases) > 0 {
		path, err = uritemplates.Expand("/_cat/aliases/{name}", map[string]string{
			"name": strings.Join(s.aliases, ","),
		})
	} else {
		path = "/_cat/aliases"
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
func (s *CatAliasesService) Do(ctx context.Context) (CatAliasesResponse, error) {
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
	var ret CatAliasesResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatAliasesResponse is the outcome of CatAliasesService.Do.
type CatAliasesResponse []CatAliasesResponseRow

// CatAliasesResponseRow is a single row in a CatAliasesResponse.
// Notice that not all of these fields might be filled; that depends
// on the number of columns chose in the request (see CatAliasesService.Columns).
type CatAliasesResponseRow struct {
	// Alias name.
	Alias string `json:"alias"`
	// Index the alias points to.
	Index string `json:"index"`
	// Filter, e.g. "*" or "-".
	Filter string `json:"filter"`
	// RoutingIndex specifies the index routing (or "-").
	RoutingIndex string `json:"routing.index"`
	// RoutingSearch specifies the search routing (or "-").
	RoutingSearch string `json:"routing.search"`
	// IsWriteIndex indicates whether the index can be written to (or "-").
	IsWriteIndex string `json:"is_write_index"`
}
