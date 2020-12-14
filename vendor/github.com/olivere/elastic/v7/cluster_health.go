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

// ClusterHealthService allows to get a very simple status on the health of the cluster.
//
// See http://www.elastic.co/guide/en/elasticsearch/reference/7.0/cluster-health.html
// for details.
type ClusterHealthService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	indices                   []string
	level                     string
	local                     *bool
	masterTimeout             string
	timeout                   string
	waitForActiveShards       *int
	waitForNodes              string
	waitForNoRelocatingShards *bool
	waitForStatus             string
}

// NewClusterHealthService creates a new ClusterHealthService.
func NewClusterHealthService(client *Client) *ClusterHealthService {
	return &ClusterHealthService{
		client:  client,
		indices: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ClusterHealthService) Pretty(pretty bool) *ClusterHealthService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ClusterHealthService) Human(human bool) *ClusterHealthService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ClusterHealthService) ErrorTrace(errorTrace bool) *ClusterHealthService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ClusterHealthService) FilterPath(filterPath ...string) *ClusterHealthService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ClusterHealthService) Header(name string, value string) *ClusterHealthService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ClusterHealthService) Headers(headers http.Header) *ClusterHealthService {
	s.headers = headers
	return s
}

// Index limits the information returned to specific indices.
func (s *ClusterHealthService) Index(indices ...string) *ClusterHealthService {
	s.indices = append(s.indices, indices...)
	return s
}

// Level specifies the level of detail for returned information.
func (s *ClusterHealthService) Level(level string) *ClusterHealthService {
	s.level = level
	return s
}

// Local indicates whether to return local information. If it is true,
// we do not retrieve the state from master node (default: false).
func (s *ClusterHealthService) Local(local bool) *ClusterHealthService {
	s.local = &local
	return s
}

// MasterTimeout specifies an explicit operation timeout for connection to master node.
func (s *ClusterHealthService) MasterTimeout(masterTimeout string) *ClusterHealthService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout specifies an explicit operation timeout.
func (s *ClusterHealthService) Timeout(timeout string) *ClusterHealthService {
	s.timeout = timeout
	return s
}

// WaitForActiveShards can be used to wait until the specified number of shards are active.
func (s *ClusterHealthService) WaitForActiveShards(waitForActiveShards int) *ClusterHealthService {
	s.waitForActiveShards = &waitForActiveShards
	return s
}

// WaitForNodes can be used to wait until the specified number of nodes are available.
// Example: "12" to wait for exact values, ">12" and "<12" for ranges.
func (s *ClusterHealthService) WaitForNodes(waitForNodes string) *ClusterHealthService {
	s.waitForNodes = waitForNodes
	return s
}

// WaitForNoRelocatingShards can be used to wait until all shard relocations are finished.
func (s *ClusterHealthService) WaitForNoRelocatingShards(waitForNoRelocatingShards bool) *ClusterHealthService {
	s.waitForNoRelocatingShards = &waitForNoRelocatingShards
	return s
}

// WaitForStatus can be used to wait until the cluster is in a specific state.
// Valid values are: green, yellow, or red.
func (s *ClusterHealthService) WaitForStatus(waitForStatus string) *ClusterHealthService {
	s.waitForStatus = waitForStatus
	return s
}

// WaitForGreenStatus will wait for the "green" state.
func (s *ClusterHealthService) WaitForGreenStatus() *ClusterHealthService {
	return s.WaitForStatus("green")
}

// WaitForYellowStatus will wait for the "yellow" state.
func (s *ClusterHealthService) WaitForYellowStatus() *ClusterHealthService {
	return s.WaitForStatus("yellow")
}

// buildURL builds the URL for the operation.
func (s *ClusterHealthService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.indices) > 0 {
		path, err = uritemplates.Expand("/_cluster/health/{index}", map[string]string{
			"index": strings.Join(s.indices, ","),
		})
	} else {
		path = "/_cluster/health"
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
	if s.level != "" {
		params.Set("level", s.level)
	}
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.waitForActiveShards != nil {
		params.Set("wait_for_active_shards", fmt.Sprintf("%v", s.waitForActiveShards))
	}
	if s.waitForNodes != "" {
		params.Set("wait_for_nodes", s.waitForNodes)
	}
	if s.waitForNoRelocatingShards != nil {
		params.Set("wait_for_no_relocating_shards", fmt.Sprintf("%v", *s.waitForNoRelocatingShards))
	}
	if s.waitForStatus != "" {
		params.Set("wait_for_status", s.waitForStatus)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ClusterHealthService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *ClusterHealthService) Do(ctx context.Context) (*ClusterHealthResponse, error) {
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
	ret := new(ClusterHealthResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ClusterHealthResponse is the response of ClusterHealthService.Do.
type ClusterHealthResponse struct {
	ClusterName                    string  `json:"cluster_name"`
	Status                         string  `json:"status"`
	TimedOut                       bool    `json:"timed_out"`
	NumberOfNodes                  int     `json:"number_of_nodes"`
	NumberOfDataNodes              int     `json:"number_of_data_nodes"`
	ActivePrimaryShards            int     `json:"active_primary_shards"`
	ActiveShards                   int     `json:"active_shards"`
	RelocatingShards               int     `json:"relocating_shards"`
	InitializingShards             int     `json:"initializing_shards"`
	UnassignedShards               int     `json:"unassigned_shards"`
	DelayedUnassignedShards        int     `json:"delayed_unassigned_shards"`
	NumberOfPendingTasks           int     `json:"number_of_pending_tasks"`
	NumberOfInFlightFetch          int     `json:"number_of_in_flight_fetch"`
	TaskMaxWaitTimeInQueue         string  `json:"task_max_waiting_in_queue"`        // "0s"
	TaskMaxWaitTimeInQueueInMillis int     `json:"task_max_waiting_in_queue_millis"` // 0
	ActiveShardsPercent            string  `json:"active_shards_percent"`            // "100.0%"
	ActiveShardsPercentAsNumber    float64 `json:"active_shards_percent_as_number"`  // 100.0

	// Index name -> index health
	Indices map[string]*ClusterIndexHealth `json:"indices"`
}

// ClusterIndexHealth will be returned as part of ClusterHealthResponse.
type ClusterIndexHealth struct {
	Status              string `json:"status"`
	NumberOfShards      int    `json:"number_of_shards"`
	NumberOfReplicas    int    `json:"number_of_replicas"`
	ActivePrimaryShards int    `json:"active_primary_shards"`
	ActiveShards        int    `json:"active_shards"`
	RelocatingShards    int    `json:"relocating_shards"`
	InitializingShards  int    `json:"initializing_shards"`
	UnassignedShards    int    `json:"unassigned_shards"`
	// Shards by id, e.g. "0" or "1"
	Shards map[string]*ClusterShardHealth `json:"shards"`
}

// ClusterShardHealth will be returned as part of ClusterHealthResponse.
type ClusterShardHealth struct {
	Status             string `json:"status"`
	PrimaryActive      bool   `json:"primary_active"`
	ActiveShards       int    `json:"active_shards"`
	RelocatingShards   int    `json:"relocating_shards"`
	InitializingShards int    `json:"initializing_shards"`
	UnassignedShards   int    `json:"unassigned_shards"`
}
