// Copyright 2018 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TeamsService provides access to the team-related functions
// in the GitHub API.
//
// GitHub API docs: https://developer.github.com/v3/teams/
type TeamsService service

// Team represents a team within a GitHub organization. Teams are used to
// manage access to an organization's repositories.
type Team struct {
	ID          *int64  `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
	Slug        *string `json:"slug,omitempty"`

	// Permission specifies the default permission for repositories owned by the team.
	Permission *string `json:"permission,omitempty"`

	// Privacy identifies the level of privacy this team should have.
	// Possible values are:
	//     secret - only visible to organization owners and members of this team
	//     closed - visible to all members of this organization
	// Default is "secret".
	Privacy *string `json:"privacy,omitempty"`

	MembersCount    *int          `json:"members_count,omitempty"`
	ReposCount      *int          `json:"repos_count,omitempty"`
	Organization    *Organization `json:"organization,omitempty"`
	MembersURL      *string       `json:"members_url,omitempty"`
	RepositoriesURL *string       `json:"repositories_url,omitempty"`
	Parent          *Team         `json:"parent,omitempty"`

	// LDAPDN is only available in GitHub Enterprise and when the team
	// membership is synchronized with LDAP.
	LDAPDN *string `json:"ldap_dn,omitempty"`
}

func (t Team) String() string {
	return Stringify(t)
}

// Invitation represents a team member's invitation status.
type Invitation struct {
	ID    *int64  `json:"id,omitempty"`
	Login *string `json:"login,omitempty"`
	Email *string `json:"email,omitempty"`
	// Role can be one of the values - 'direct_member', 'admin', 'billing_manager', 'hiring_manager', or 'reinstate'.
	Role              *string    `json:"role,omitempty"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	Inviter           *User      `json:"inviter,omitempty"`
	TeamCount         *int       `json:"team_count,omitempty"`
	InvitationTeamURL *string    `json:"invitation_team_url,omitempty"`
}

func (i Invitation) String() string {
	return Stringify(i)
}

// ListTeams lists all of the teams for an organization.
//
// GitHub API docs: https://developer.github.com/v3/teams/#list-teams
func (s *TeamsService) ListTeams(ctx context.Context, org string, opt *ListOptions) ([]*Team, *Response, error) {
	u := fmt.Sprintf("orgs/%v/teams", org)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	var teams []*Team
	resp, err := s.client.Do(ctx, req, &teams)
	if err != nil {
		return nil, resp, err
	}

	return teams, resp, nil
}

