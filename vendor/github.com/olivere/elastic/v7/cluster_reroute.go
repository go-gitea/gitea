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
)

// ClusterRerouteService allows for manual changes to the allocation of
// individual shards in the cluster. For example, a shard can be moved from
// one node to another explicitly, an allocation can be cancelled, and
// an unassigned shard can be explicitly allocated to a specific node.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cluster-reroute.html
// for details.
type ClusterRerouteService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	metrics       []string
	dryRun        *bool
	explain       *bool
	retryFailed   *bool
	masterTimeout string
	timeout       string
	commands      []AllocationCommand
	body          interface{}
}

// NewClusterRerouteService creates a new ClusterRerouteService.
func NewClusterRerouteService(client *Client) *ClusterRerouteService {
	return &ClusterRerouteService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ClusterRerouteService) Pretty(pretty bool) *ClusterRerouteService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ClusterRerouteService) Human(human bool) *ClusterRerouteService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ClusterRerouteService) ErrorTrace(errorTrace bool) *ClusterRerouteService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ClusterRerouteService) FilterPath(filterPath ...string) *ClusterRerouteService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ClusterRerouteService) Header(name string, value string) *ClusterRerouteService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ClusterRerouteService) Headers(headers http.Header) *ClusterRerouteService {
	s.headers = headers
	return s
}

// Metric limits the information returned to the specified metric.
// It can be one of: "_all", "blocks", "metadata", "nodes", "routing_table", "master_node", "version".
// Defaults to all but metadata.
func (s *ClusterRerouteService) Metric(metrics ...string) *ClusterRerouteService {
	s.metrics = append(s.metrics, metrics...)
	return s
}

// DryRun indicates whether to simulate the operation only and return the
// resulting state.
func (s *ClusterRerouteService) DryRun(dryRun bool) *ClusterRerouteService {
	s.dryRun = &dryRun
	return s
}

// Explain, when set to true, returns an explanation of why the commands
// can or cannot be executed.
func (s *ClusterRerouteService) Explain(explain bool) *ClusterRerouteService {
	s.explain = &explain
	return s
}

// RetryFailed indicates whether to retry allocation of shards that are blocked
// due to too many subsequent allocation failures.
func (s *ClusterRerouteService) RetryFailed(retryFailed bool) *ClusterRerouteService {
	s.retryFailed = &retryFailed
	return s
}

// MasterTimeout specifies an explicit timeout for connection to master.
func (s *ClusterRerouteService) MasterTimeout(masterTimeout string) *ClusterRerouteService {
	s.masterTimeout = masterTimeout
	return s
}

// Timeout specifies an explicit operationtimeout.
func (s *ClusterRerouteService) Timeout(timeout string) *ClusterRerouteService {
	s.timeout = timeout
	return s
}

// Add adds one or more commands to be executed.
func (s *ClusterRerouteService) Add(commands ...AllocationCommand) *ClusterRerouteService {
	s.commands = append(s.commands, commands...)
	return s
}

// Body specifies the body to be sent.
// If you specify Body, the commands passed via Add are ignored.
// In other words: Body takes precedence over Add.
func (s *ClusterRerouteService) Body(body interface{}) *ClusterRerouteService {
	s.body = body
	return s
}

