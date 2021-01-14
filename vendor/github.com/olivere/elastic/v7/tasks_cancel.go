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

// TasksCancelService can cancel long-running tasks.
// It is supported as of Elasticsearch 2.3.0.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/tasks.html#task-cancellation
// for details.
type TasksCancelService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	taskId       string
	actions      []string
	nodeId       []string
	parentTaskId string
}

// NewTasksCancelService creates a new TasksCancelService.
func NewTasksCancelService(client *Client) *TasksCancelService {
	return &TasksCancelService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *TasksCancelService) Pretty(pretty bool) *TasksCancelService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *TasksCancelService) Human(human bool) *TasksCancelService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *TasksCancelService) ErrorTrace(errorTrace bool) *TasksCancelService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *TasksCancelService) FilterPath(filterPath ...string) *TasksCancelService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *TasksCancelService) Header(name string, value string) *TasksCancelService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *TasksCancelService) Headers(headers http.Header) *TasksCancelService {
	s.headers = headers
	return s
}

// TaskId specifies the task to cancel. Notice that the caller is responsible
// for using the correct format, i.e. node_id:task_number, as specified in
// the REST API.
func (s *TasksCancelService) TaskId(taskId string) *TasksCancelService {
	s.taskId = taskId
	return s
}

// TaskIdFromNodeAndId specifies the task to cancel. Set id to -1 for all tasks.
func (s *TasksCancelService) TaskIdFromNodeAndId(nodeId string, id int64) *TasksCancelService {
	if id != -1 {
		s.taskId = fmt.Sprintf("%s:%d", nodeId, id)
	}
	return s
}

// Actions is a list of actions that should be cancelled. Leave empty to cancel all.
func (s *TasksCancelService) Actions(actions ...string) *TasksCancelService {
	s.actions = append(s.actions, actions...)
	return s
}

// NodeId is a list of node IDs or names to limit the returned information;
// use `_local` to return information from the node you're connecting to,
// leave empty to get information from all nodes.
func (s *TasksCancelService) NodeId(nodeId ...string) *TasksCancelService {
	s.nodeId = append(s.nodeId, nodeId...)
	return s
}

// ParentTaskId specifies to cancel tasks with specified parent task id.
// Notice that the caller is responsible for using the correct format,
// i.e. node_id:task_number, as specified in the REST API.
func (s *TasksCancelService) ParentTaskId(parentTaskId string) *TasksCancelService {
	s.parentTaskId = parentTaskId
	return s
}

// ParentTaskIdFromNodeAndId specifies to cancel tasks with specified parent task id.
func (s *TasksCancelService) ParentTaskIdFromNodeAndId(nodeId string, id int64) *TasksCancelService {
	if id != -1 {
		s.parentTaskId = fmt.Sprintf("%s:%d", nodeId, id)
	}
	return s
}

// buildURL builds the URL for the operation.
func (s *TasksCancelService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if s.taskId != "" {
		path, err = uritemplates.Expand("/_tasks/{task_id}/_cancel", map[string]string{
			"task_id": s.taskId,
		})
	} else {
		path = "/_tasks/_cancel"
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
	if len(s.actions) > 0 {
		params.Set("actions", strings.Join(s.actions, ","))
	}
	if len(s.nodeId) > 0 {
		params.Set("nodes", strings.Join(s.nodeId, ","))
	}
	if s.parentTaskId != "" {
		params.Set("parent_task_id", s.parentTaskId)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *TasksCancelService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *TasksCancelService) Do(ctx context.Context) (*TasksListResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(TasksListResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
