//
// Copyright 2017, Sander van Harmelen, Michael Lihs
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
	"net/url"
)

// ProtectedBranchesService handles communication with the protected branch
// related methods of the GitLab API.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#protected-branches-api
type ProtectedBranchesService struct {
	client *Client
}

// BranchAccessDescription represents the access description for a protected
// branch.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#protected-branches-api
type BranchAccessDescription struct {
	AccessLevel            AccessLevelValue `json:"access_level"`
	AccessLevelDescription string           `json:"access_level_description"`
}

// ProtectedBranch represents a protected branch.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#list-protected-branches
type ProtectedBranch struct {
	Name              string                     `json:"name"`
	PushAccessLevels  []*BranchAccessDescription `json:"push_access_levels"`
	MergeAccessLevels []*BranchAccessDescription `json:"merge_access_levels"`
}

// ListProtectedBranchesOptions represents the available ListProtectedBranches()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#list-protected-branches
type ListProtectedBranchesOptions ListOptions

// ListProtectedBranches gets a list of protected branches from a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#list-protected-branches
func (s *ProtectedBranchesService) ListProtectedBranches(pid interface{}, opt *ListProtectedBranchesOptions, options ...OptionFunc) ([]*ProtectedBranch, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_branches", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var p []*ProtectedBranch
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, err
}

// GetProtectedBranch gets a single protected branch or wildcard protected branch.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#get-a-single-protected-branch-or-wildcard-protected-branch
func (s *ProtectedBranchesService) GetProtectedBranch(pid interface{}, branch string, options ...OptionFunc) (*ProtectedBranch, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_branches/%s", pathEscape(project), url.PathEscape(branch))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	p := new(ProtectedBranch)
	resp, err := s.client.Do(req, p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, err
}

// ProtectRepositoryBranchesOptions represents the available
// ProtectRepositoryBranches() options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#protect-repository-branches
type ProtectRepositoryBranchesOptions struct {
	Name             *string           `url:"name,omitempty" json:"name,omitempty"`
	PushAccessLevel  *AccessLevelValue `url:"push_access_level,omitempty" json:"push_access_level,omitempty"`
	MergeAccessLevel *AccessLevelValue `url:"merge_access_level,omitempty" json:"merge_access_level,omitempty"`
}

// ProtectRepositoryBranches protects a single repository branch or several
// project repository branches using a wildcard protected branch.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#protect-repository-branches
func (s *ProtectedBranchesService) ProtectRepositoryBranches(pid interface{}, opt *ProtectRepositoryBranchesOptions, options ...OptionFunc) (*ProtectedBranch, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_branches", pathEscape(project))

	req, err := s.client.NewRequest("POST", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	p := new(ProtectedBranch)
	resp, err := s.client.Do(req, p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, err
}

// UnprotectRepositoryBranches unprotects the given protected branch or wildcard
// protected branch.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/protected_branches.html#unprotect-repository-branches
func (s *ProtectedBranchesService) UnprotectRepositoryBranches(pid interface{}, branch string, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/protected_branches/%s", pathEscape(project), url.PathEscape(branch))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
