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

// ValidateService allows a user to validate a potentially
// expensive query without executing it.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-validate.html.
type ValidateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	typ               []string
	q                 string
	explain           *bool
	rewrite           *bool
	allShards         *bool
	lenient           *bool
	analyzer          string
	df                string
	analyzeWildcard   *bool
	defaultOperator   string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
	bodyJson          interface{}
	bodyString        string
}

// NewValidateService creates a new ValidateService.
func NewValidateService(client *Client) *ValidateService {
	return &ValidateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ValidateService) Pretty(pretty bool) *ValidateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ValidateService) Human(human bool) *ValidateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ValidateService) ErrorTrace(errorTrace bool) *ValidateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ValidateService) FilterPath(filterPath ...string) *ValidateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ValidateService) Header(name string, value string) *ValidateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ValidateService) Headers(headers http.Header) *ValidateService {
	s.headers = headers
	return s
}

// Index sets the names of the indices to use for search.
func (s *ValidateService) Index(index ...string) *ValidateService {
	s.index = append(s.index, index...)
	return s
}

// Type adds search restrictions for a list of types.
//
// Deprecated: Types are in the process of being removed. Instead of using a type, prefer to
// filter on a field on the document.
func (s *ValidateService) Type(typ ...string) *ValidateService {
	s.typ = append(s.typ, typ...)
	return s
}

// Lenient specifies whether format-based query failures
// (such as providing text to a numeric field) should be ignored.
func (s *ValidateService) Lenient(lenient bool) *ValidateService {
	s.lenient = &lenient
	return s
}

// Query in the Lucene query string syntax.
func (s *ValidateService) Q(q string) *ValidateService {
	s.q = q
	return s
}

// An explain parameter can be specified to get more detailed information about why a query failed.
func (s *ValidateService) Explain(explain *bool) *ValidateService {
	s.explain = explain
	return s
}

// Provide a more detailed explanation showing the actual Lucene query that will be executed.
func (s *ValidateService) Rewrite(rewrite *bool) *ValidateService {
	s.rewrite = rewrite
	return s
}

// Execute validation on all shards instead of one random shard per index.
func (s *ValidateService) AllShards(allShards *bool) *ValidateService {
	s.allShards = allShards
	return s
}

// AnalyzeWildcard specifies whether wildcards and prefix queries
// in the query string query should be analyzed (default: false).
func (s *ValidateService) AnalyzeWildcard(analyzeWildcard bool) *ValidateService {
	s.analyzeWildcard = &analyzeWildcard
	return s
}

// Analyzer is the analyzer for the query string query.
func (s *ValidateService) Analyzer(analyzer string) *ValidateService {
	s.analyzer = analyzer
	return s
}

// Df is the default field for query string query (default: _all).
func (s *ValidateService) Df(df string) *ValidateService {
	s.df = df
	return s
}

// DefaultOperator is the default operator for query string query (AND or OR).
func (s *ValidateService) DefaultOperator(defaultOperator string) *ValidateService {
	s.defaultOperator = defaultOperator
	return s
}

// Query sets a query definition using the Query DSL.
func (s *ValidateService) Query(query Query) *ValidateService {
	src, err := query.Source()
	if err != nil {
		// Do nothing in case of an error
		return s
	}
	body := make(map[string]interface{})
	body["query"] = src
	s.bodyJson = body
	return s
}

// IgnoreUnavailable indicates whether the specified concrete indices
// should be ignored when unavailable (missing or closed).
func (s *ValidateService) IgnoreUnavailable(ignoreUnavailable bool) *ValidateService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all` string
// or when no indices have been specified).
func (s *ValidateService) AllowNoIndices(allowNoIndices bool) *ValidateService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *ValidateService) ExpandWildcards(expandWildcards string) *ValidateService {
	s.expandWildcards = expandWildcards
	return s
}

// BodyJson sets the query definition using the Query DSL.
func (s *ValidateService) BodyJson(body interface{}) *ValidateService {
	s.bodyJson = body
	return s
}

// BodyString sets the query definition using the Query DSL as a string.
func (s *ValidateService) BodyString(body string) *ValidateService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *ValidateService) buildURL() (string, url.Values, error) {
	var err error
	var path string
	// Build URL
	if len(s.index) > 0 && len(s.typ) > 0 {
		path, err = uritemplates.Expand("/{index}/{type}/_validate/query", map[string]string{
			"index": strings.Join(s.index, ","),
			"type":  strings.Join(s.typ, ","),
		})
	} else if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_validate/query", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path, err = uritemplates.Expand("/_validate/query", map[string]string{
			"type": strings.Join(s.typ, ","),
		})
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
	if s.explain != nil {
		params.Set("explain", fmt.Sprintf("%v", *s.explain))
	}
	if s.rewrite != nil {
		params.Set("rewrite", fmt.Sprintf("%v", *s.rewrite))
	}
	if s.allShards != nil {
		params.Set("all_shards", fmt.Sprintf("%v", *s.allShards))
	}
	if s.defaultOperator != "" {
		params.Set("default_operator", s.defaultOperator)
	}
	if v := s.lenient; v != nil {
		params.Set("lenient", fmt.Sprint(*v))
	}
	if s.q != "" {
		params.Set("q", s.q)
	}
	if v := s.analyzeWildcard; v != nil {
		params.Set("analyze_wildcard", fmt.Sprint(*v))
	}
	if s.analyzer != "" {
		params.Set("analyzer", s.analyzer)
	}
	if s.df != "" {
		params.Set("df", s.df)
	}
	if v := s.allowNoIndices; v != nil {
		params.Set("allow_no_indices", fmt.Sprint(*v))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if v := s.ignoreUnavailable; v != nil {
		params.Set("ignore_unavailable", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ValidateService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *ValidateService) Do(ctx context.Context) (*ValidateResponse, error) {
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
		Method:  "GET",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(ValidateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ValidateResponse is the response of ValidateService.Do.
type ValidateResponse struct {
	Valid        bool                   `json:"valid"`
	Shards       map[string]interface{} `json:"_shards"`
	Explanations []interface{}          `json:"explanations"`
}
