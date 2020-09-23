// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ReindexService is a method to copy documents from one index to another.
// It is documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-reindex.html.
type ReindexService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	refresh             string
	timeout             string
	waitForActiveShards string
	waitForCompletion   *bool
	requestsPerSecond   *int
	slices              interface{}
	body                interface{}
	source              *ReindexSource
	destination         *ReindexDestination
	conflicts           string
	size                *int
	script              *Script
}

// NewReindexService creates a new ReindexService.
func NewReindexService(client *Client) *ReindexService {
	return &ReindexService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ReindexService) Pretty(pretty bool) *ReindexService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ReindexService) Human(human bool) *ReindexService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ReindexService) ErrorTrace(errorTrace bool) *ReindexService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ReindexService) FilterPath(filterPath ...string) *ReindexService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ReindexService) Header(name string, value string) *ReindexService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ReindexService) Headers(headers http.Header) *ReindexService {
	s.headers = headers
	return s
}

// WaitForActiveShards sets the number of shard copies that must be active before
// proceeding with the reindex operation. Defaults to 1, meaning the primary shard only.
// Set to `all` for all shard copies, otherwise set to any non-negative value less than or
// equal to the total number of copies for the shard (number of replicas + 1).
func (s *ReindexService) WaitForActiveShards(waitForActiveShards string) *ReindexService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// RequestsPerSecond specifies the throttle to set on this request in sub-requests per second.
// -1 means set no throttle as does "unlimited" which is the only non-float this accepts.
func (s *ReindexService) RequestsPerSecond(requestsPerSecond int) *ReindexService {
	s.requestsPerSecond = &requestsPerSecond
	return s
}

// Slices specifies the number of slices this task should be divided into. Defaults to 1.
// It used to  be a number, but can be set to "auto" as of 6.7.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-reindex.html#docs-reindex-slice
// for details.
func (s *ReindexService) Slices(slices interface{}) *ReindexService {
	s.slices = slices
	return s
}

// Refresh indicates whether Elasticsearch should refresh the effected indexes
// immediately.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *ReindexService) Refresh(refresh string) *ReindexService {
	s.refresh = refresh
	return s
}

// Timeout is the time each individual bulk request should wait for shards
// that are unavailable.
func (s *ReindexService) Timeout(timeout string) *ReindexService {
	s.timeout = timeout
	return s
}

// WaitForCompletion indicates whether Elasticsearch should block until the
// reindex is complete.
func (s *ReindexService) WaitForCompletion(waitForCompletion bool) *ReindexService {
	s.waitForCompletion = &waitForCompletion
	return s
}

// Source specifies the source of the reindexing process.
func (s *ReindexService) Source(source *ReindexSource) *ReindexService {
	s.source = source
	return s
}

// SourceIndex specifies the source index of the reindexing process.
func (s *ReindexService) SourceIndex(index string) *ReindexService {
	if s.source == nil {
		s.source = NewReindexSource()
	}
	s.source = s.source.Index(index)
	return s
}

// Destination specifies the destination of the reindexing process.
func (s *ReindexService) Destination(destination *ReindexDestination) *ReindexService {
	s.destination = destination
	return s
}

// DestinationIndex specifies the destination index of the reindexing process.
func (s *ReindexService) DestinationIndex(index string) *ReindexService {
	if s.destination == nil {
		s.destination = NewReindexDestination()
	}
	s.destination = s.destination.Index(index)
	return s
}

// DestinationIndexAndType specifies both the destination index and type
// of the reindexing process.
func (s *ReindexService) DestinationIndexAndType(index, typ string) *ReindexService {
	if s.destination == nil {
		s.destination = NewReindexDestination()
	}
	s.destination = s.destination.Index(index)
	s.destination = s.destination.Type(typ)
	return s
}

// Conflicts indicates what to do when the process detects version conflicts.
// Possible values are "proceed" and "abort".
func (s *ReindexService) Conflicts(conflicts string) *ReindexService {
	s.conflicts = conflicts
	return s
}

// AbortOnVersionConflict aborts the request on version conflicts.
// It is an alias to setting Conflicts("abort").
func (s *ReindexService) AbortOnVersionConflict() *ReindexService {
	s.conflicts = "abort"
	return s
}

// ProceedOnVersionConflict aborts the request on version conflicts.
// It is an alias to setting Conflicts("proceed").
func (s *ReindexService) ProceedOnVersionConflict() *ReindexService {
	s.conflicts = "proceed"
	return s
}

