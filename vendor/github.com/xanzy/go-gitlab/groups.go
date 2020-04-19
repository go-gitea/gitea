//
// Copyright 2017, Sander van Harmelen
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
)

// GroupsService handles communication with the group related methods of
// the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html
type GroupsService struct {
	client *Client
}

// Group represents a GitLab group.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html
type Group struct {
	ID                             int                `json:"id"`
	Name                           string             `json:"name"`
	Path                           string             `json:"path"`
	Description                    string             `json:"description"`
	Visibility                     *VisibilityValue   `json:"visibility"`
	LFSEnabled                     bool               `json:"lfs_enabled"`
	AvatarURL                      string             `json:"avatar_url"`
	WebURL                         string             `json:"web_url"`
	RequestAccessEnabled           bool               `json:"request_access_enabled"`
	FullName                       string             `json:"full_name"`
	FullPath                       string             `json:"full_path"`
	ParentID                       int                `json:"parent_id"`
	Projects                       []*Project         `json:"projects"`
	Statistics                     *StorageStatistics `json:"statistics"`
	CustomAttributes               []*CustomAttribute `json:"custom_attributes"`
	ShareWithGroupLock             bool               `json:"share_with_group_lock"`
	RequireTwoFactorAuth           bool               `json:"require_two_factor_authentication"`
	TwoFactorGracePeriod           int                `json:"two_factor_grace_period"`
	ProjectCreationLevel           string             `json:"project_creation_level"`
	AutoDevopsEnabled              bool               `json:"auto_devops_enabled"`
	SubGroupCreationLevel          string             `json:"subgroup_creation_level"`
	EmailsDisabled                 bool               `json:"emails_disabled"`
	RunnersToken                   string             `json:"runners_token"`
	SharedProjects                 []*Project         `json:"shared_projects"`
	LDAPCN                         string             `json:"ldap_cn"`
	LDAPAccess                     bool               `json:"ldap_access"`
	SharedRunnersMinutesLimit      int                `json:"shared_runners_minutes_limit"`
	ExtraSharedRunnersMinutesLimit int                `json:"extra_shared_runners_minutes_limit"`
}

// ListGroupsOptions represents the available ListGroups() options.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#list-project-groups
type ListGroupsOptions struct {
	ListOptions
	AllAvailable         *bool             `url:"all_available,omitempty" json:"all_available,omitempty"`
	MinAccessLevel       *AccessLevelValue `url:"min_access_level,omitempty" json:"min_access_level,omitempty"`
	OrderBy              *string           `url:"order_by,omitempty" json:"order_by,omitempty"`
	Owned                *bool             `url:"owned,omitempty" json:"owned,omitempty"`
	Search               *string           `url:"search,omitempty" json:"search,omitempty"`
	SkipGroups           []int             `url:"skip_groups,omitempty" json:"skip_groups,omitempty"`
	Sort                 *string           `url:"sort,omitempty" json:"sort,omitempty"`
	Statistics           *bool             `url:"statistics,omitempty" json:"statistics,omitempty"`
	WithCustomAttributes *bool             `url:"with_custom_attributes,omitempty" json:"with_custom_attributes,omitempty"`
}

