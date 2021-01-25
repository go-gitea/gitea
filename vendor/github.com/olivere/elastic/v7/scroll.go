// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/olivere/elastic/v7/uritemplates"
)

const (
	// DefaultScrollKeepAlive is the default time a scroll cursor will be kept alive.
	DefaultScrollKeepAlive = "5m"
)

// ScrollService iterates over pages of search results from Elasticsearch.
type ScrollService struct {
	client  *Client
	retrier Retrier

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	indices            []string
	types              []string
	keepAlive          string
	body               interface{}
	ss                 *SearchSource
	size               *int
	routing            string
	preference         string
	ignoreUnavailable  *bool
	ignoreThrottled    *bool
	allowNoIndices     *bool
	expandWildcards    string
	maxResponseSize    int64
	restTotalHitsAsInt *bool

	mu       sync.RWMutex
	scrollId string
}

// NewScrollService initializes and returns a new ScrollService.
func NewScrollService(client *Client) *ScrollService {
	builder := &ScrollService{
		client:    client,
		ss:        NewSearchSource(),
		keepAlive: DefaultScrollKeepAlive,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ScrollService) Pretty(pretty bool) *ScrollService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ScrollService) Human(human bool) *ScrollService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ScrollService) ErrorTrace(errorTrace bool) *ScrollService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ScrollService) FilterPath(filterPath ...string) *ScrollService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ScrollService) Header(name string, value string) *ScrollService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ScrollService) Headers(headers http.Header) *ScrollService {
	s.headers = headers
	return s
}

// Retrier allows to set specific retry logic for this ScrollService.
// If not specified, it will use the client's default retrier.
func (s *ScrollService) Retrier(retrier Retrier) *ScrollService {
	s.retrier = retrier
	return s
}

// Index sets the name of one or more indices to iterate over.
func (s *ScrollService) Index(indices ...string) *ScrollService {
	if s.indices == nil {
		s.indices = make([]string, 0)
	}
	s.indices = append(s.indices, indices...)
	return s
}

// Type sets the name of one or more types to iterate over.
//
// Deprecated: Types are in the process of being removed. Instead of using a type, prefer to
// filter on a field on the document.
func (s *ScrollService) Type(types ...string) *ScrollService {
	if s.types == nil {
		s.types = make([]string, 0)
	}
	s.types = append(s.types, types...)
	return s
}

// Scroll is an alias for KeepAlive, the time to keep
// the cursor alive (e.g. "5m" for 5 minutes).
func (s *ScrollService) Scroll(keepAlive string) *ScrollService {
	s.keepAlive = keepAlive
	return s
}

// KeepAlive sets the maximum time after which the cursor will expire.
// It is "5m" by default.
func (s *ScrollService) KeepAlive(keepAlive string) *ScrollService {
	s.keepAlive = keepAlive
	return s
}

// Size specifies the number of documents Elasticsearch should return
// from each shard, per page.
func (s *ScrollService) Size(size int) *ScrollService {
	s.size = &size
	return s
}

// Highlight allows to highlight search results on one or more fields
func (s *ScrollService) Highlight(highlight *Highlight) *ScrollService {
	s.ss = s.ss.Highlight(highlight)
	return s
}

// Body sets the raw body to send to Elasticsearch. This can be e.g. a string,
// a map[string]interface{} or anything that can be serialized into JSON.
// Notice that setting the body disables the use of SearchSource and many
// other properties of the ScanService.
func (s *ScrollService) Body(body interface{}) *ScrollService {
	s.body = body
	return s
}

// SearchSource sets the search source builder to use with this iterator.
// Notice that only a certain number of properties can be used when scrolling,
// e.g. query and sorting.
func (s *ScrollService) SearchSource(searchSource *SearchSource) *ScrollService {
	s.ss = searchSource
	if s.ss == nil {
		s.ss = NewSearchSource()
	}
	return s
}

// Query sets the query to perform, e.g. a MatchAllQuery.
func (s *ScrollService) Query(query Query) *ScrollService {
	s.ss = s.ss.Query(query)
	return s
}

