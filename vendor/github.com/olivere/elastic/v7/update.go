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

// UpdateService updates a document in Elasticsearch.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-update.html
// for details.
type UpdateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index               string
	typ                 string
	id                  string
	routing             string
	parent              string
	script              *Script
	fields              []string
	fsc                 *FetchSourceContext
	version             *int64
	versionType         string
	retryOnConflict     *int
	refresh             string
	waitForActiveShards string
	upsert              interface{}
	scriptedUpsert      *bool
	docAsUpsert         *bool
	detectNoop          *bool
	doc                 interface{}
	timeout             string
	ifSeqNo             *int64
	ifPrimaryTerm       *int64
}

// NewUpdateService creates the service to update documents in Elasticsearch.
func NewUpdateService(client *Client) *UpdateService {
	return &UpdateService{
		client: client,
		typ:    "_doc",
		fields: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *UpdateService) Pretty(pretty bool) *UpdateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *UpdateService) Human(human bool) *UpdateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *UpdateService) ErrorTrace(errorTrace bool) *UpdateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *UpdateService) FilterPath(filterPath ...string) *UpdateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *UpdateService) Header(name string, value string) *UpdateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *UpdateService) Headers(headers http.Header) *UpdateService {
	s.headers = headers
	return s
}

// Index is the name of the Elasticsearch index (required).
func (s *UpdateService) Index(name string) *UpdateService {
	s.index = name
	return s
}

// Type is the type of the document.
//
// Deprecated: Types are in the process of being removed.
func (s *UpdateService) Type(typ string) *UpdateService {
	s.typ = typ
	return s
}

// Id is the identifier of the document to update (required).
func (s *UpdateService) Id(id string) *UpdateService {
	s.id = id
	return s
}

// Routing specifies a specific routing value.
func (s *UpdateService) Routing(routing string) *UpdateService {
	s.routing = routing
	return s
}

// Parent sets the id of the parent document.
func (s *UpdateService) Parent(parent string) *UpdateService {
	s.parent = parent
	return s
}

// Script is the script definition.
func (s *UpdateService) Script(script *Script) *UpdateService {
	s.script = script
	return s
}

// RetryOnConflict specifies how many times the operation should be retried
// when a conflict occurs (default: 0).
func (s *UpdateService) RetryOnConflict(retryOnConflict int) *UpdateService {
	s.retryOnConflict = &retryOnConflict
	return s
}

// Fields is a list of fields to return in the response.
func (s *UpdateService) Fields(fields ...string) *UpdateService {
	s.fields = make([]string, 0, len(fields))
	s.fields = append(s.fields, fields...)
	return s
}

// Version defines the explicit version number for concurrency control.
func (s *UpdateService) Version(version int64) *UpdateService {
	s.version = &version
	return s
}

// VersionType is e.g. "internal".
func (s *UpdateService) VersionType(versionType string) *UpdateService {
	s.versionType = versionType
	return s
}

// Refresh the index after performing the update.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *UpdateService) Refresh(refresh string) *UpdateService {
	s.refresh = refresh
	return s
}

// WaitForActiveShards sets the number of shard copies that must be active before
// proceeding with the update operation. Defaults to 1, meaning the primary shard only.
// Set to `all` for all shard copies, otherwise set to any non-negative value less than
// or equal to the total number of copies for the shard (number of replicas + 1).
func (s *UpdateService) WaitForActiveShards(waitForActiveShards string) *UpdateService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// Doc allows for updating a partial document.
func (s *UpdateService) Doc(doc interface{}) *UpdateService {
	s.doc = doc
	return s
}

// Upsert can be used to index the document when it doesn't exist yet.
// Use this e.g. to initialize a document with a default value.
func (s *UpdateService) Upsert(doc interface{}) *UpdateService {
	s.upsert = doc
	return s
}

// DocAsUpsert can be used to insert the document if it doesn't already exist.
func (s *UpdateService) DocAsUpsert(docAsUpsert bool) *UpdateService {
	s.docAsUpsert = &docAsUpsert
	return s
}

// DetectNoop will instruct Elasticsearch to check if changes will occur
// when updating via Doc. It there aren't any changes, the request will
// turn into a no-op.
func (s *UpdateService) DetectNoop(detectNoop bool) *UpdateService {
	s.detectNoop = &detectNoop
	return s
}

