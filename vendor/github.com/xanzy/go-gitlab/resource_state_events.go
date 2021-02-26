//
// Copyright 2021, Matthias Simon
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gitlab

import (
	"fmt"
	"net/http"
	"time"
)

// ResourceStateEventsService handles communication with the event related
// methods of the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/resource_state_events.html
type ResourceStateEventsService struct {
	client *Client
}

// StateEvent represents a resource state event.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/resource_state_events.html
type StateEvent struct {
	ID           int            `json:"id"`
	User         *BasicUser     `json:"user"`
	CreatedAt    *time.Time     `json:"created_at"`
	ResourceType string         `json:"resource_type"`
	ResourceID   int            `json:"resource_id"`
	State        EventTypeValue `json:"state"`
}

// ListStateEventsOptions represents the options for all resource state events
// list methods.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_state_events.html#list-project-issue-state-events
type ListStateEventsOptions struct {
	ListOptions
}

// ListIssueStateEvents retrieves resource state events for the specified
// project and issue.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_state_events.html#list-project-issue-state-events
func (s *ResourceStateEventsService) ListIssueStateEvents(pid interface{}, issue int, opt *ListStateEventsOptions, options ...RequestOptionFunc) ([]*StateEvent, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/issues/%d/resource_state_events", pathEscape(project), issue)

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var ses []*StateEvent
	resp, err := s.client.Do(req, &ses)
	if err != nil {
		return nil, resp, err
	}

	return ses, resp, err
}

// GetIssueStateEvent gets a single issue-state-event.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_state_events.html#get-single-issue-state-event
func (s *ResourceStateEventsService) GetIssueStateEvent(pid interface{}, issue int, event int, options ...RequestOptionFunc) (*StateEvent, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/issues/%d/resource_state_events/%d", pathEscape(project), issue, event)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	se := new(StateEvent)
	resp, err := s.client.Do(req, se)
	if err != nil {
		return nil, resp, err
	}

	return se, resp, err
}

// ListMergeStateEvents retrieves resource state events for the specified
// project and merge request.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_state_events.html#list-project-merge-request-state-events
func (s *ResourceStateEventsService) ListMergeStateEvents(pid interface{}, request int, opt *ListStateEventsOptions, options ...RequestOptionFunc) ([]*StateEvent, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/merge_requests/%d/resource_state_events", pathEscape(project), request)

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var ses []*StateEvent
	resp, err := s.client.Do(req, &ses)
	if err != nil {
		return nil, resp, err
	}

	return ses, resp, err
}

// GetMergeRequestStateEvent gets a single merge request state event.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_state_events.html#get-single-merge-request-state-event
func (s *ResourceStateEventsService) GetMergeRequestStateEvent(pid interface{}, request int, event int, options ...RequestOptionFunc) (*StateEvent, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/merge_requests/%d/resource_state_events/%d", pathEscape(project), request, event)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	se := new(StateEvent)
	resp, err := s.client.Do(req, se)
	if err != nil {
		return nil, resp, err
	}

	return se, resp, err
}
