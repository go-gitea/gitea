//
// Copyright 2021, Patrick Webster
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

// ProjectAccessTokensService handles communication with the
// project access tokens related methods of the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/resource_access_tokens.html
type ProjectAccessTokensService struct {
	client *Client
}

// ProjectAccessToken represents a GitLab Project Access Token.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/resource_access_tokens.html
type ProjectAccessToken struct {
	ID        int        `json:"id"`
	UserID    int        `json:"user_id"`
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	CreatedAt *time.Time `json:"created_at"`
	ExpiresAt *ISOTime   `json:"expires_at"`
	Active    bool       `json:"active"`
	Revoked   bool       `json:"revoked"`
	Token     string     `json:"token"`
}

func (v ProjectAccessToken) String() string {
	return Stringify(v)
}

// ListProjectAccessTokensOptions represents the available options for
// listing variables in a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_access_tokens.html#list-project-access-tokens
type ListProjectAccessTokensOptions ListOptions

// ListProjectAccessTokens gets a list of all Project Access Tokens in a
// project.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_access_tokens.html#list-project-access-tokens
func (s *ProjectAccessTokensService) ListProjectAccessTokens(pid interface{}, opt *ListProjectAccessTokensOptions, options ...RequestOptionFunc) ([]*ProjectAccessToken, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/access_tokens", pathEscape(project))

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var pats []*ProjectAccessToken
	resp, err := s.client.Do(req, &pats)
	if err != nil {
		return nil, resp, err
	}

	return pats, resp, err
}

// CreateProjectAccessTokenOptions represents the available CreateVariable()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_access_tokens.html#create-a-project-access-token
type CreateProjectAccessTokenOptions struct {
	Name      *string  `url:"name,omitempty" json:"name,omitempty"`
	Scopes    []string `url:"scopes,omitempty" json:"scopes,omitempty"`
	ExpiresAt *ISOTime `url:"expires_at,omitempty" json:"expires_at,omitempty"`
}

// CreateProjectAccessToken creates a new Project Access Token.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_access_tokens.html#create-a-project-access-token
func (s *ProjectAccessTokensService) CreateProjectAccessToken(pid interface{}, opt *CreateProjectAccessTokenOptions, options ...RequestOptionFunc) (*ProjectAccessToken, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/access_tokens", pathEscape(project))

	req, err := s.client.NewRequest(http.MethodPost, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	pat := new(ProjectAccessToken)
	resp, err := s.client.Do(req, pat)
	if err != nil {
		return nil, resp, err
	}

	return pat, resp, err
}

// DeleteProjectAccessToken deletes a Project Access Token.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/resource_access_tokens.html#revoke-a-project-access-token
func (s *ProjectAccessTokensService) DeleteProjectAccessToken(pid interface{}, id int, options ...RequestOptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/access_tokens/%d", pathEscape(project), id)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