// PostFilter is executed as the last filter. It only affects the
// search hits but not facets. See
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-post-filter.html
// for details.
func (s *ScrollService) PostFilter(postFilter Query) *ScrollService {
	s.ss = s.ss.PostFilter(postFilter)
	return s
}

// Slice allows slicing the scroll request into several batches.
// This is supported in Elasticsearch 5.0 or later.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-scroll.html#sliced-scroll
// for details.
func (s *ScrollService) Slice(sliceQuery Query) *ScrollService {
	s.ss = s.ss.Slice(sliceQuery)
	return s
}

// FetchSource indicates whether the response should contain the stored
// _source for every hit.
func (s *ScrollService) FetchSource(fetchSource bool) *ScrollService {
	s.ss = s.ss.FetchSource(fetchSource)
	return s
}

// FetchSourceContext indicates how the _source should be fetched.
func (s *ScrollService) FetchSourceContext(fetchSourceContext *FetchSourceContext) *ScrollService {
	s.ss = s.ss.FetchSourceContext(fetchSourceContext)
	return s
}

// Version can be set to true to return a version for each search hit.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-request-version.html.
func (s *ScrollService) Version(version bool) *ScrollService {
	s.ss = s.ss.Version(version)
	return s
}

// Sort adds a sort order. This can have negative effects on the performance
// of the scroll operation as Elasticsearch needs to sort first.
func (s *ScrollService) Sort(field string, ascending bool) *ScrollService {
	s.ss = s.ss.Sort(field, ascending)
	return s
}

// SortWithInfo specifies a sort order. Notice that sorting can have a
// negative impact on scroll performance.
func (s *ScrollService) SortWithInfo(info SortInfo) *ScrollService {
	s.ss = s.ss.SortWithInfo(info)
	return s
}

// SortBy specifies a sort order. Notice that sorting can have a
// negative impact on scroll performance.
func (s *ScrollService) SortBy(sorter ...Sorter) *ScrollService {
	s.ss = s.ss.SortBy(sorter...)
	return s
}

// TrackTotalHits controls if the total hit count for the query should be tracked.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.1/search-request-track-total-hits.html
// for details.
func (s *ScrollService) TrackTotalHits(trackTotalHits interface{}) *ScrollService {
	s.ss = s.ss.TrackTotalHits(trackTotalHits)
	return s
}

// RestTotalHitsAsInt indicates whether hits.total should be rendered as an
// integer or an object in the rest search response.
func (s *ScrollService) RestTotalHitsAsInt(enabled bool) *ScrollService {
	s.restTotalHitsAsInt = &enabled
	return s
}

// Routing is a list of specific routing values to control the shards
// the search will be executed on.
func (s *ScrollService) Routing(routings ...string) *ScrollService {
	s.routing = strings.Join(routings, ",")
	return s
}

// Preference sets the preference to execute the search. Defaults to
// randomize across shards ("random"). Can be set to "_local" to prefer
// local shards, "_primary" to execute on primary shards only,
// or a custom value which guarantees that the same order will be used
// across different requests.
func (s *ScrollService) Preference(preference string) *ScrollService {
	s.preference = preference
	return s
}