// Size sets an upper limit for the number of processed documents.
func (s *ReindexService) Size(size int) *ReindexService {
	s.size = &size
	return s
}

// Script allows for modification of the documents as they are reindexed
// from source to destination.
func (s *ReindexService) Script(script *Script) *ReindexService {
	s.script = script
	return s
}

// Body specifies the body of the request to send to Elasticsearch.
// It overrides settings specified with other setters, e.g. Query.
func (s *ReindexService) Body(body interface{}) *ReindexService {
	s.body = body
	return s
}

// buildURL builds the URL for the operation.
func (s *ReindexService) buildURL() (string, url.Values, error) {
	// Build URL path
	path := "/_reindex"

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
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.requestsPerSecond != nil {
		params.Set("requests_per_second", fmt.Sprintf("%v", *s.requestsPerSecond))
	}
	if s.slices != nil {
		params.Set("slices", fmt.Sprintf("%v", s.slices))
	}
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}
	if s.waitForCompletion != nil {
		params.Set("wait_for_completion", fmt.Sprintf("%v", *s.waitForCompletion))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ReindexService) Validate() error {
	var invalid []string
	if s.body != nil {
		return nil
	}
	if s.source == nil {
		invalid = append(invalid, "Source")
	} else {
		if len(s.source.request.indices) == 0 {
			invalid = append(invalid, "Source.Index")
		}
	}
	if s.destination == nil {
		invalid = append(invalid, "Destination")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// getBody returns the body part of the document request.
func (s *ReindexService) getBody() (interface{}, error) {
	if s.body != nil {
		return s.body, nil
	}

	body := make(map[string]interface{})

	if s.conflicts != "" {
		body["conflicts"] = s.conflicts
	}
	if s.size != nil {
		body["size"] = *s.size
	}
	if s.script != nil {
		out, err := s.script.Source()
		if err != nil {
			return nil, err
		}
		body["script"] = out
	}

	src, err := s.source.Source()
	if err != nil {
		return nil, err
	}
	body["source"] = src

	dst, err := s.destination.Source()
	if err != nil {
		return nil, err
	}
	body["dest"] = dst

	return body, nil
}

// Do executes the operation.
func (s *ReindexService) Do(ctx context.Context) (*BulkIndexByScrollResponse, error) {
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
	body, err := s.getBody()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
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

	// Return operation response
	ret := new(BulkIndexByScrollResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	ret.Header = res.Header
	return ret, nil
}

// DoAsync executes the reindexing operation asynchronously by starting a new task.
// Callers need to use the Task Management API to watch the outcome of the reindexing
// operation.
func (s *ReindexService) DoAsync(ctx context.Context) (*StartTaskResult, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// DoAsync only makes sense with WaitForCompletion set to true
	if s.waitForCompletion != nil && *s.waitForCompletion {
		return nil, fmt.Errorf("cannot start a task with WaitForCompletion set to true")
	}
	f := false
	s.waitForCompletion = &f

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	body, err := s.getBody()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
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

	// Return operation response
	ret := new(StartTaskResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	ret.Header = res.Header
	return ret, nil
}

// -- Source of Reindex --

// ReindexSource specifies the source of a Reindex process.
type ReindexSource struct {
	request    *SearchRequest
	remoteInfo *ReindexRemoteInfo
}

// NewReindexSource creates a new ReindexSource.
func NewReindexSource() *ReindexSource {
	return &ReindexSource{
		request: NewSearchRequest(),
	}
}

// Request specifies the search request used for source.
func (r *ReindexSource) Request(request *SearchRequest) *ReindexSource {
	if request == nil {
		r.request = NewSearchRequest()
	} else {
		r.request = request
	}
	return r
}

// SearchType is the search operation type. Possible values are
// "query_then_fetch" and "dfs_query_then_fetch".
func (r *ReindexSource) SearchType(searchType string) *ReindexSource {
	r.request = r.request.SearchType(searchType)
	return r
}

func (r *ReindexSource) SearchTypeDfsQueryThenFetch() *ReindexSource {
	r.request = r.request.SearchType("dfs_query_then_fetch")
	return r
}

func (r *ReindexSource) SearchTypeQueryThenFetch() *ReindexSource {
	r.request = r.request.SearchType("query_then_fetch")
	return r
}

func (r *ReindexSource) Index(indices ...string) *ReindexSource {
	r.request = r.request.Index(indices...)
	return r
}

func (r *ReindexSource) Type(types ...string) *ReindexSource {
	r.request = r.request.Type(types...)
	return r
}

func (r *ReindexSource) Preference(preference string) *ReindexSource {
	r.request = r.request.Preference(preference)
	return r
}

func (r *ReindexSource) RequestCache(requestCache bool) *ReindexSource {
	r.request = r.request.RequestCache(requestCache)
	return r
}

func (r *ReindexSource) Scroll(scroll string) *ReindexSource {
	r.request = r.request.Scroll(scroll)
	return r
}

func (r *ReindexSource) Query(query Query) *ReindexSource {
	r.request = r.request.Query(query)
	return r
}

// Sort adds a sort order.
func (r *ReindexSource) Sort(field string, ascending bool) *ReindexSource {
	r.request = r.request.Sort(field, ascending)
	return r
}

// SortWithInfo adds a sort order.
func (r *ReindexSource) SortWithInfo(info SortInfo) *ReindexSource {
	r.request = r.request.SortWithInfo(info)
	return r
}

// SortBy adds a sort order.
func (r *ReindexSource) SortBy(sorter ...Sorter) *ReindexSource {
	r.request = r.request.SortBy(sorter...)
	return r
}

// FetchSource indicates whether the response should contain the stored
// _source for every hit.
func (r *ReindexSource) FetchSource(fetchSource bool) *ReindexSource {
	r.request = r.request.FetchSource(fetchSource)
	return r
}

// FetchSourceIncludeExclude specifies that _source should be returned
// with each hit, where "include" and "exclude" serve as a simple wildcard
// matcher that gets applied to its fields
// (e.g. include := []string{"obj1.*","obj2.*"}, exclude := []string{"description.*"}).
func (r *ReindexSource) FetchSourceIncludeExclude(include, exclude []string) *ReindexSource {
	r.request = r.request.FetchSourceIncludeExclude(include, exclude)
	return r
}

// FetchSourceContext indicates how the _source should be fetched.
func (r *ReindexSource) FetchSourceContext(fsc *FetchSourceContext) *ReindexSource {
	r.request = r.request.FetchSourceContext(fsc)
	return r
}

// RemoteInfo sets up reindexing from a remote cluster.
func (r *ReindexSource) RemoteInfo(ri *ReindexRemoteInfo) *ReindexSource {
	r.remoteInfo = ri
	return r
}

// Source returns a serializable JSON request for the request.
func (r *ReindexSource) Source() (interface{}, error) {
	src, err := r.request.sourceAsMap()
	if err != nil {
		return nil, err
	}
	source, ok := src.(map[string]interface{})
	if !ok {
		return nil, errors.New("unable to use SearchRequest as map[string]interface{}")
	}

	switch len(r.request.indices) {
	case 1:
		source["index"] = r.request.indices[0]
	default:
		source["index"] = r.request.indices
	}
	switch len(r.request.types) {
	case 0:
	case 1:
		source["type"] = r.request.types[0]
	default:
		source["type"] = r.request.types
	}
	if r.remoteInfo != nil {
		src, err := r.remoteInfo.Source()
		if err != nil {
			return nil, err
		}
		source["remote"] = src
	}
	return source, nil
}

// ReindexRemoteInfo contains information for reindexing from a remote cluster.
type ReindexRemoteInfo struct {
	host           string
	username       string
	password       string
	socketTimeout  string // e.g. "1m" or "30s"
	connectTimeout string // e.g. "1m" or "30s"
}

// NewReindexRemoteInfo creates a new ReindexRemoteInfo.
func NewReindexRemoteInfo() *ReindexRemoteInfo {
	return &ReindexRemoteInfo{}
}

// Host sets the host information of the remote cluster.
// It must be of the form "http(s)://<hostname>:<port>"
func (ri *ReindexRemoteInfo) Host(host string) *ReindexRemoteInfo {
	ri.host = host
	return ri
}

// Username sets the username to authenticate with the remote cluster.
func (ri *ReindexRemoteInfo) Username(username string) *ReindexRemoteInfo {
	ri.username = username
	return ri
}

// Password sets the password to authenticate with the remote cluster.
func (ri *ReindexRemoteInfo) Password(password string) *ReindexRemoteInfo {
	ri.password = password
	return ri
}

// SocketTimeout sets the socket timeout to connect with the remote cluster.
// Use ES compatible values like e.g. "30s" or "1m".
func (ri *ReindexRemoteInfo) SocketTimeout(timeout string) *ReindexRemoteInfo {
	ri.socketTimeout = timeout
	return ri
}

// ConnectTimeout sets the connection timeout to connect with the remote cluster.
// Use ES compatible values like e.g. "30s" or "1m".
func (ri *ReindexRemoteInfo) ConnectTimeout(timeout string) *ReindexRemoteInfo {
	ri.connectTimeout = timeout
	return ri
}

// Source returns the serializable JSON data for the request.
func (ri *ReindexRemoteInfo) Source() (interface{}, error) {
	res := make(map[string]interface{})
	res["host"] = ri.host
	if len(ri.username) > 0 {
		res["username"] = ri.username
	}
	if len(ri.password) > 0 {
		res["password"] = ri.password
	}
	if len(ri.socketTimeout) > 0 {
		res["socket_timeout"] = ri.socketTimeout
	}
	if len(ri.connectTimeout) > 0 {
		res["connect_timeout"] = ri.connectTimeout
	}
	return res, nil
}

// -source Destination of Reindex --

// ReindexDestination is the destination of a Reindex API call.
// It is basically the meta data of a BulkIndexRequest.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-reindex.html
// fsourcer details.
type ReindexDestination struct {
	index       string
	typ         string
	routing     string
	parent      string
	opType      string
	version     int64  // default is MATCH_ANY
	versionType string // default is "internal"
	pipeline    string
}

// NewReindexDestination returns a new ReindexDestination.
func NewReindexDestination() *ReindexDestination {
	return &ReindexDestination{}
}

// Index specifies name of the Elasticsearch index to use as the destination
// of a reindexing process.
func (r *ReindexDestination) Index(index string) *ReindexDestination {
	r.index = index
	return r
}

// Type specifies the Elasticsearch type to use for reindexing.
func (r *ReindexDestination) Type(typ string) *ReindexDestination {
	r.typ = typ
	return r
}

// Routing specifies a routing value for the reindexing request.
// It can be "keep", "discard", or start with "=". The latter specifies
// the routing on the bulk request.
func (r *ReindexDestination) Routing(routing string) *ReindexDestination {
	r.routing = routing
	return r
}

// Keep sets the routing on the bulk request sent for each match to the routing
// of the match (the default).
func (r *ReindexDestination) Keep() *ReindexDestination {
	r.routing = "keep"
	return r
}

// Discard sets the routing on the bulk request sent for each match to null.
func (r *ReindexDestination) Discard() *ReindexDestination {
	r.routing = "discard"
	return r
}

// Parent specifies the identifier of the parent document (if available).
func (r *ReindexDestination) Parent(parent string) *ReindexDestination {
	r.parent = parent
	return r
}

// OpType specifies if this request should follow create-only or upsert
// behavior. This follows the OpType of the standard document index API.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-index_.html#operation-type
// for details.
func (r *ReindexDestination) OpType(opType string) *ReindexDestination {
	r.opType = opType
	return r
}

// Version indicates the version of the document as part of an optimistic
// concurrency model.
func (r *ReindexDestination) Version(version int64) *ReindexDestination {
	r.version = version
	return r
}

// VersionType specifies how versions are created.
func (r *ReindexDestination) VersionType(versionType string) *ReindexDestination {
	r.versionType = versionType
	return r
}

// Pipeline specifies the pipeline to use for reindexing.
func (r *ReindexDestination) Pipeline(pipeline string) *ReindexDestination {
	r.pipeline = pipeline
	return r
}

// Source returns a serializable JSON request for the request.
func (r *ReindexDestination) Source() (interface{}, error) {
	source := make(map[string]interface{})
	if r.index != "" {
		source["index"] = r.index
	}
	if r.typ != "" {
		source["type"] = r.typ
	}
	if r.routing != "" {
		source["routing"] = r.routing
	}
	if r.opType != "" {
		source["op_type"] = r.opType
	}
	if r.parent != "" {
		source["parent"] = r.parent
	}
	if r.version > 0 {
		source["version"] = r.version
	}
	if r.versionType != "" {
		source["version_type"] = r.versionType
	}
	if r.pipeline != "" {
		source["pipeline"] = r.pipeline
	}
	return source, nil
}
