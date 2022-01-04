//
// Copyright 2021, Sander van Harmelen
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
)

// ProtectedEnvironmentsService handles communication with the protected
// environment methods of the GitLab API.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html
type ProtectedEnvironmentsService struct {
	client *Client
}

// ProtectedEnvironment represents a protected environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html
type ProtectedEnvironment struct {
	Name               string                          `json:"name"`
	DeployAccessLevels []*EnvironmentAccessDescription `json:"deploy_access_levels"`
}

// EnvironmentAccessDescription represents the access decription for a protected
// environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html
type EnvironmentAccessDescription struct {
	AccessLevel            AccessLevelValue `json:"access_level"`
	AccessLevelDescription string           `json:"access_level_description"`
	UserID                 int              `json:"user_id"`
	GroupID                int              `json:"group_id"`
}

// ListProtectedEnvironmentsOptions represents the available
// ListProtectedEnvironments() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#list-protected-environments
type ListProtectedEnvironmentsOptions ListOptions

// ListProtectedEnvironments returns a list of protected environments from a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#list-protected-environments
func (s *ProtectedEnvironmentsService) ListProtectedEnvironments(pid interface{}, opt *ListProtectedEnvironmentsOptions, options ...RequestOptionFunc) ([]*ProtectedEnvironment, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_environments", pathEscape(project))

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var pes []*ProtectedEnvironment
	resp, err := s.client.Do(req, &pes)
	if err != nil {
		return nil, resp, err
	}

	return pes, resp, err
}

// GetProtectedEnvironment returns a single protected environment or wildcard protected environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#get-a-single-protected-environment-or-wildcard-protected-environment
func (s *ProtectedEnvironmentsService) GetProtectedEnvironment(pid interface{}, environment string, options ...RequestOptionFunc) (*ProtectedEnvironment, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_environments/%s", pathEscape(project), pathEscape(environment))

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	pe := new(ProtectedEnvironment)
	resp, err := s.client.Do(req, pe)
	if err != nil {
		return nil, resp, err
	}

	return pe, resp, err
}

// ProtectRepositoryEnvironmentsOptions represents the available
// ProtectRepositoryEnvironments() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#protect-repository-environments
type ProtectRepositoryEnvironmentsOptions struct {
	Name               *string                     `url:"name,omitempty" json:"name,omitempty"`
	DeployAccessLevels []*EnvironmentAccessOptions `url:"deploy_access_levels,omitempty" json:"deploy_access_levels,omitempty"`
}

// EnvironmentAccessOptions represents the options for an access decription for
// a protected environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#protect-repository-environments
type EnvironmentAccessOptions struct {
	AccessLevel *AccessLevelValue `url:"access_level,omitempty" json:"access_level,omitempty"`
	UserID      *int              `url:"user_id,omitempty" json:"user_id,omitempty"`
	GroupID     *int              `url:"group_id,omitempty" json:"group_id,omitempty"`
}

// ProtectRepositoryEnvironments protects a single repository environment or several project
// repository environments using a wildcard protected environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#protect-repository-environments
func (s *ProtectedEnvironmentsService) ProtectRepositoryEnvironments(pid interface{}, opt *ProtectRepositoryEnvironmentsOptions, options ...RequestOptionFunc) (*ProtectedEnvironment, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_environments", pathEscape(project))

	req, err := s.client.NewRequest(http.MethodPost, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	pe := new(ProtectedEnvironment)
	resp, err := s.client.Do(req, pe)
	if err != nil {
		return nil, resp, err
	}

	return pe, resp, err
}

// UnprotectEnvironment unprotects the given protected environment or wildcard
// protected environment.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/protected_environments.html#unprotect-repository-environments
func (s *ProtectedEnvironmentsService) UnprotectEnvironment(pid interface{}, environment string, options ...RequestOptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_environments/%s", pathEscape(project), pathEscape(environment))

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