// IgnoreUnavailable indicates whether the specified concrete indices
// should be ignored when unavailable (missing or closed).
func (s *ScrollService) IgnoreUnavailable(ignoreUnavailable bool) *ScrollService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// IgnoreThrottled indicates whether specified concrete, expanded or aliased
// indices should be ignored when throttled.
func (s *ScrollService) IgnoreThrottled(ignoreThrottled bool) *ScrollService {
	s.ignoreThrottled = &ignoreThrottled
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices
// expression resolves into no concrete indices. (This includes `_all` string
// or when no indices have been specified).
func (s *ScrollService) AllowNoIndices(allowNoIndices bool) *ScrollService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether to expand wildcard expression to
// concrete indices that are open, closed or both.
func (s *ScrollService) ExpandWildcards(expandWildcards string) *ScrollService {
	s.expandWildcards = expandWildcards
	return s
}

// MaxResponseSize sets an upper limit on the response body size that we accept,
// to guard against OOM situations.
func (s *ScrollService) MaxResponseSize(maxResponseSize int64) *ScrollService {
	s.maxResponseSize = maxResponseSize
	return s
}

// NoStoredFields indicates that no stored fields should be loaded, resulting in only
// id and type to be returned per field.
func (s *ScrollService) NoStoredFields() *ScrollService {
	s.ss = s.ss.NoStoredFields()
	return s
}

// StoredField adds a single field to load and return (note, must be stored) as
// part of the search request. If none are specified, the source of the
// document will be returned.
func (s *ScrollService) StoredField(fieldName string) *ScrollService {
	s.ss = s.ss.StoredField(fieldName)
	return s
}

// StoredFields	sets the fields to load and return as part of the search request.
// If none are specified, the source of the document will be returned.
func (s *ScrollService) StoredFields(fields ...string) *ScrollService {
	s.ss = s.ss.StoredFields(fields...)
	return s
}

// ScrollId specifies the identifier of a scroll in action.
func (s *ScrollService) ScrollId(scrollId string) *ScrollService {
	s.mu.Lock()
	s.scrollId = scrollId
	s.mu.Unlock()
	return s
}

// Do returns the next search result. It will return io.EOF as error if there
// are no more search results.
func (s *ScrollService) Do(ctx context.Context) (*SearchResult, error) {
	s.mu.RLock()
	nextScrollId := s.scrollId
	s.mu.RUnlock()
	if len(nextScrollId) == 0 {
		return s.first(ctx)
	}
	return s.next(ctx)
}

// Clear cancels the current scroll operation. If you don't do this manually,
// the scroll will be expired automatically by Elasticsearch. You can control
// how long a scroll cursor is kept alive with the KeepAlive func.
func (s *ScrollService) Clear(ctx context.Context) error {
	s.mu.RLock()
	scrollId := s.scrollId
	s.mu.RUnlock()
	if len(scrollId) == 0 {
		return nil
	}

	path := "/_search/scroll"
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
	body := struct {
		ScrollId []string `json:"scroll_id,omitempty"`
	}{
		ScrollId: []string{scrollId},
	}

	_, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "DELETE",
		Path:    path,
		Params:  params,
		Body:    body,
		Retrier: s.retrier,
	})
	if err != nil {
		return err
	}

	return nil
}

// -- First --

