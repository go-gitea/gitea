package elastic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// TasksGetTaskService retrieves the state of a task in the cluster. It is part of the Task Management API
// documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/tasks.html#_current_tasks_information.
type TasksGetTaskService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	taskId            string
	waitForCompletion *bool
}

// NewTasksGetTaskService creates a new TasksGetTaskService.
func NewTasksGetTaskService(client *Client) *TasksGetTaskService {
	return &TasksGetTaskService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *TasksGetTaskService) Pretty(pretty bool) *TasksGetTaskService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *TasksGetTaskService) Human(human bool) *TasksGetTaskService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *TasksGetTaskService) ErrorTrace(errorTrace bool) *TasksGetTaskService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *TasksGetTaskService) FilterPath(filterPath ...string) *TasksGetTaskService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *TasksGetTaskService) Header(name string, value string) *TasksGetTaskService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *TasksGetTaskService) Headers(headers http.Header) *TasksGetTaskService {
	s.headers = headers
	return s
}

// TaskId specifies the task to return. Notice that the caller is responsible
// for using the correct format, i.e. node_id:task_number, as specified in
// the REST API.
func (s *TasksGetTaskService) TaskId(taskId string) *TasksGetTaskService {
	s.taskId = taskId
	return s
}

// TaskIdFromNodeAndId indicates to return the task on the given node with specified id.
func (s *TasksGetTaskService) TaskIdFromNodeAndId(nodeId string, id int64) *TasksGetTaskService {
	s.taskId = fmt.Sprintf("%s:%d", nodeId, id)
	return s
}

// WaitForCompletion indicates whether to wait for the matching tasks
// to complete (default: false).
func (s *TasksGetTaskService) WaitForCompletion(waitForCompletion bool) *TasksGetTaskService {
	s.waitForCompletion = &waitForCompletion
	return s
}

// buildURL builds the URL for the operation.
func (s *TasksGetTaskService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_tasks/{task_id}", map[string]string{
		"task_id": s.taskId,
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
	if v := s.waitForCompletion; v != nil {
		params.Set("wait_for_completion", fmt.Sprint(*v))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *TasksGetTaskService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *TasksGetTaskService) Do(ctx context.Context) (*TasksGetTaskResponse, error) {
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
	ret := new(TasksGetTaskResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	ret.Header = res.Header
	return ret, nil
}

type TasksGetTaskResponse struct {
	Header    http.Header `json:"-"`
	Completed bool        `json:"completed"`
	Task      *TaskInfo   `json:"task,omitempty"`
}