// ScriptedUpsert should be set to true if the referenced script
// (defined in Script or ScriptId) should be called to perform an insert.
// The default is false.
func (s *UpdateService) ScriptedUpsert(scriptedUpsert bool) *UpdateService {
	s.scriptedUpsert = &scriptedUpsert
	return s
}

// Timeout is an explicit timeout for the operation, e.g. "1000", "1s" or "500ms".
func (s *UpdateService) Timeout(timeout string) *UpdateService {
	s.timeout = timeout
	return s
}

// IfSeqNo indicates to only perform the update operation if the last
// operation that has changed the document has the specified sequence number.
func (s *UpdateService) IfSeqNo(seqNo int64) *UpdateService {
	s.ifSeqNo = &seqNo
	return s
}

// IfPrimaryTerm indicates to only perform the update operation if the
// last operation that has changed the document has the specified primary term.
func (s *UpdateService) IfPrimaryTerm(primaryTerm int64) *UpdateService {
	s.ifPrimaryTerm = &primaryTerm
	return s
}

// FetchSource asks Elasticsearch to return the updated _source in the response.
func (s *UpdateService) FetchSource(fetchSource bool) *UpdateService {
	if s.fsc == nil {
		s.fsc = NewFetchSourceContext(fetchSource)
	} else {
		s.fsc.SetFetchSource(fetchSource)
	}
	return s
}

// FetchSourceContext indicates that _source should be returned in the response,
// allowing wildcard patterns to be defined via FetchSourceContext.
func (s *UpdateService) FetchSourceContext(fetchSourceContext *FetchSourceContext) *UpdateService {
	s.fsc = fetchSourceContext
	return s
}

// url returns the URL part of the document request.
func (s *UpdateService) url() (string, url.Values, error) {
	// Build url
	var path string
	var err error
	if s.typ == "" || s.typ == "_doc" {
		path, err = uritemplates.Expand("/{index}/_update/{id}", map[string]string{
			"index": s.index,
			"id":    s.id,
		})
	} else {
		path, err = uritemplates.Expand("/{index}/{type}/{id}/_update", map[string]string{
			"index": s.index,
			"type":  s.typ,
			"id":    s.id,
		})
	}
	if err != nil {
		return "", url.Values{}, err
	}

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
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if s.parent != "" {
		params.Set("parent", s.parent)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}
	if len(s.fields) > 0 {
		params.Set("fields", strings.Join(s.fields, ","))
	}
	if s.version != nil {
		params.Set("version", fmt.Sprintf("%d", *s.version))
	}
	if s.versionType != "" {
		params.Set("version_type", s.versionType)
	}
	if s.retryOnConflict != nil {
		params.Set("retry_on_conflict", fmt.Sprintf("%v", *s.retryOnConflict))
	}
	if v := s.ifSeqNo; v != nil {
		params.Set("if_seq_no", fmt.Sprintf("%d", *v))
	}
	if v := s.ifPrimaryTerm; v != nil {
		params.Set("if_primary_term", fmt.Sprintf("%d", *v))
	}
	return path, params, nil
}

// body returns the body part of the document request.
func (s *UpdateService) body() (interface{}, error) {
	source := make(map[string]interface{})

	if s.script != nil {
		src, err := s.script.Source()
		if err != nil {
			return nil, err
		}
		source["script"] = src
	}

	if v := s.scriptedUpsert; v != nil {
		source["scripted_upsert"] = *v
	}

	if s.upsert != nil {
		source["upsert"] = s.upsert
	}

	if s.doc != nil {
		source["doc"] = s.doc
	}
	if v := s.docAsUpsert; v != nil {
		source["doc_as_upsert"] = *v
	}
	if v := s.detectNoop; v != nil {
		source["detect_noop"] = *v
	}
	if s.fsc != nil {
		src, err := s.fsc.Source()
		if err != nil {
			return nil, err
		}
		source["_source"] = src
	}

	return source, nil
}

// Do executes the update operation.
func (s *UpdateService) Do(ctx context.Context) (*UpdateResponse, error) {
	path, params, err := s.url()
	if err != nil {
		return nil, err
	}

	// Get body of the request
	body, err := s.body()
	if err != nil {
		return nil, err
	}

	// Get response
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

	// Return result
	ret := new(UpdateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// UpdateResponse is the result of updating a document in Elasticsearch.
type UpdateResponse struct {
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
	GetResult     *GetResult  `json:"get,omitempty"`
}
