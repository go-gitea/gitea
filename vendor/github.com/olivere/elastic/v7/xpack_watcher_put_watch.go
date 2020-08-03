// Copyright 2012-2018 Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// XPackWatcherPutWatchService either registers a new watch in Watcher
// or update an existing one.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/watcher-api-put-watch.html.
type XPackWatcherPutWatchService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id            string
	active        *bool
	masterTimeout string
	ifSeqNo       *int64
	ifPrimaryTerm *int64
	body          interface{}
}

// NewXPackWatcherPutWatchService creates a new XPackWatcherPutWatchService.
func NewXPackWatcherPutWatchService(client *Client) *XPackWatcherPutWatchService {
	return &XPackWatcherPutWatchService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackWatcherPutWatchService) Pretty(pretty bool) *XPackWatcherPutWatchService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackWatcherPutWatchService) Human(human bool) *XPackWatcherPutWatchService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackWatcherPutWatchService) ErrorTrace(errorTrace bool) *XPackWatcherPutWatchService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackWatcherPutWatchService) FilterPath(filterPath ...string) *XPackWatcherPutWatchService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackWatcherPutWatchService) Header(name string, value string) *XPackWatcherPutWatchService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackWatcherPutWatchService) Headers(headers http.Header) *XPackWatcherPutWatchService {
	s.headers = headers
	return s
}

// Id of the watch to upsert.
func (s *XPackWatcherPutWatchService) Id(id string) *XPackWatcherPutWatchService {
	s.id = id
	return s
}

// Active specifies whether the watch is in/active by default.
func (s *XPackWatcherPutWatchService) Active(active bool) *XPackWatcherPutWatchService {
	s.active = &active
	return s
}

// MasterTimeout is an explicit operation timeout for connection to master node.
func (s *XPackWatcherPutWatchService) MasterTimeout(masterTimeout string) *XPackWatcherPutWatchService {
	s.masterTimeout = masterTimeout
	return s
}

// IfSeqNo indicates to update the watch only if the last operation that
// has changed the watch has the specified sequence number.
func (s *XPackWatcherPutWatchService) IfSeqNo(seqNo int64) *XPackWatcherPutWatchService {
	s.ifSeqNo = &seqNo
	return s
}

// IfPrimaryTerm indicates to update the watch only if the last operation that
// has changed the watch has the specified primary term.
func (s *XPackWatcherPutWatchService) IfPrimaryTerm(primaryTerm int64) *XPackWatcherPutWatchService {
	s.ifPrimaryTerm = &primaryTerm
	return s
}

// Body specifies the watch. Use a string or a type that will get serialized as JSON.
func (s *XPackWatcherPutWatchService) Body(body interface{}) *XPackWatcherPutWatchService {
	s.body = body
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackWatcherPutWatchService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_watcher/watch/{id}", map[string]string{
		"id": s.id,
	})
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
	if v := s.active; v != nil {
		params.Set("active", fmt.Sprint(*v))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if v := s.ifSeqNo; v != nil {
		params.Set("if_seq_no", fmt.Sprintf("%d", *v))
	}
	if v := s.ifPrimaryTerm; v != nil {
		params.Set("if_primary_term", fmt.Sprintf("%d", *v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackWatcherPutWatchService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.body == nil {
		invalid = append(invalid, "Body")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackWatcherPutWatchService) Do(ctx context.Context) (*XPackWatcherPutWatchResponse, error) {
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
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    s.body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(XPackWatcherPutWatchResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// XPackWatcherPutWatchResponse is the response of XPackWatcherPutWatchService.Do.
type XPackWatcherPutWatchResponse struct {
}
