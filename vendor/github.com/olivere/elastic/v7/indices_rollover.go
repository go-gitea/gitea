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

// IndicesRolloverService rolls an alias over to a new index when the
// existing index is considered to be too large or too old.
//
// It is documented at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-rollover-index.html.
type IndicesRolloverService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	dryRun              bool
	newIndex            string
	alias               string
	masterTimeout       string
	timeout             string
	waitForActiveShards string
	conditions          map[string]interface{}
	settings            map[string]interface{}
	mappings            map[string]interface{}
	bodyJson            interface{}
	bodyString          string
}

// NewIndicesRolloverService creates a new IndicesRolloverService.
func NewIndicesRolloverService(client *Client) *IndicesRolloverService {
	return &IndicesRolloverService{
		client:     client,
		conditions: make(map[string]interface{}),
		settings:   make(map[string]interface{}),
		mappings:   make(map[string]interface{}),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesRolloverService) Pretty(pretty bool) *IndicesRolloverService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesRolloverService) Human(human bool) *IndicesRolloverService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesRolloverService) ErrorTrace(errorTrace bool) *IndicesRolloverService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesRolloverService) FilterPath(filterPath ...string) *IndicesRolloverService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesRolloverService) Header(name string, value string) *IndicesRolloverService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesRolloverService) Headers(headers http.Header) *IndicesRolloverService {
	s.headers = headers
	return s
}

// Alias is the name of the alias to rollover.
func (s *IndicesRolloverService) Alias(alias string) *IndicesRolloverService {
	s.alias = alias
	return s
}

// NewIndex is the name of the rollover index.
func (s *IndicesRolloverService) NewIndex(newIndex string) *IndicesRolloverService {
	s.newIndex = newIndex
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesRolloverService) MasterTimeout(masterTimeout string) *IndicesRolloverService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout sets an explicit operation timeout.
func (s *IndicesRolloverService) Timeout(timeout string) *IndicesRolloverService {
	s.timeout = timeout
	return s
}

// WaitForActiveShards sets the number of active shards to wait for on the
// newly created rollover index before the operation returns.
func (s *IndicesRolloverService) WaitForActiveShards(waitForActiveShards string) *IndicesRolloverService {
	s.waitForActiveShards = waitForActiveShards
	return s
}

// DryRun, when set, specifies that only conditions are checked without
// performing the actual rollover.
func (s *IndicesRolloverService) DryRun(dryRun bool) *IndicesRolloverService {
	s.dryRun = dryRun
	return s
}

// Conditions allows to specify all conditions as a dictionary.
func (s *IndicesRolloverService) Conditions(conditions map[string]interface{}) *IndicesRolloverService {
	s.conditions = conditions
	return s
}

// AddCondition adds a condition to the rollover decision.
func (s *IndicesRolloverService) AddCondition(name string, value interface{}) *IndicesRolloverService {
	s.conditions[name] = value
	return s
}

// AddMaxIndexAgeCondition adds a condition to set the max index age.
func (s *IndicesRolloverService) AddMaxIndexAgeCondition(time string) *IndicesRolloverService {
	s.conditions["max_age"] = time
	return s
}

// AddMaxIndexDocsCondition adds a condition to set the max documents in the index.
func (s *IndicesRolloverService) AddMaxIndexDocsCondition(docs int64) *IndicesRolloverService {
	s.conditions["max_docs"] = docs
	return s
}

// Settings adds the index settings.
func (s *IndicesRolloverService) Settings(settings map[string]interface{}) *IndicesRolloverService {
	s.settings = settings
	return s
}

// AddSetting adds an index setting.
func (s *IndicesRolloverService) AddSetting(name string, value interface{}) *IndicesRolloverService {
	s.settings[name] = value
	return s
}

// Mappings adds the index mappings.
func (s *IndicesRolloverService) Mappings(mappings map[string]interface{}) *IndicesRolloverService {
	s.mappings = mappings
	return s
}

// AddMapping adds a mapping for the given type.
func (s *IndicesRolloverService) AddMapping(typ string, mapping interface{}) *IndicesRolloverService {
	s.mappings[typ] = mapping
	return s
}

// BodyJson sets the conditions that needs to be met for executing rollover,
// specified as a serializable JSON instance which is sent as the body of
// the request.
func (s *IndicesRolloverService) BodyJson(body interface{}) *IndicesRolloverService {
	s.bodyJson = body
	return s
}

// BodyString sets the conditions that needs to be met for executing rollover,
// specified as a string which is sent as the body of the request.
func (s *IndicesRolloverService) BodyString(body string) *IndicesRolloverService {
	s.bodyString = body
	return s
}

// getBody returns the body of the request, if not explicitly set via
// BodyJson or BodyString.
func (s *IndicesRolloverService) getBody() interface{} {
	body := make(map[string]interface{})
	if len(s.conditions) > 0 {
		body["conditions"] = s.conditions
	}
	if len(s.settings) > 0 {
		body["settings"] = s.settings
	}
	if len(s.mappings) > 0 {
		body["mappings"] = s.mappings
	}
	return body
}

// buildURL builds the URL for the operation.
func (s *IndicesRolloverService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if s.newIndex != "" {
		path, err = uritemplates.Expand("/{alias}/_rollover/{new_index}", map[string]string{
			"alias":     s.alias,
			"new_index": s.newIndex,
		})
	} else {
		path, err = uritemplates.Expand("/{alias}/_rollover", map[string]string{
			"alias": s.alias,
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
	if s.dryRun {
		params.Set("dry_run", "true")
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.waitForActiveShards != "" {
		params.Set("wait_for_active_shards", s.waitForActiveShards)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesRolloverService) Validate() error {
	var invalid []string
	if s.alias == "" {
		invalid = append(invalid, "Alias")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesRolloverService) Do(ctx context.Context) (*IndicesRolloverResponse, error) {
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
	} else if s.bodyString != "" {
		body = s.bodyString
	} else {
		body = s.getBody()
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
	ret := new(IndicesRolloverResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesRolloverResponse is the response of IndicesRolloverService.Do.
type IndicesRolloverResponse struct {
	OldIndex           string          `json:"old_index"`
	NewIndex           string          `json:"new_index"`
	RolledOver         bool            `json:"rolled_over"`
	DryRun             bool            `json:"dry_run"`
	Acknowledged       bool            `json:"acknowledged"`
	ShardsAcknowledged bool            `json:"shards_acknowledged"`
	Conditions         map[string]bool `json:"conditions"`
}
