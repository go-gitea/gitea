// Copyright 2012-present Oliver Eilhard. All rights reserved.
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

// GetService allows to get a typed JSON document from the index based
// on its id.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-get.html
// for details.
type GetService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index                         string
	typ                           string
	id                            string
	routing                       string
	preference                    string
	storedFields                  []string
	refresh                       string
	realtime                      *bool
	fsc                           *FetchSourceContext
	version                       interface{}
	versionType                   string
	parent                        string
	ignoreErrorsOnGeneratedFields *bool
}

// NewGetService creates a new GetService.
func NewGetService(client *Client) *GetService {
	return &GetService{
		client: client,
		typ:    "_doc",
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *GetService) Pretty(pretty bool) *GetService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *GetService) Human(human bool) *GetService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *GetService) ErrorTrace(errorTrace bool) *GetService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *GetService) FilterPath(filterPath ...string) *GetService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *GetService) Header(name string, value string) *GetService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *GetService) Headers(headers http.Header) *GetService {
	s.headers = headers
	return s
}

// Index is the name of the index.
func (s *GetService) Index(index string) *GetService {
	s.index = index
	return s
}

// Type is the type of the document
//
// Deprecated: Types are in the process of being removed.
func (s *GetService) Type(typ string) *GetService {
	s.typ = typ
	return s
}

// Id is the document ID.
func (s *GetService) Id(id string) *GetService {
	s.id = id
	return s
}

// Parent is the ID of the parent document.
func (s *GetService) Parent(parent string) *GetService {
	s.parent = parent
	return s
}

// Routing is the specific routing value.
func (s *GetService) Routing(routing string) *GetService {
	s.routing = routing
	return s
}

// Preference specifies the node or shard the operation should be performed on (default: random).
func (s *GetService) Preference(preference string) *GetService {
	s.preference = preference
	return s
}

// StoredFields is a list of fields to return in the response.
func (s *GetService) StoredFields(storedFields ...string) *GetService {
	s.storedFields = append(s.storedFields, storedFields...)
	return s
}

func (s *GetService) FetchSource(fetchSource bool) *GetService {
	if s.fsc == nil {
		s.fsc = NewFetchSourceContext(fetchSource)
	} else {
		s.fsc.SetFetchSource(fetchSource)
	}
	return s
}

func (s *GetService) FetchSourceContext(fetchSourceContext *FetchSourceContext) *GetService {
	s.fsc = fetchSourceContext
	return s
}

// Refresh the shard containing the document before performing the operation.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/docs-refresh.html
// for details.
func (s *GetService) Refresh(refresh string) *GetService {
	s.refresh = refresh
	return s
}

// Realtime specifies whether to perform the operation in realtime or search mode.
func (s *GetService) Realtime(realtime bool) *GetService {
	s.realtime = &realtime
	return s
}

// VersionType is the specific version type.
func (s *GetService) VersionType(versionType string) *GetService {
	s.versionType = versionType
	return s
}

// Version is an explicit version number for concurrency control.
func (s *GetService) Version(version interface{}) *GetService {
	s.version = version
	return s
}

// IgnoreErrorsOnGeneratedFields indicates whether to ignore fields that
// are generated if the transaction log is accessed.
func (s *GetService) IgnoreErrorsOnGeneratedFields(ignore bool) *GetService {
	s.ignoreErrorsOnGeneratedFields = &ignore
	return s
}

// Validate checks if the operation is valid.
func (s *GetService) Validate() error {
	var invalid []string
	if s.id == "" {
		invalid = append(invalid, "Id")
	}
	if s.index == "" {
		invalid = append(invalid, "Index")
	}
	if s.typ == "" {
		invalid = append(invalid, "Type")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// buildURL builds the URL for the operation.
func (s *GetService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}/{type}/{id}", map[string]string{
		"id":    s.id,
		"index": s.index,
		"type":  s.typ,
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
	if s.routing != "" {
		params.Set("routing", s.routing)
	}
	if s.parent != "" {
		params.Set("parent", s.parent)
	}
	if s.preference != "" {
		params.Set("preference", s.preference)
	}
	if len(s.storedFields) > 0 {
		params.Set("stored_fields", strings.Join(s.storedFields, ","))
	}
	if s.refresh != "" {
		params.Set("refresh", s.refresh)
	}
	if s.version != nil {
		params.Set("version", fmt.Sprintf("%v", s.version))
	}
	if s.versionType != "" {
		params.Set("version_type", s.versionType)
	}
	if s.realtime != nil {
		params.Set("realtime", fmt.Sprintf("%v", *s.realtime))
	}
	if s.ignoreErrorsOnGeneratedFields != nil {
		params.Add("ignore_errors_on_generated_fields", fmt.Sprintf("%v", *s.ignoreErrorsOnGeneratedFields))
	}
	if s.fsc != nil {
		for k, values := range s.fsc.Query() {
			params.Add(k, strings.Join(values, ","))
		}
	}
	return path, params, nil
}

// Do executes the operation.
func (s *GetService) Do(ctx context.Context) (*GetResult, error) {
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
		Method:  "GET",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(GetResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// GetResult is the outcome of GetService.Do.
type GetResult struct {
	Index       string                 `json:"_index"`   // index meta field
	Type        string                 `json:"_type"`    // type meta field
	Id          string                 `json:"_id"`      // id meta field
	Uid         string                 `json:"_uid"`     // uid meta field (see MapperService.java for all meta fields)
	Routing     string                 `json:"_routing"` // routing meta field
	Parent      string                 `json:"_parent"`  // parent meta field
	Version     *int64                 `json:"_version"` // version number, when Version is set to true in SearchService
	SeqNo       *int64                 `json:"_seq_no"`
	PrimaryTerm *int64                 `json:"_primary_term"`
	Source      json.RawMessage        `json:"_source,omitempty"`
	Found       bool                   `json:"found,omitempty"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	//Error     string                 `json:"error,omitempty"` // used only in MultiGet
	// TODO double-check that MultiGet now returns details error information
	Error *ErrorDetails `json:"error,omitempty"` // only used in MultiGet
}