// first takes the first page of search results.
func (s *ScrollService) first(ctx context.Context) (*SearchResult, error) {
	// Get URL and parameters for request
	path, params, err := s.buildFirstURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP request body
	body, err := s.bodyFirst()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:          "POST",
		Path:            path,
		Params:          params,
		Body:            body,
		Retrier:         s.retrier,
		Headers:         s.headers,
		MaxResponseSize: s.maxResponseSize,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(SearchResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.scrollId = ret.ScrollId
	s.mu.Unlock()
	if ret.Hits == nil || len(ret.Hits.Hits) == 0 {
		return ret, io.EOF
	}
	return ret, nil
}

// buildFirstURL builds the URL for retrieving the first page.
func (s *ScrollService) buildFirstURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.indices) == 0 && len(s.types) == 0 {
		path = "/_search"
	} else if len(s.indices) > 0 && len(s.types) == 0 {
		path, err = uritemplates.Expand("/{index}/_search", map[string]string{
			"index": strings.Join(s.indices, ","),
		})
	} else if len(s.indices) == 0 && len(s.types) > 0 {
		path, err = uritemplates.Expand("/_all/{typ}/_search", map[string]string{
			"typ": strings.Join(s.types, ","),
		})
	} else {
		path, err = uritemplates.Expand("/{index}/{typ}/_search", map[string]string{
			"index": strings.Join(s.indices, ","),
			"typ":   strings.Join(s.types, ","),
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
		// Always add "hits._scroll_id", otherwise we cannot scroll
		var found bool
		for _, path := range s.filterPath {
			if path == "_scroll_id" {
				found = true
				break
			}
		}
		if !found {
			s.filterPath = append(s.filterPath, "_scroll_id")
		}
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.size != nil && *s.size > 0 {
		params.Set("size", fmt.Sprintf("%d", *s.size))
	}
	if len(s.keepAlive) > 0 {
		params.Set("scroll", s.keepAlive)
	}
	if len(s.routing) > 0 {
		params.Set("routing", s.routing)
	}
	if len(s.preference) > 0 {
		params.Set("preference", s.preference)
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if len(s.expandWildcards) > 0 {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.ignoreThrottled != nil {
		params.Set("ignore_throttled", fmt.Sprintf("%v", *s.ignoreThrottled))
	}
	if v := s.restTotalHitsAsInt; v != nil {
		params.Set("rest_total_hits_as_int", fmt.Sprint(*v))
	}

	return path, params, nil
}

// bodyFirst returns the request to fetch the first batch of results.
func (s *ScrollService) bodyFirst() (interface{}, error) {
	var err error
	var body interface{}

	if s.body != nil {
		body = s.body
	} else {
		// Use _doc sort by default if none is specified
		if !s.ss.hasSort() {
			// Use efficient sorting when no user-defined query/body is specified
			s.ss = s.ss.SortBy(SortByDoc{})
		}

		// Body from search source
		body, err = s.ss.Source()
		if err != nil {
			return nil, err
		}
	}

	return body, nil
}

// -- Next --

func (s *ScrollService) next(ctx context.Context) (*SearchResult, error) {
	// Get URL for request
	path, params, err := s.buildNextURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	body, err := s.bodyNext()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:          "POST",
		Path:            path,
		Params:          params,
		Body:            body,
		Retrier:         s.retrier,
		Headers:         s.headers,
		MaxResponseSize: s.maxResponseSize,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(SearchResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.scrollId = ret.ScrollId
	s.mu.Unlock()
	if ret.Hits == nil || len(ret.Hits.Hits) == 0 {
		return ret, io.EOF
	}
	return ret, nil
}

// buildNextURL builds the URL for the operation.
func (s *ScrollService) buildNextURL() (string, url.Values, error) {
	path := "/_search/scroll"

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
		// Always add "hits._scroll_id", otherwise we cannot scroll
		var found bool
		for _, path := range s.filterPath {
			if path == "_scroll_id" {
				found = true
				break
			}
		}
		if !found {
			s.filterPath = append(s.filterPath, "_scroll_id")
		}
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if v := s.restTotalHitsAsInt; v != nil {
		params.Set("rest_total_hits_as_int", fmt.Sprint(*v))
	}

	return path, params, nil
}

// body returns the request to fetch the next batch of results.
func (s *ScrollService) bodyNext() (interface{}, error) {
	s.mu.RLock()
	body := struct {
		Scroll   string `json:"scroll"`
		ScrollId string `json:"scroll_id,omitempty"`
	}{
		Scroll:   s.keepAlive,
		ScrollId: s.scrollId,
	}
	s.mu.RUnlock()
	return body, nil
}

// DocvalueField adds a single field to load from the field data cache
// and return as part of the search.
func (s *ScrollService) DocvalueField(docvalueField string) *ScrollService {
	s.ss = s.ss.DocvalueField(docvalueField)
	return s
}

// DocvalueFieldWithFormat adds a single field to load from the field data cache
// and return as part of the search.
func (s *ScrollService) DocvalueFieldWithFormat(docvalueField DocvalueField) *ScrollService {
	s.ss = s.ss.DocvalueFieldWithFormat(docvalueField)
	return s
}

// DocvalueFields adds one or more fields to load from the field data cache
// and return as part of the search.
func (s *ScrollService) DocvalueFields(docvalueFields ...string) *ScrollService {
	s.ss = s.ss.DocvalueFields(docvalueFields...)
	return s
}

// DocvalueFieldsWithFormat adds one or more fields to load from the field data cache
// and return as part of the search.
func (s *ScrollService) DocvalueFieldsWithFormat(docvalueFields ...DocvalueField) *ScrollService {
	s.ss = s.ss.DocvalueFieldsWithFormat(docvalueFields...)
	return s
}