// buildURL builds the URL for the operation.
func (s *ClusterRerouteService) buildURL() (string, url.Values, error) {
	// Build URL
	path := "/_cluster/reroute"

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
	if v := s.dryRun; v != nil {
		params.Set("dry_run", fmt.Sprint(*v))
	}
	if v := s.explain; v != nil {
		params.Set("explain", fmt.Sprint(*v))
	}
	if v := s.retryFailed; v != nil {
		params.Set("retry_failed", fmt.Sprint(*v))
	}
	if len(s.metrics) > 0 {
		params.Set("metric", strings.Join(s.metrics, ","))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ClusterRerouteService) Validate() error {
	if s.body == nil && len(s.commands) == 0 {
		return errors.New("missing allocate commands or raw body")
	}
	return nil
}

// Do executes the operation.
func (s *ClusterRerouteService) Do(ctx context.Context) (*ClusterRerouteResponse, error) {
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
	if s.body != nil {
		body = s.body
	} else {
		commands := make([]interface{}, len(s.commands))
		for i, cmd := range s.commands {
			src, err := cmd.Source()
			if err != nil {
				return nil, err
			}
			commands[i] = map[string]interface{}{
				cmd.Name(): src,
			}
		}
		query := make(map[string]interface{})
		query["commands"] = commands
		body = query
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
	ret := new(ClusterRerouteResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ClusterRerouteResponse is the response of ClusterRerouteService.Do.
type ClusterRerouteResponse struct {
	State        *ClusterStateResponse `json:"state"`
	Explanations []RerouteExplanation  `json:"explanations,omitempty"`
}

// RerouteExplanation is returned in ClusterRerouteResponse if
// the "explain" parameter is set to "true".
type RerouteExplanation struct {
	Command    string                 `json:"command"`
	Parameters map[string]interface{} `json:"parameters"`
	Decisions  []RerouteDecision      `json:"decisions"`
}

// RerouteDecision is a decision the decider made while rerouting.
type RerouteDecision interface{}

// -- Allocation commands --

// AllocationCommand is a command to be executed in a call
// to Cluster Reroute API.
type AllocationCommand interface {
	Name() string
	Source() (interface{}, error)
}

var _ AllocationCommand = (*MoveAllocationCommand)(nil)

// MoveAllocationCommand moves a shard from a specific node to
// another node.
type MoveAllocationCommand struct {
	index    string
	shardId  int
	fromNode string
	toNode   string
}

// NewMoveAllocationCommand creates a new MoveAllocationCommand.
func NewMoveAllocationCommand(index string, shardId int, fromNode, toNode string) *MoveAllocationCommand {
	return &MoveAllocationCommand{
		index:    index,
		shardId:  shardId,
		fromNode: fromNode,
		toNode:   toNode,
	}
}

// Name of the command in a request to the Cluster Reroute API.
func (cmd *MoveAllocationCommand) Name() string { return "move" }

// Source generates the (inner) JSON to be used when serializing the command.
func (cmd *MoveAllocationCommand) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["index"] = cmd.index
	source["shard"] = cmd.shardId
	source["from_node"] = cmd.fromNode
	source["to_node"] = cmd.toNode
	return source, nil
}

var _ AllocationCommand = (*CancelAllocationCommand)(nil)

// CancelAllocationCommand cancels relocation, or recovery of a given shard on a node.
type CancelAllocationCommand struct {
	index        string
	shardId      int
	node         string
	allowPrimary bool
}

// NewCancelAllocationCommand creates a new CancelAllocationCommand.
func NewCancelAllocationCommand(index string, shardId int, node string, allowPrimary bool) *CancelAllocationCommand {
	return &CancelAllocationCommand{
		index:        index,
		shardId:      shardId,
		node:         node,
		allowPrimary: allowPrimary,
	}
}

// Name of the command in a request to the Cluster Reroute API.
func (cmd *CancelAllocationCommand) Name() string { return "cancel" }

// Source generates the (inner) JSON to be used when serializing the command.
func (cmd *CancelAllocationCommand) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["index"] = cmd.index
	source["shard"] = cmd.shardId
	source["node"] = cmd.node
	source["allow_primary"] = cmd.allowPrimary
	return source, nil
}

var _ AllocationCommand = (*AllocateStalePrimaryAllocationCommand)(nil)

// AllocateStalePrimaryAllocationCommand allocates an unassigned stale
// primary shard to a specific node. Use with extreme care as this will
// result in data loss. Allocation deciders are ignored.
type AllocateStalePrimaryAllocationCommand struct {
	index          string
	shardId        int
	node           string
	acceptDataLoss bool
}

// NewAllocateStalePrimaryAllocationCommand creates a new
// AllocateStalePrimaryAllocationCommand.
func NewAllocateStalePrimaryAllocationCommand(index string, shardId int, node string, acceptDataLoss bool) *AllocateStalePrimaryAllocationCommand {
	return &AllocateStalePrimaryAllocationCommand{
		index:          index,
		shardId:        shardId,
		node:           node,
		acceptDataLoss: acceptDataLoss,
	}
}

// Name of the command in a request to the Cluster Reroute API.
func (cmd *AllocateStalePrimaryAllocationCommand) Name() string { return "allocate_stale_primary" }

// Source generates the (inner) JSON to be used when serializing the command.
func (cmd *AllocateStalePrimaryAllocationCommand) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["index"] = cmd.index
	source["shard"] = cmd.shardId
	source["node"] = cmd.node
	source["accept_data_loss"] = cmd.acceptDataLoss
	return source, nil
}

var _ AllocationCommand = (*AllocateReplicaAllocationCommand)(nil)

// AllocateReplicaAllocationCommand allocates an unassigned replica shard
// to a specific node. Checks if allocation deciders allow allocation.
type AllocateReplicaAllocationCommand struct {
	index   string
	shardId int
	node    string
}

// NewAllocateReplicaAllocationCommand creates a new
// AllocateReplicaAllocationCommand.
func NewAllocateReplicaAllocationCommand(index string, shardId int, node string) *AllocateReplicaAllocationCommand {
	return &AllocateReplicaAllocationCommand{
		index:   index,
		shardId: shardId,
		node:    node,
	}
}

// Name of the command in a request to the Cluster Reroute API.
func (cmd *AllocateReplicaAllocationCommand) Name() string { return "allocate_replica" }

// Source generates the (inner) JSON to be used when serializing the command.
func (cmd *AllocateReplicaAllocationCommand) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["index"] = cmd.index
	source["shard"] = cmd.shardId
	source["node"] = cmd.node
	return source, nil
}

// AllocateEmptyPrimaryAllocationCommand allocates an unassigned empty
// primary shard to a specific node. Use with extreme care as this will
// result in data loss. Allocation deciders are ignored.
type AllocateEmptyPrimaryAllocationCommand struct {
	index          string
	shardId        int
	node           string
	acceptDataLoss bool
}

// NewAllocateEmptyPrimaryAllocationCommand creates a new
// AllocateEmptyPrimaryAllocationCommand.
func NewAllocateEmptyPrimaryAllocationCommand(index string, shardId int, node string, acceptDataLoss bool) *AllocateEmptyPrimaryAllocationCommand {
	return &AllocateEmptyPrimaryAllocationCommand{
		index:          index,
		shardId:        shardId,
		node:           node,
		acceptDataLoss: acceptDataLoss,
	}
}

// Name of the command in a request to the Cluster Reroute API.
func (cmd *AllocateEmptyPrimaryAllocationCommand) Name() string { return "allocate_empty_primary" }

// Source generates the (inner) JSON to be used when serializing the command.
func (cmd *AllocateEmptyPrimaryAllocationCommand) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["index"] = cmd.index
	source["shard"] = cmd.shardId
	source["node"] = cmd.node
	source["accept_data_loss"] = cmd.acceptDataLoss
	return source, nil
}
