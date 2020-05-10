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
)

// -- Actions --

// AliasAction is an action to apply to an alias, e.g. "add" or "remove".
type AliasAction interface {
	Source() (interface{}, error)
}

// AliasAddAction is an action to add to an alias.
type AliasAddAction struct {
	index         []string // index name(s)
	alias         string   // alias name
	filter        Query
	routing       string
	searchRouting string
	indexRouting  string
	isWriteIndex  *bool
}

// NewAliasAddAction returns an action to add an alias.
func NewAliasAddAction(alias string) *AliasAddAction {
	return &AliasAddAction{
		alias: alias,
	}
}

// Index associates one or more indices to the alias.
func (a *AliasAddAction) Index(index ...string) *AliasAddAction {
	a.index = append(a.index, index...)
	return a
}

func (a *AliasAddAction) removeBlankIndexNames() {
	var indices []string
	for _, index := range a.index {
		if len(index) > 0 {
			indices = append(indices, index)
		}
	}
	a.index = indices
}

// Filter associates a filter to the alias.
func (a *AliasAddAction) Filter(filter Query) *AliasAddAction {
	a.filter = filter
	return a
}

// Routing associates a routing value to the alias.
// This basically sets index and search routing to the same value.
func (a *AliasAddAction) Routing(routing string) *AliasAddAction {
	a.routing = routing
	return a
}

// IndexRouting associates an index routing value to the alias.
func (a *AliasAddAction) IndexRouting(routing string) *AliasAddAction {
	a.indexRouting = routing
	return a
}

// SearchRouting associates a search routing value to the alias.
func (a *AliasAddAction) SearchRouting(routing ...string) *AliasAddAction {
	a.searchRouting = strings.Join(routing, ",")
	return a
}

// IsWriteIndex associates an is_write_index flag to the alias.
func (a *AliasAddAction) IsWriteIndex(flag bool) *AliasAddAction {
	a.isWriteIndex = &flag
	return a
}

