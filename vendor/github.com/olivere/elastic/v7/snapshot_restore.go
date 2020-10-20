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

// SnapshotRestoreService restores a snapshot from a snapshot repository.
//
// It is documented at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.1/modules-snapshots.html#_restore.
type SnapshotRestoreService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository         string
	snapshot           string
	masterTimeout      string
	waitForCompletion  *bool
	ignoreUnavailable  *bool
	partial            *bool
	includeAliases     *bool
	includeGlobalState *bool
	bodyString         string
	renamePattern      string
	renameReplacement  string
	indices            []string
	indexSettings      map[string]interface{}
}

// NewSnapshotCreateService creates a new SnapshotRestoreService.
func NewSnapshotRestoreService(client *Client) *SnapshotRestoreService {
	return &SnapshotRestoreService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotRestoreService) Pretty(pretty bool) *SnapshotRestoreService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotRestoreService) Human(human bool) *SnapshotRestoreService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotRestoreService) ErrorTrace(errorTrace bool) *SnapshotRestoreService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotRestoreService) FilterPath(filterPath ...string) *SnapshotRestoreService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotRestoreService) Header(name string, value string) *SnapshotRestoreService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotRestoreService) Headers(headers http.Header) *SnapshotRestoreService {
	s.headers = headers
	return s
}

// Repository name.
func (s *SnapshotRestoreService) Repository(repository string) *SnapshotRestoreService {
	s.repository = repository
	return s
}

// Snapshot name.
func (s *SnapshotRestoreService) Snapshot(snapshot string) *SnapshotRestoreService {
	s.snapshot = snapshot
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *SnapshotRestoreService) MasterTimeout(masterTimeout string) *SnapshotRestoreService {
	s.masterTimeout = masterTimeout
	return s
}

// WaitForCompletion indicates whether this request should wait until the operation has
// completed before returning.
func (s *SnapshotRestoreService) WaitForCompletion(waitForCompletion bool) *SnapshotRestoreService {
	s.waitForCompletion = &waitForCompletion
	return s
}

// Indices sets the name of the indices that should be restored from the snapshot.
func (s *SnapshotRestoreService) Indices(indices ...string) *SnapshotRestoreService {
	s.indices = indices
	return s
}

// IncludeGlobalState allows the global cluster state to be restored, defaults to false.
func (s *SnapshotRestoreService) IncludeGlobalState(includeGlobalState bool) *SnapshotRestoreService {
	s.includeGlobalState = &includeGlobalState
	return s
}

// RenamePattern helps rename indices on restore using regular expressions.
func (s *SnapshotRestoreService) RenamePattern(renamePattern string) *SnapshotRestoreService {
	s.renamePattern = renamePattern
	return s
}

// RenameReplacement as RenamePattern, helps rename indices on restore using regular expressions.
func (s *SnapshotRestoreService) RenameReplacement(renameReplacement string) *SnapshotRestoreService {
	s.renameReplacement = renameReplacement
	return s
}

// Partial indicates whether to restore indices that where partially snapshoted, defaults to false.
func (s *SnapshotRestoreService) Partial(partial bool) *SnapshotRestoreService {
	s.partial = &partial
	return s
}

// BodyString allows the user to specify the body of the HTTP request manually.
func (s *SnapshotRestoreService) BodyString(body string) *SnapshotRestoreService {
	s.bodyString = body
	return s
}

// IndexSettings sets the settings to be overwritten during the restore process
func (s *SnapshotRestoreService) IndexSettings(indexSettings map[string]interface{}) *SnapshotRestoreService {
	s.indexSettings = indexSettings
	return s
}

// IncludeAliases flags whether indices should be restored with their respective aliases,
// defaults to false.
func (s *SnapshotRestoreService) IncludeAliases(includeAliases bool) *SnapshotRestoreService {
	s.includeAliases = &includeAliases
	return s
}

// IgnoreUnavailable specifies whether to ignore unavailable snapshots, defaults to false.
func (s *SnapshotRestoreService) IgnoreUnavailable(ignoreUnavailable bool) *SnapshotRestoreService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// Do executes the operation.
func (s *SnapshotRestoreService) Do(ctx context.Context) (*SnapshotRestoreResponse, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	var body interface{}
	if len(s.bodyString) > 0 {
		body = s.bodyString
	} else {
		body = s.buildBody()
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

	ret := new(SnapshotRestoreResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// Validate checks if the operation is valid.
func (s *SnapshotRestoreService) Validate() error {
	var invalid []string
	if s.repository == "" {
		invalid = append(invalid, "Repository")
	}
	if s.snapshot == "" {
		invalid = append(invalid, "Snapshot")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

func (s *SnapshotRestoreService) buildURL() (string, url.Values, error) {
	path, err := uritemplates.Expand("/_snapshot/{repository}/{snapshot}/_restore", map[string]string{
		"snapshot":   s.snapshot,
		"repository": s.repository,
	})
	if err != nil {
		return "", url.Values{}, err
	}

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
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if v := s.waitForCompletion; v != nil {
		params.Set("wait_for_completion", fmt.Sprint(*v))
	}
	if v := s.ignoreUnavailable; v != nil {
		params.Set("ignore_unavailable", fmt.Sprint(*v))
	}
	return path, params, nil
}

func (s *SnapshotRestoreService) buildBody() interface{} {
	body := map[string]interface{}{}

	if s.includeGlobalState != nil {
		body["include_global_state"] = *s.includeGlobalState
	}
	if s.partial != nil {
		body["partial"] = *s.partial
	}
	if s.includeAliases != nil {
		body["include_aliases"] = *s.includeAliases
	}
	if len(s.indices) > 0 {
		body["indices"] = strings.Join(s.indices, ",")
	}
	if len(s.renamePattern) > 0 {
		body["rename_pattern"] = s.renamePattern
	}
	if len(s.renamePattern) > 0 {
		body["rename_replacement"] = s.renameReplacement
	}
	if len(s.indexSettings) > 0 {
		body["index_settings"] = s.indexSettings
	}
	return body
}

// SnapshotRestoreResponse represents the response for SnapshotRestoreService.Do
type SnapshotRestoreResponse struct {
	// Accepted indicates whether the request was accepted by Elasticsearch.
	Accepted *bool `json:"accepted"`

	// Snapshot information.
	Snapshot *RestoreInfo `json:"snapshot"`
}

// RestoreInfo represents information about the restored snapshot.
type RestoreInfo struct {
	Snapshot string     `json:"snapshot"`
	Indices  []string   `json:"indices"`
	Shards   ShardsInfo `json:"shards"`
}
