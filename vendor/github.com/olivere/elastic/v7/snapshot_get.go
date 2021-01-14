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
	"time"

	"github.com/olivere/elastic/v7/uritemplates"
)

// SnapshotGetService lists the snapshots on a repository
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html
// for details.
type SnapshotGetService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	repository        string
	snapshot          []string
	masterTimeout     string
	ignoreUnavailable *bool
	verbose           *bool
}

// NewSnapshotGetService creates a new SnapshotGetService.
func NewSnapshotGetService(client *Client) *SnapshotGetService {
	return &SnapshotGetService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *SnapshotGetService) Pretty(pretty bool) *SnapshotGetService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *SnapshotGetService) Human(human bool) *SnapshotGetService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *SnapshotGetService) ErrorTrace(errorTrace bool) *SnapshotGetService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *SnapshotGetService) FilterPath(filterPath ...string) *SnapshotGetService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *SnapshotGetService) Header(name string, value string) *SnapshotGetService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *SnapshotGetService) Headers(headers http.Header) *SnapshotGetService {
	s.headers = headers
	return s
}

// Repository is the repository name.
func (s *SnapshotGetService) Repository(repository string) *SnapshotGetService {
	s.repository = repository
	return s
}

// Snapshot is the list of snapshot names. If not set, defaults to all snapshots.
func (s *SnapshotGetService) Snapshot(snapshots ...string) *SnapshotGetService {
	s.snapshot = append(s.snapshot, snapshots...)
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *SnapshotGetService) MasterTimeout(masterTimeout string) *SnapshotGetService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable specifies whether to ignore unavailable snapshots, defaults to false
func (s *SnapshotGetService) IgnoreUnavailable(ignoreUnavailable bool) *SnapshotGetService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// Verbose specifies whether to show verbose snapshot info or only show the basic info found in the repository index blob
func (s *SnapshotGetService) Verbose(verbose bool) *SnapshotGetService {
	s.verbose = &verbose
	return s
}

// buildURL builds the URL for the operation.
func (s *SnapshotGetService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.snapshot) > 0 {
		path, err = uritemplates.Expand("/_snapshot/{repository}/{snapshot}", map[string]string{
			"repository": s.repository,
			"snapshot":   strings.Join(s.snapshot, ","),
		})
	} else {
		path, err = uritemplates.Expand("/_snapshot/{repository}/_all", map[string]string{
			"repository": s.repository,
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
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if v := s.ignoreUnavailable; v != nil {
		params.Set("ignore_unavailable", fmt.Sprint(*v))
	}
	if v := s.verbose; v != nil {
		params.Set("verbose", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *SnapshotGetService) Validate() error {
	var invalid []string
	if s.repository == "" {
		invalid = append(invalid, "Repository")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *SnapshotGetService) Do(ctx context.Context) (*SnapshotGetResponse, error) {
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
	ret := new(SnapshotGetResponse)
	if err := json.Unmarshal(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// SnapshotGetResponse is the response of SnapshotGetService.Do.
type SnapshotGetResponse struct {
	Snapshots []*Snapshot `json:"snapshots"`
}

// Snapshot contains all information about a single snapshot
type Snapshot struct {
	Snapshot          string                 `json:"snapshot"`
	UUID              string                 `json:"uuid"`
	VersionID         int                    `json:"version_id"`
	Version           string                 `json:"version"`
	Indices           []string               `json:"indices"`
	State             string                 `json:"state"`
	Reason            string                 `json:"reason"`
	StartTime         time.Time              `json:"start_time"`
	StartTimeInMillis int64                  `json:"start_time_in_millis"`
	EndTime           time.Time              `json:"end_time"`
	EndTimeInMillis   int64                  `json:"end_time_in_millis"`
	DurationInMillis  int64                  `json:"duration_in_millis"`
	Failures          []SnapshotShardFailure `json:"failures"`
	Shards            *ShardsInfo            `json:"shards"`
}