// Validate checks if the operation is valid.
func (a *AliasAddAction) Validate() error {
	var invalid []string
	if len(a.alias) == 0 {
		invalid = append(invalid, "Alias")
	}
	if len(a.index) == 0 {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	if a.isWriteIndex != nil && len(a.index) > 1 {
		return fmt.Errorf("more than 1 target index specified in operation with 'is_write_index' flag present")
	}
	return nil
}

// Source returns the JSON-serializable data.
func (a *AliasAddAction) Source() (interface{}, error) {
	a.removeBlankIndexNames()
	if err := a.Validate(); err != nil {
		return nil, err
	}
	src := make(map[string]interface{})
	act := make(map[string]interface{})
	src["add"] = act
	act["alias"] = a.alias
	switch len(a.index) {
	case 1:
		act["index"] = a.index[0]
	default:
		act["indices"] = a.index
	}
	if a.filter != nil {
		f, err := a.filter.Source()
		if err != nil {
			return nil, err
		}
		act["filter"] = f
	}
	if len(a.routing) > 0 {
		act["routing"] = a.routing
	}
	if len(a.indexRouting) > 0 {
		act["index_routing"] = a.indexRouting
	}
	if len(a.searchRouting) > 0 {
		act["search_routing"] = a.searchRouting
	}
	if a.isWriteIndex != nil {
		act["is_write_index"] = *a.isWriteIndex
	}
	return src, nil
}

// AliasRemoveAction is an action to remove an alias.
type AliasRemoveAction struct {
	index []string // index name(s)
	alias string   // alias name
}

// NewAliasRemoveAction returns an action to remove an alias.
func NewAliasRemoveAction(alias string) *AliasRemoveAction {
	return &AliasRemoveAction{
		alias: alias,
	}
}

// Index associates one or more indices to the alias.
func (a *AliasRemoveAction) Index(index ...string) *AliasRemoveAction {
	a.index = append(a.index, index...)
	return a
}

func (a *AliasRemoveAction) removeBlankIndexNames() {
	var indices []string
	for _, index := range a.index {
		if len(index) > 0 {
			indices = append(indices, index)
		}
	}
	a.index = indices
}

// Validate checks if the operation is valid.
func (a *AliasRemoveAction) Validate() error {
	var invalid []string
	if len(a.alias) == 0 {
		invalid = append(invalid, "Alias")
	}
	if len(a.index) == 0 {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Source returns the JSON-serializable data.
func (a *AliasRemoveAction) Source() (interface{}, error) {
	a.removeBlankIndexNames()
	if err := a.Validate(); err != nil {
		return nil, err
	}
	src := make(map[string]interface{})
	act := make(map[string]interface{})
	src["remove"] = act
	act["alias"] = a.alias
	switch len(a.index) {
	case 1:
		act["index"] = a.index[0]
	default:
		act["indices"] = a.index
	}
	return src, nil
}

// AliasRemoveIndexAction is an action to remove an index during an alias
// operation.
type AliasRemoveIndexAction struct {
	index string // index name
}

// NewAliasRemoveIndexAction returns an action to remove an index.
func NewAliasRemoveIndexAction(index string) *AliasRemoveIndexAction {
	return &AliasRemoveIndexAction{
		index: index,
	}
}

// Validate checks if the operation is valid.
func (a *AliasRemoveIndexAction) Validate() error {
	if a.index == "" {
		return fmt.Errorf("missing required field: index")
	}
	return nil
}

// Source returns the JSON-serializable data.
func (a *AliasRemoveIndexAction) Source() (interface{}, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	src := make(map[string]interface{})
	act := make(map[string]interface{})
	src["remove_index"] = act
	act["index"] = a.index
	return src, nil
}

// -- Service --

// AliasService enables users to add or remove an alias.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-aliases.html
// for details.
type AliasService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	actions []AliasAction
}

// NewAliasService implements a service to manage aliases.
func NewAliasService(client *Client) *AliasService {
	builder := &AliasService{
		client: client,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *AliasService) Pretty(pretty bool) *AliasService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *AliasService) Human(human bool) *AliasService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *AliasService) ErrorTrace(errorTrace bool) *AliasService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *AliasService) FilterPath(filterPath ...string) *AliasService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *AliasService) Header(name string, value string) *AliasService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *AliasService) Headers(headers http.Header) *AliasService {
	s.headers = headers
	return s
}

// Add adds an alias to an index.
func (s *AliasService) Add(indexName string, aliasName string) *AliasService {
	action := NewAliasAddAction(aliasName).Index(indexName)
	s.actions = append(s.actions, action)
	return s
}

// Add adds an alias to an index and associates a filter to the alias.
func (s *AliasService) AddWithFilter(indexName string, aliasName string, filter Query) *AliasService {
	action := NewAliasAddAction(aliasName).Index(indexName).Filter(filter)
	s.actions = append(s.actions, action)
	return s
}

// Remove removes an alias.
func (s *AliasService) Remove(indexName string, aliasName string) *AliasService {
	action := NewAliasRemoveAction(aliasName).Index(indexName)
	s.actions = append(s.actions, action)
	return s
}

// Action accepts one or more AliasAction instances which can be
// of type AliasAddAction or AliasRemoveAction.
func (s *AliasService) Action(action ...AliasAction) *AliasService {
	s.actions = append(s.actions, action...)
	return s
}

// buildURL builds the URL for the operation.
func (s *AliasService) buildURL() (string, url.Values, error) {
	path := "/_aliases"

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
	return path, params, nil
}

// Do executes the command.
func (s *AliasService) Do(ctx context.Context) (*AliasResult, error) {
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Body with actions
	body := make(map[string]interface{})
	var actions []interface{}
	for _, action := range s.actions {
		src, err := action.Source()
		if err != nil {
			return nil, err
		}
		actions = append(actions, src)
	}
	body["actions"] = actions

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

	// Return results
	ret := new(AliasResult)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of an alias request.

// AliasResult is the outcome of calling Do on AliasService.
type AliasResult struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
