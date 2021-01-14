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

// DeleteService allows to delete a typed JSON document from a specified
// index based on its id.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-delete.html
// for details.
type DeleteService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	id                  string
	index               string
	typ                 string
	routing             string
	timeout             string
	version             interface{}
	versionType         string
	waitForActiveShards string
	parent              string
	refresh             string
	ifSeqNo             *int64
	ifPrimaryTerm       *int64
}

// NewDeleteService creates a new DeleteService.
func NewDeleteService(client *Client) *DeleteService {
	return &DeleteService{
		client: client,
		typ:    "_doc",
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *DeleteService) Pretty(pretty bool) *DeleteService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *DeleteService) Human(human bool) *DeleteService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *DeleteService) ErrorTrace(errorTrace bool) *DeleteService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *DeleteService) FilterPath(filterPath ...string) *DeleteService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *DeleteService) Header(name string, value string) *DeleteService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *DeleteService) Headers(headers http.Header) *DeleteService {
	s.headers = headers
	return s
}

// Type is the type of the document.
//
// Deprecated: Types are in the process of being removed.
func (s *DeleteService) Type(typ string) *DeleteService {
	s.typ = typ
	return s
}

// Id is the document ID.
func (s *DeleteService) Id(id string) *DeleteService {
	s.id = id
	return s
}

// Index is the name of the index.
func (s *DeleteService) Index(index string) *DeleteService {
	s.index = index
	return s
}

// Routing is a specific routing value.
func (s *DeleteService) Routing(routing string) *DeleteService {
	s.routing = routing
	return s
}

// Timeout is an explicit operation timeout.
func (s *DeleteService) Timeout(timeout string) *DeleteService {
	s.timeout = timeout
	return s
}

// Version is an explicit version number for concurrency control.
func (s *DeleteService) Version(version interface{}) *DeleteService {
	s.version = version
	return s
}

// VersionType is a specific version type.
func (s *DeleteService) VersionType(versionType string) *DeleteService {
	s.versionType = versionType
	return s
}

// WaitForActiveShards sets the number of shard copies that must be active
// before proceeding with the delete operation. Defaults to 1, meaning the
// primary shard only. Set to `all` for all shard copies, otherwise set to
// any non-negative value less than or equal to the total number of copies
// for the shard (number of replicas + 1).
func (s *DeleteService) WaitForActiveShards(waitForActiveShards string) *DeleteService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// Parent is the ID of parent document.
func (s *DeleteService) Parent(parent string) *DeleteService {
	s.parent = parent
	return s
}

// Refresh the index after performing the operation.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *DeleteService) Refresh(refresh string) *DeleteService {
	s.refresh = refresh
	return s
}

// IfSeqNo indicates to only perform the delete operation if the last
// operation that has changed the document has the specified sequence number.
func (s *DeleteService) IfSeqNo(seqNo int64) *DeleteService {
	s.ifSeqNo = &seqNo
	return s
}

// IfPrimaryTerm indicates to only perform the delete operation if the
// last operation that has changed the document has the specified primary term.
func (s *DeleteService) IfPrimaryTerm(primaryTerm int64) *DeleteService {
	s.ifPrimaryTerm = &primaryTerm
	return s
}

// buildURL builds the URL for the operation.
func (s *DeleteService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/{type}/{id}", map[string]string{
		"index": s.index,
		"type":  s.typ,
		"id":    s.id,
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
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if v := s.version; v != nil {
		params.Set("version", fmt.Sprint(v))
	}
	if s.versionType != "" {
		params.Set("version_type", s.versionType)
	}
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}
	if s.parent != "" {
		params.Set("parent", s.parent)
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
func (s *DeleteService) Validate() error {
	var invalid []string
	if s.typ == "" {
		invalid = append(invalid, "Type")
	}
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.index == "" {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation. If the document is not found (404), Elasticsearch will
// still return a response. This response is serialized and returned as well. In other
// words, for HTTP status code 404, both an error and a response might be returned.
func (s *DeleteService) Do(ctx context.Context) (*DeleteResponse, error) {
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
		Method:       "DELETE",
		Path:         path,
		Params:       params,
		IgnoreErrors: []int{http.StatusNotFound},
		Headers:      s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(DeleteResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}

	// If we have a 404, we return both a result and an error, just like ES does
	if res.StatusCode == http.StatusNotFound {
		return ret, &Error{Status: http.StatusNotFound}
	}

	return ret, nil
}

// -- Result of a delete request.

// DeleteResponse is the outcome of running DeleteService.Do.
type DeleteResponse struct {
	Index         string      `json:"_index,omitempty"`
	Type          string      `json:"_type,omitempty"`
	Id            string      `json:"_id,omitempty"`
	Version       int64       `json:"_version,omitempty"`
	Result        string      `json:"result,omitempty"`
	Shards        *ShardsInfo `json:"_shards,omitempty"`
	SeqNo         int64       `json:"_seq_no,omitempty"`
	PrimaryTerm   int64       `json:"_primary_term,omitempty"`
	Status        int         `json:"status,omitempty"`
	ForcedRefresh bool        `json:"forced_refresh,omitempty"`
}
