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

	"github.com/olivere/elastic/v7/uritemplates"
)

// BulkService allows for batching bulk requests and sending them to
// Elasticsearch in one roundtrip. Use the Add method with BulkIndexRequest,
// BulkUpdateRequest, and BulkDeleteRequest to add bulk requests to a batch,
// then use Do to send them to Elasticsearch.
//
// BulkService will be reset after each Do call. In other words, you can
// reuse BulkService to send many batches. You do not have to create a new
// BulkService for each batch.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-bulk.html
// for more details.
type BulkService struct {
	client  *Client
	retrier Retrier

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index               string
	typ                 string
	requests            []BulkableRequest
	pipeline            string
	timeout             string
	refresh             string
	routing             string
	waitForActiveShards string

	// estimated bulk size in bytes, up to the request index sizeInBytesCursor
	sizeInBytes       int64
	sizeInBytesCursor int
}

// NewBulkService initializes a new BulkService.
func NewBulkService(client *Client) *BulkService {
	builder := &BulkService{
		client: client,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *BulkService) Pretty(pretty bool) *BulkService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *BulkService) Human(human bool) *BulkService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *BulkService) ErrorTrace(errorTrace bool) *BulkService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *BulkService) FilterPath(filterPath ...string) *BulkService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *BulkService) Header(name string, value string) *BulkService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *BulkService) Headers(headers http.Header) *BulkService {
	s.headers = headers
	return s
}

// Reset cleans up the request queue
func (s *BulkService) Reset() {
	s.requests = make([]BulkableRequest, 0)
	s.sizeInBytes = 0
	s.sizeInBytesCursor = 0
}

// Retrier allows to set specific retry logic for this BulkService.
// If not specified, it will use the client's default retrier.
func (s *BulkService) Retrier(retrier Retrier) *BulkService {
	s.retrier = retrier
	return s
}

// Index specifies the index to use for all batches. You may also leave
// this blank and specify the index in the individual bulk requests.
func (s *BulkService) Index(index string) *BulkService {
	s.index = index
	return s
}

// Type specifies the type to use for all batches. You may also leave
// this blank and specify the type in the individual bulk requests.
func (s *BulkService) Type(typ string) *BulkService {
	s.typ = typ
	return s
}

// Timeout is a global timeout for processing bulk requests. This is a
// server-side timeout, i.e. it tells Elasticsearch the time after which
// it should stop processing.
func (s *BulkService) Timeout(timeout string) *BulkService {
	s.timeout = timeout
	return s
}

// Refresh controls when changes made by this request are made visible
// to search. The allowed values are: "true" (refresh the relevant
// primary and replica shards immediately), "wait_for" (wait for the
// changes to be made visible by a refresh before replying), or "false"
// (no refresh related actions). The default value is "false".
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *BulkService) Refresh(refresh string) *BulkService {
	s.refresh = refresh
	return s
}

// Routing specifies the routing value.
func (s *BulkService) Routing(routing string) *BulkService {
	s.routing = routing
	return s
}

// Pipeline specifies the pipeline id to preprocess incoming documents with.
func (s *BulkService) Pipeline(pipeline string) *BulkService {
	s.pipeline = pipeline
	return s
}

// WaitForActiveShards sets the number of shard copies that must be active
// before proceeding with the bulk operation. Defaults to 1, meaning the
// primary shard only. Set to `all` for all shard copies, otherwise set to
// any non-negative value less than or equal to the total number of copies
// for the shard (number of replicas + 1).
func (s *BulkService) WaitForActiveShards(waitForActiveShards string) *BulkService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// Add adds bulkable requests, i.e. BulkIndexRequest, BulkUpdateRequest,
// and/or BulkDeleteRequest.
func (s *BulkService) Add(requests ...BulkableRequest) *BulkService {
	s.requests = append(s.requests, requests...)
	return s
}

// EstimatedSizeInBytes returns the estimated size of all bulkable
// requests added via Add.
func (s *BulkService) EstimatedSizeInBytes() int64 {
	if s.sizeInBytesCursor == len(s.requests) {
		return s.sizeInBytes
	}
	for _, r := range s.requests[s.sizeInBytesCursor:] {
		s.sizeInBytes += s.estimateSizeInBytes(r)
		s.sizeInBytesCursor++
	}
	return s.sizeInBytes
}

// estimateSizeInBytes returns the estimates size of the given
// bulkable request, i.e. BulkIndexRequest, BulkUpdateRequest, and
// BulkDeleteRequest.
func (s *BulkService) estimateSizeInBytes(r BulkableRequest) int64 {
	lines, _ := r.Source()
	size := 0
	for _, line := range lines {
		// +1 for the \n
		size += len(line) + 1
	}
	return int64(size)
}

// NumberOfActions returns the number of bulkable requests that need to
// be sent to Elasticsearch on the next batch.
func (s *BulkService) NumberOfActions() int {
	return len(s.requests)
}

