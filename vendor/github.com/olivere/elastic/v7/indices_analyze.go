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

// IndicesAnalyzeService performs the analysis process on a text and returns
// the tokens breakdown of the text.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-analyze.html
// for detail.
type IndicesAnalyzeService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index       string
	request     *IndicesAnalyzeRequest
	format      string
	preferLocal *bool
	bodyJson    interface{}
	bodyString  string
}

// NewIndicesAnalyzeService creates a new IndicesAnalyzeService.
func NewIndicesAnalyzeService(client *Client) *IndicesAnalyzeService {
	return &IndicesAnalyzeService{
		client:  client,
		request: new(IndicesAnalyzeRequest),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesAnalyzeService) Pretty(pretty bool) *IndicesAnalyzeService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesAnalyzeService) Human(human bool) *IndicesAnalyzeService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesAnalyzeService) ErrorTrace(errorTrace bool) *IndicesAnalyzeService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesAnalyzeService) FilterPath(filterPath ...string) *IndicesAnalyzeService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesAnalyzeService) Header(name string, value string) *IndicesAnalyzeService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesAnalyzeService) Headers(headers http.Header) *IndicesAnalyzeService {
	s.headers = headers
	return s
}

// Index is the name of the index to scope the operation.
func (s *IndicesAnalyzeService) Index(index string) *IndicesAnalyzeService {
	s.index = index
	return s
}

// Format of the output.
func (s *IndicesAnalyzeService) Format(format string) *IndicesAnalyzeService {
	s.format = format
	return s
}

// PreferLocal, when true, specifies that a local shard should be used
// if available. When false, a random shard is used (default: true).
func (s *IndicesAnalyzeService) PreferLocal(preferLocal bool) *IndicesAnalyzeService {
	s.preferLocal = &preferLocal
	return s
}

// Request passes the analyze request to use.
func (s *IndicesAnalyzeService) Request(request *IndicesAnalyzeRequest) *IndicesAnalyzeService {
	if request == nil {
		s.request = new(IndicesAnalyzeRequest)
	} else {
		s.request = request
	}
	return s
}

// Analyzer is the name of the analyzer to use.
func (s *IndicesAnalyzeService) Analyzer(analyzer string) *IndicesAnalyzeService {
	s.request.Analyzer = analyzer
	return s
}

// Attributes is a list of token attributes to output; this parameter works
// only with explain=true.
func (s *IndicesAnalyzeService) Attributes(attributes ...string) *IndicesAnalyzeService {
	s.request.Attributes = attributes
	return s
}

// CharFilter is a list of character filters to use for the analysis.
func (s *IndicesAnalyzeService) CharFilter(charFilter ...string) *IndicesAnalyzeService {
	s.request.CharFilter = charFilter
	return s
}

// Explain, when true, outputs more advanced details (default: false).
func (s *IndicesAnalyzeService) Explain(explain bool) *IndicesAnalyzeService {
	s.request.Explain = explain
	return s
}

// Field specifies to use a specific analyzer configured for this field (instead of passing the analyzer name).
func (s *IndicesAnalyzeService) Field(field string) *IndicesAnalyzeService {
	s.request.Field = field
	return s
}

// Filter is a list of filters to use for the analysis.
func (s *IndicesAnalyzeService) Filter(filter ...string) *IndicesAnalyzeService {
	s.request.Filter = filter
	return s
}

// Text is the text on which the analysis should be performed (when request body is not used).
func (s *IndicesAnalyzeService) Text(text ...string) *IndicesAnalyzeService {
	s.request.Text = text
	return s
}

// Tokenizer is the name of the tokenizer to use for the analysis.
func (s *IndicesAnalyzeService) Tokenizer(tokenizer string) *IndicesAnalyzeService {
	s.request.Tokenizer = tokenizer
	return s
}

// BodyJson is the text on which the analysis should be performed.
func (s *IndicesAnalyzeService) BodyJson(body interface{}) *IndicesAnalyzeService {
	s.bodyJson = body
	return s
}

// BodyString is the text on which the analysis should be performed.
func (s *IndicesAnalyzeService) BodyString(body string) *IndicesAnalyzeService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesAnalyzeService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string

	if s.index == "" {
		path = "/_analyze"
	} else {
		path, err = uritemplates.Expand("/{index}/_analyze", map[string]string{
			"index": s.index,
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
	if s.format != "" {
		params.Set("format", s.format)
	}
	if s.preferLocal != nil {
		params.Set("prefer_local", fmt.Sprintf("%v", *s.preferLocal))
	}

	return path, params, nil
}

// Do will execute the request with the given context.
func (s *IndicesAnalyzeService) Do(ctx context.Context) (*IndicesAnalyzeResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else if s.bodyString != "" {
		body = s.bodyString
	} else {
		// Request parameters are deprecated in 5.1.1, and we must use a JSON
		// structure in the body to pass the parameters.
		// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-analyze.html
		body = s.request
	}

	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "POST",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	ret := new(IndicesAnalyzeResponse)
	if err = s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *IndicesAnalyzeService) Validate() error {
	var invalid []string
	if s.bodyJson == nil && s.bodyString == "" {
		if len(s.request.Text) == 0 {
			invalid = append(invalid, "Text")
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// IndicesAnalyzeRequest specifies the parameters of the analyze request.
type IndicesAnalyzeRequest struct {
	Text       []string `json:"text,omitempty"`
	Analyzer   string   `json:"analyzer,omitempty"`
	Tokenizer  string   `json:"tokenizer,omitempty"`
	Filter     []string `json:"filter,omitempty"`
	CharFilter []string `json:"char_filter,omitempty"`
	Field      string   `json:"field,omitempty"`
	Explain    bool     `json:"explain,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

type IndicesAnalyzeResponse struct {
	Tokens []AnalyzeToken               `json:"tokens"` // json part for normal message
	Detail IndicesAnalyzeResponseDetail `json:"detail"` // json part for verbose message of explain request
}

type AnalyzeTokenList struct {
	Name   string         `json:"name"`
	Tokens []AnalyzeToken `json:"tokens,omitempty"`
}

type AnalyzeToken struct {
	Token          string `json:"token"`
	Type           string `json:"type"` // e.g. "<ALPHANUM>"
	StartOffset    int    `json:"start_offset"`
	EndOffset      int    `json:"end_offset"`
	Bytes          string `json:"bytes"` // e.g. "[67 75 79]"
	Position       int    `json:"position"`
	PositionLength int    `json:"positionLength"` // seems to be wrong in 7.2+ (no snake_case), see https://github.com/elastic/elasticsearch/blob/7.2/server/src/main/java/org/elasticsearch/action/admin/indices/analyze/AnalyzeResponse.java
	TermFrequency  int    `json:"termFrequency"`
	Keyword        bool   `json:"keyword"`
}

type CharFilteredText struct {
	Name         string   `json:"name"`
	FilteredText []string `json:"filtered_text"`
}

type IndicesAnalyzeResponseDetail struct {
	CustomAnalyzer bool                `json:"custom_analyzer"`
	Analyzer       *AnalyzeTokenList   `json:"analyzer,omitempty"`
	Charfilters    []*CharFilteredText `json:"charfilters,omitempty"`
	Tokenizer      *AnalyzeTokenList   `json:"tokenizer,omitempty"`
	TokenFilters   []*AnalyzeTokenList `json:"tokenfilters,omitempty"`
}