// GetTeam fetches a team by ID.
//
// GitHub API docs: https://developer.github.com/v3/teams/#get-team
func (s *TeamsService) GetTeam(ctx context.Context, team int64) (*Team, *Response, error) {
	u := fmt.Sprintf("teams/%v", team)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	t := new(Team)
	resp, err := s.client.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// NewTeam represents a team to be created or modified.
type NewTeam struct {
	Name         string   `json:"name"` // Name of the team. (Required.)
	Description  *string  `json:"description,omitempty"`
	Maintainers  []string `json:"maintainers,omitempty"`
	RepoNames    []string `json:"repo_names,omitempty"`
	ParentTeamID *int64   `json:"parent_team_id,omitempty"`

	// Deprecated: Permission is deprecated when creating or editing a team in an org
	// using the new GitHub permission model. It no longer identifies the
	// permission a team has on its repos, but only specifies the default
	// permission a repo is initially added with. Avoid confusion by
	// specifying a permission value when calling AddTeamRepo.
	Permission *string `json:"permission,omitempty"`

	// Privacy identifies the level of privacy this team should have.
	// Possible values are:
	//     secret - only visible to organization owners and members of this team
	//     closed - visible to all members of this organization
	// Default is "secret".
	Privacy *string `json:"privacy,omitempty"`

	// LDAPDN may be used in GitHub Enterprise when the team membership
	// is synchronized with LDAP.
	LDAPDN *string `json:"ldap_dn,omitempty"`
}

func (s NewTeam) String() string {
	return Stringify(s)
}

// CreateTeam creates a new team within an organization.
//
// GitHub API docs: https://developer.github.com/v3/teams/#create-team
func (s *TeamsService) CreateTeam(ctx context.Context, org string, team NewTeam) (*Team, *Response, error) {
	u := fmt.Sprintf("orgs/%v/teams", org)
	req, err := s.client.NewRequest("POST", u, team)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	t := new(Team)
	resp, err := s.client.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// EditTeam edits a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/#edit-team
func (s *TeamsService) EditTeam(ctx context.Context, id int64, team NewTeam) (*Team, *Response, error) {
	u := fmt.Sprintf("teams/%v", id)
	req, err := s.client.NewRequest("PATCH", u, team)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	t := new(Team)
	resp, err := s.client.Do(ctx, req, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}

// DeleteTeam deletes a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/#delete-team
func (s *TeamsService) DeleteTeam(ctx context.Context, team int64) (*Response, error) {
	u := fmt.Sprintf("teams/%v", team)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	return s.client.Do(ctx, req, nil)
}

// ListChildTeams lists child teams for a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/#list-child-teams
func (s *TeamsService) ListChildTeams(ctx context.Context, teamID int64, opt *ListOptions) ([]*Team, *Response, error) {
	u := fmt.Sprintf("teams/%v/teams", teamID)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	var teams []*Team
	resp, err := s.client.Do(ctx, req, &teams)
	if err != nil {
		return nil, resp, err
	}

	return teams, resp, nil
}

// ListTeamRepos lists the repositories that the specified team has access to.
//
// GitHub API docs: https://developer.github.com/v3/teams/#list-team-repos
func (s *TeamsService) ListTeamRepos(ctx context.Context, team int64, opt *ListOptions) ([]*Repository, *Response, error) {
	u := fmt.Sprintf("teams/%v/repos", team)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when topics API fully launches.
	headers := []string{mediaTypeTopicsPreview, mediaTypeNestedTeamsPreview}
	req.Header.Set("Accept", strings.Join(headers, ", "))

	var repos []*Repository
	resp, err := s.client.Do(ctx, req, &repos)
	if err != nil {
		return nil, resp, err
	}

	return repos, resp, nil
}

// IsTeamRepo checks if a team manages the specified repository. If the
// repository is managed by team, a Repository is returned which includes the
// permissions team has for that repo.
//
// GitHub API docs: https://developer.github.com/v3/teams/#check-if-a-team-manages-a-repository
func (s *TeamsService) IsTeamRepo(ctx context.Context, team int64, owner string, repo string) (*Repository, *Response, error) {
	u := fmt.Sprintf("teams/%v/repos/%v/%v", team, owner, repo)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	headers := []string{mediaTypeOrgPermissionRepo, mediaTypeNestedTeamsPreview}
	req.Header.Set("Accept", strings.Join(headers, ", "))

	repository := new(Repository)
	resp, err := s.client.Do(ctx, req, repository)
	if err != nil {
		return nil, resp, err
	}

	return repository, resp, nil
}

// TeamAddTeamRepoOptions specifies the optional parameters to the
// TeamsService.AddTeamRepo method.
type TeamAddTeamRepoOptions struct {
	// Permission specifies the permission to grant the team on this repository.
	// Possible values are:
	//     pull - team members can pull, but not push to or administer this repository
	//     push - team members can pull and push, but not administer this repository
	//     admin - team members can pull, push and administer this repository
	//
	// If not specified, the team's permission attribute will be used.
	Permission string `json:"permission,omitempty"`
}

// AddTeamRepo adds a repository to be managed by the specified team. The
// specified repository must be owned by the organization to which the team
// belongs, or a direct fork of a repository owned by the organization.
//
// GitHub API docs: https://developer.github.com/v3/teams/#add-team-repo
func (s *TeamsService) AddTeamRepo(ctx context.Context, team int64, owner string, repo string, opt *TeamAddTeamRepoOptions) (*Response, error) {
	u := fmt.Sprintf("teams/%v/repos/%v/%v", team, owner, repo)
	req, err := s.client.NewRequest("PUT", u, opt)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// RemoveTeamRepo removes a repository from being managed by the specified
// team. Note that this does not delete the repository, it just removes it
// from the team.
//
// GitHub API docs: https://developer.github.com/v3/teams/#remove-team-repo
func (s *TeamsService) RemoveTeamRepo(ctx context.Context, team int64, owner string, repo string) (*Response, error) {
	u := fmt.Sprintf("teams/%v/repos/%v/%v", team, owner, repo)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// ListUserTeams lists a user's teams
// GitHub API docs: https://developer.github.com/v3/teams/#list-user-teams
func (s *TeamsService) ListUserTeams(ctx context.Context, opt *ListOptions) ([]*Team, *Response, error) {
	u := "user/teams"
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeNestedTeamsPreview)

	var teams []*Team
	resp, err := s.client.Do(ctx, req, &teams)
	if err != nil {
		return nil, resp, err
	}

	return teams, resp, nil
}

// ListTeamProjects lists the organization projects for a team.
//
// GitHub API docs: https://developer.github.com/v3/teams/#list-team-projects
func (s *TeamsService) ListTeamProjects(ctx context.Context, teamID int64) ([]*Project, *Response, error) {
	u := fmt.Sprintf("teams/%v/projects", teamID)

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	acceptHeaders := []string{mediaTypeNestedTeamsPreview, mediaTypeProjectsPreview}
	req.Header.Set("Accept", strings.Join(acceptHeaders, ", "))

	var projects []*Project
	resp, err := s.client.Do(ctx, req, &projects)
	if err != nil {
		return nil, resp, err
	}

	return projects, resp, nil
}

// ReviewTeamProjects checks whether a team has read, write, or admin
// permissions for an organization project.
//
// GitHub API docs: https://developer.github.com/v3/teams/#review-a-team-project
func (s *TeamsService) ReviewTeamProjects(ctx context.Context, teamID, projectID int64) (*Project, *Response, error) {
	u := fmt.Sprintf("teams/%v/projects/%v", teamID, projectID)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	acceptHeaders := []string{mediaTypeNestedTeamsPreview, mediaTypeProjectsPreview}
	req.Header.Set("Accept", strings.Join(acceptHeaders, ", "))

	projects := &Project{}
	resp, err := s.client.Do(ctx, req, &projects)
	if err != nil {
		return nil, resp, err
	}

	return projects, resp, nil
}

// TeamProjectOptions specifies the optional parameters to the
// TeamsService.AddTeamProject method.
type TeamProjectOptions struct {
	// Permission specifies the permission to grant to the team for this project.
	// Possible values are:
	//     "read" - team members can read, but not write to or administer this project.
	//     "write" - team members can read and write, but not administer this project.
	//     "admin" - team members can read, write and administer this project.
	//
	Permission *string `json:"permission,omitempty"`
}

// AddTeamProject adds an organization project to a team. To add a project to a team or
// update the team's permission on a project, the authenticated user must have admin
// permissions for the project.
//
// GitHub API docs: https://developer.github.com/v3/teams/#add-or-update-team-project
func (s *TeamsService) AddTeamProject(ctx context.Context, teamID, projectID int64, opt *TeamProjectOptions) (*Response, error) {
	u := fmt.Sprintf("teams/%v/projects/%v", teamID, projectID)
	req, err := s.client.NewRequest("PUT", u, opt)
	if err != nil {
		return nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	acceptHeaders := []string{mediaTypeNestedTeamsPreview, mediaTypeProjectsPreview}
	req.Header.Set("Accept", strings.Join(acceptHeaders, ", "))

	return s.client.Do(ctx, req, nil)
}

// RemoveTeamProject removes an organization project from a team. An organization owner or
// a team maintainer can remove any project from the team. To remove a project from a team
// as an organization member, the authenticated user must have "read" access to both the team
// and project, or "admin" access to the team or project.
// Note: This endpoint removes the project from the team, but does not delete it.
//
// GitHub API docs: https://developer.github.com/v3/teams/#remove-team-project
func (s *TeamsService) RemoveTeamProject(ctx context.Context, teamID int64, projectID int64) (*Response, error) {
	u := fmt.Sprintf("teams/%v/projects/%v", teamID, projectID)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	acceptHeaders := []string{mediaTypeNestedTeamsPreview, mediaTypeProjectsPreview}
	req.Header.Set("Accept", strings.Join(acceptHeaders, ", "))

	return s.client.Do(ctx, req, nil)
}