func (s *BulkService) bodyAsString() (string, error) {
	// Pre-allocate to reduce allocs
	var buf strings.Builder
	buf.Grow(int(s.EstimatedSizeInBytes()))

	for _, req := range s.requests {
		source, err := req.Source()
		if err != nil {
			return "", err
		}
		for _, line := range source {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}

	return buf.String(), nil
}

// Do sends the batched requests to Elasticsearch. Note that, when successful,
// you can reuse the BulkService for the next batch as the list of bulk
// requests is cleared on success.
func (s *BulkService) Do(ctx context.Context) (*BulkResponse, error) {
	// No actions?
	if s.NumberOfActions() == 0 {
		return nil, errors.New("elastic: No bulk actions to commit")
	}

	// Get body
	body, err := s.bodyAsString()
	if err != nil {
		return nil, err
	}

	// Build url
	path := "/"
	if len(s.index) > 0 {
		index, err := uritemplates.Expand("{index}", map[string]string{
			"index": s.index,
		})
		if err != nil {
			return nil, err
		}
		path += index + "/"
	}
	if len(s.typ) > 0 {
		typ, err := uritemplates.Expand("{type}", map[string]string{
			"type": s.typ,
		})
		if err != nil {
			return nil, err
		}
		path += typ + "/"
	}
	path += "_bulk"

	// Parameters
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
	if s.pipeline != "" {
		params.Set("pipeline", s.pipeline)
	}
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}

	// Get response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:      "POST",
		Path:        path,
		Params:      params,
		Body:        body,
		ContentType: "application/x-ndjson",
		Retrier:     s.retrier,
		Headers:     s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return results
	ret := new(BulkResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}

	// Reset so the request can be reused
	s.Reset()

	return ret, nil
}

// BulkResponse is a response to a bulk execution.
//
// Example:
// {
//   "took":3,
//   "errors":false,
//   "items":[{
//     "index":{
//       "_index":"index1",
//       "_type":"tweet",
//       "_id":"1",
//       "_version":3,
//       "status":201
//     }
//   },{
//     "index":{
//       "_index":"index2",
//       "_type":"tweet",
//       "_id":"2",
//       "_version":3,
//       "status":200
//     }
//   },{
//     "delete":{
//       "_index":"index1",
//       "_type":"tweet",
//       "_id":"1",
//       "_version":4,
//       "status":200,
//       "found":true
//     }
//   },{
//     "update":{
//       "_index":"index2",
//       "_type":"tweet",
//       "_id":"2",
//       "_version":4,
//       "status":200
//     }
//   }]
// }
type BulkResponse struct {
	Took   int                            `json:"took,omitempty"`
	Errors bool                           `json:"errors,omitempty"`
	Items  []map[string]*BulkResponseItem `json:"items,omitempty"`
}

// BulkResponseItem is the result of a single bulk request.
type BulkResponseItem struct {
	Index         string        `json:"_index,omitempty"`
	Type          string        `json:"_type,omitempty"`
	Id            string        `json:"_id,omitempty"`
	Version       int64         `json:"_version,omitempty"`
	Result        string        `json:"result,omitempty"`
	Shards        *ShardsInfo   `json:"_shards,omitempty"`
	SeqNo         int64         `json:"_seq_no,omitempty"`
	PrimaryTerm   int64         `json:"_primary_term,omitempty"`
	Status        int           `json:"status,omitempty"`
	ForcedRefresh bool          `json:"forced_refresh,omitempty"`
	Error         *ErrorDetails `json:"error,omitempty"`
	GetResult     *GetResult    `json:"get,omitempty"`
}

// Indexed returns all bulk request results of "index" actions.
func (r *BulkResponse) Indexed() []*BulkResponseItem {
	return r.ByAction("index")
}

// Created returns all bulk request results of "create" actions.
func (r *BulkResponse) Created() []*BulkResponseItem {
	return r.ByAction("create")
}

// Updated returns all bulk request results of "update" actions.
func (r *BulkResponse) Updated() []*BulkResponseItem {
	return r.ByAction("update")
}

// Deleted returns all bulk request results of "delete" actions.
func (r *BulkResponse) Deleted() []*BulkResponseItem {
	return r.ByAction("delete")
}

// ByAction returns all bulk request results of a certain action,
// e.g. "index" or "delete".
func (r *BulkResponse) ByAction(action string) []*BulkResponseItem {
	if r.Items == nil {
		return nil
	}
	var items []*BulkResponseItem
	for _, item := range r.Items {
		if result, found := item[action]; found {
			items = append(items, result)
		}
	}
	return items
}

// ById returns all bulk request results of a given document id,
// regardless of the action ("index", "delete" etc.).
func (r *BulkResponse) ById(id string) []*BulkResponseItem {
	if r.Items == nil {
		return nil
	}
	var items []*BulkResponseItem
	for _, item := range r.Items {
		for _, result := range item {
			if result.Id == id {
				items = append(items, result)
			}
		}
	}
	return items
}

// Failed returns those items of a bulk response that have errors,
// i.e. those that don't have a status code between 200 and 299.
func (r *BulkResponse) Failed() []*BulkResponseItem {
	if r.Items == nil {
		return nil
	}
	var errors []*BulkResponseItem
	for _, item := range r.Items {
		for _, result := range item {
			if !(result.Status >= 200 && result.Status <= 299) {
				errors = append(errors, result)
			}
		}
	}
	return errors
}

// Succeeded returns those items of a bulk response that have no errors,
// i.e. those have a status code between 200 and 299.
func (r *BulkResponse) Succeeded() []*BulkResponseItem {
	if r.Items == nil {
		return nil
	}
	var succeeded []*BulkResponseItem
	for _, item := range r.Items {
		for _, result := range item {
			if result.Status >= 200 && result.Status <= 299 {
				succeeded = append(succeeded, result)
			}
		}
	}
	return succeeded
}