// ListGroups gets a list of groups (as user: my groups, as admin: all groups).
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#list-project-groups
func (s *GroupsService) ListGroups(opt *ListGroupsOptions, options ...OptionFunc) ([]*Group, *Response, error) {
	req, err := s.client.NewRequest("GET", "groups", opt, options)
	if err != nil {
		return nil, nil, err
	}

	var g []*Group
	resp, err := s.client.Do(req, &g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// GetGroup gets all details of a group.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#details-of-a-group
func (s *GroupsService) GetGroup(gid interface{}, options ...OptionFunc) (*Group, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s", pathEscape(group))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	g := new(Group)
	resp, err := s.client.Do(req, g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// CreateGroupOptions represents the available CreateGroup() options.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#new-group
type CreateGroupOptions struct {
	Name                 *string          `url:"name,omitempty" json:"name,omitempty"`
	Path                 *string          `url:"path,omitempty" json:"path,omitempty"`
	Description          *string          `url:"description,omitempty" json:"description,omitempty"`
	Visibility           *VisibilityValue `url:"visibility,omitempty" json:"visibility,omitempty"`
	LFSEnabled           *bool            `url:"lfs_enabled,omitempty" json:"lfs_enabled,omitempty"`
	RequestAccessEnabled *bool            `url:"request_access_enabled,omitempty" json:"request_access_enabled,omitempty"`
	ParentID             *int             `url:"parent_id,omitempty" json:"parent_id,omitempty"`
}

// CreateGroup creates a new project group. Available only for users who can
// create groups.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#new-group
func (s *GroupsService) CreateGroup(opt *CreateGroupOptions, options ...OptionFunc) (*Group, *Response, error) {
	req, err := s.client.NewRequest("POST", "groups", opt, options)
	if err != nil {
		return nil, nil, err
	}

	g := new(Group)
	resp, err := s.client.Do(req, g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// TransferGroup transfers a project to the Group namespace. Available only
// for admin.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#transfer-project-to-group
func (s *GroupsService) TransferGroup(gid interface{}, pid interface{}, options ...OptionFunc) (*Group, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/projects/%s", pathEscape(group), pathEscape(project))

	req, err := s.client.NewRequest("POST", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	g := new(Group)
	resp, err := s.client.Do(req, g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// UpdateGroupOptions represents the set of available options to update a Group;
// as of today these are exactly the same available when creating a new Group.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#update-group
type UpdateGroupOptions CreateGroupOptions

// UpdateGroup updates an existing group; only available to group owners and
// administrators.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#update-group
func (s *GroupsService) UpdateGroup(gid interface{}, opt *UpdateGroupOptions, options ...OptionFunc) (*Group, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s", pathEscape(group))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	g := new(Group)
	resp, err := s.client.Do(req, g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// DeleteGroup removes group with all projects inside.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#remove-group
func (s *GroupsService) DeleteGroup(gid interface{}, options ...OptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s", pathEscape(group))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SearchGroup get all groups that match your string in their name or path.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/groups.html#search-for-group
func (s *GroupsService) SearchGroup(query string, options ...OptionFunc) ([]*Group, *Response, error) {
	var q struct {
		Search string `url:"search,omitempty" json:"search,omitempty"`
	}
	q.Search = query

	req, err := s.client.NewRequest("GET", "groups", &q, options)
	if err != nil {
		return nil, nil, err
	}

	var g []*Group
	resp, err := s.client.Do(req, &g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}

// ListGroupProjectsOptions represents the available ListGroupProjects()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#list-a-group-39-s-projects
type ListGroupProjectsOptions struct {
	ListOptions
	Archived                 *bool            `url:"archived,omitempty" json:"archived,omitempty"`
	Visibility               *VisibilityValue `url:"visibility,omitempty" json:"visibility,omitempty"`
	OrderBy                  *string          `url:"order_by,omitempty" json:"order_by,omitempty"`
	Sort                     *string          `url:"sort,omitempty" json:"sort,omitempty"`
	Search                   *string          `url:"search,omitempty" json:"search,omitempty"`
	Simple                   *bool            `url:"simple,omitempty" json:"simple,omitempty"`
	Owned                    *bool            `url:"owned,omitempty" json:"owned,omitempty"`
	Starred                  *bool            `url:"starred,omitempty" json:"starred,omitempty"`
	WithIssuesEnabled        *bool            `url:"with_issues_enabled,omitempty" json:"with_issues_enabled,omitempty"`
	WithMergeRequestsEnabled *bool            `url:"with_merge_requests_enabled,omitempty" json:"with_merge_requests_enabled,omitempty"`
	WithShared               *bool            `url:"with_shared,omitempty" json:"with_shared,omitempty"`
	IncludeSubgroups         *bool            `url:"include_subgroups,omitempty" json:"include_subgroups,omitempty"`
	WithCustomAttributes     *bool            `url:"with_custom_attributes,omitempty" json:"with_custom_attributes,omitempty"`
}

// ListGroupProjects get a list of group projects
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#list-a-group-39-s-projects
func (s *GroupsService) ListGroupProjects(gid interface{}, opt *ListGroupProjectsOptions, options ...OptionFunc) ([]*Project, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/projects", pathEscape(group))

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var p []*Project
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, err
}

// ListSubgroupsOptions represents the available ListSubgroupsOptions()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#list-a-groups-s-subgroups
type ListSubgroupsOptions ListGroupsOptions

// ListSubgroups gets a list of subgroups for a given project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/groups.html#list-a-groups-s-subgroups
func (s *GroupsService) ListSubgroups(gid interface{}, opt *ListSubgroupsOptions, options ...OptionFunc) ([]*Group, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/subgroups", pathEscape(group))

	req, err := s.client.NewRequest("GET", u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var g []*Group
	resp, err := s.client.Do(req, &g)
	if err != nil {
		return nil, resp, err
	}

	return g, resp, err
}
