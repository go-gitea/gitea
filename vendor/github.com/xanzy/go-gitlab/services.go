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
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ServicesService handles communication with the services related methods of
// the GitLab API.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/services.html
type ServicesService struct {
	client *Client
}

// Service represents a GitLab service.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/services.html
type Service struct {
	ID                       int        `json:"id"`
	Title                    string     `json:"title"`
	CreatedAt                *time.Time `json:"created_at"`
	UpdatedAt                *time.Time `json:"updated_at"`
	Active                   bool       `json:"active"`
	PushEvents               bool       `json:"push_events"`
	IssuesEvents             bool       `json:"issues_events"`
	ConfidentialIssuesEvents bool       `json:"confidential_issues_events"`
	MergeRequestsEvents      bool       `json:"merge_requests_events"`
	TagPushEvents            bool       `json:"tag_push_events"`
	NoteEvents               bool       `json:"note_events"`
	ConfidentialNoteEvents   bool       `json:"confidential_note_events"`
	PipelineEvents           bool       `json:"pipeline_events"`
	JobEvents                bool       `json:"job_events"`
	WikiPageEvents           bool       `json:"wiki_page_events"`
}

// SetGitLabCIServiceOptions represents the available SetGitLabCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-gitlab-ci-service
type SetGitLabCIServiceOptions struct {
	Token      *string `url:"token,omitempty" json:"token,omitempty"`
	ProjectURL *string `url:"project_url,omitempty" json:"project_url,omitempty"`
}

// SetGitLabCIService sets GitLab CI service for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-gitlab-ci-service
func (s *ServicesService) SetGitLabCIService(pid interface{}, opt *SetGitLabCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/gitlab-ci", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteGitLabCIService deletes GitLab CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-gitlab-ci-service
func (s *ServicesService) DeleteGitLabCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/gitlab-ci", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GithubService represents Github service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#github-premium
type GithubService struct {
	Service
	Properties *GithubServiceProperties `json:"properties"`
}

// GithubServiceProperties represents Github specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#github-premium
type GithubServiceProperties struct {
	RepositoryURL string `json:"repository_url,omitempty"`
	StaticContext string `json:"static_context,omitempty"`
}

// GetGithubService gets Github service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-github-service-settings
func (s *ServicesService) GetGithubService(pid interface{}, options ...OptionFunc) (*GithubService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/github", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(GithubService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetGithubServiceOptions represents the available SetGithubService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-github-service
type SetGithubServiceOptions struct {
	Token         *string `url:"token,omitempty" json:"token,omitempty"`
	RepositoryURL *string `url:"repository_url,omitempty" json:"repository_url,omitempty"`
	StaticContext *bool   `url:"static_context,omitempty" json:"static_context,omitempty"`
}

// SetGithubService sets Github service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-github-service
func (s *ServicesService) SetGithubService(pid interface{}, opt *SetGithubServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/github", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteGithubService deletes Github service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-github-service
func (s *ServicesService) DeleteGithubService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/github", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SetHipChatServiceOptions represents the available SetHipChatService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-hipchat-service
type SetHipChatServiceOptions struct {
	Token *string `url:"token,omitempty" json:"token,omitempty" `
	Room  *string `url:"room,omitempty" json:"room,omitempty"`
}

// SetHipChatService sets HipChat service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-hipchat-service
func (s *ServicesService) SetHipChatService(pid interface{}, opt *SetHipChatServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/hipchat", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteHipChatService deletes HipChat service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-hipchat-service
func (s *ServicesService) DeleteHipChatService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/hipchat", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DroneCIService represents Drone CI service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#drone-ci
type DroneCIService struct {
	Service
	Properties *DroneCIServiceProperties `json:"properties"`
}

// DroneCIServiceProperties represents Drone CI specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#drone-ci
type DroneCIServiceProperties struct {
	Token                 string `json:"token"`
	DroneURL              string `json:"drone_url"`
	EnableSSLVerification bool   `json:"enable_ssl_verification"`
}

// GetDroneCIService gets Drone CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-drone-ci-service-settings
func (s *ServicesService) GetDroneCIService(pid interface{}, options ...OptionFunc) (*DroneCIService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(DroneCIService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetDroneCIServiceOptions represents the available SetDroneCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-drone-ci-service
type SetDroneCIServiceOptions struct {
	Token                 *string `url:"token" json:"token" `
	DroneURL              *string `url:"drone_url" json:"drone_url"`
	EnableSSLVerification *bool   `url:"enable_ssl_verification,omitempty" json:"enable_ssl_verification,omitempty"`
}

// SetDroneCIService sets Drone CI service for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-drone-ci-service
func (s *ServicesService) SetDroneCIService(pid interface{}, opt *SetDroneCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteDroneCIService deletes Drone CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-drone-ci-service
func (s *ServicesService) DeleteDroneCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/drone-ci", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SlackService represents Slack service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#slack
type SlackService struct {
	Service
	Properties *SlackServiceProperties `json:"properties"`
}

// SlackServiceProperties represents Slack specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#slack
type SlackServiceProperties struct {
	WebHook                   string    `json:"webhook,omitempty"`
	Username                  string    `json:"username,omitempty"`
	Channel                   string    `json:"channel,omitempty"`
	NotifyOnlyBrokenPipelines BoolValue `json:"notify_only_broken_pipelines,omitempty"`
	NotifyOnlyDefaultBranch   BoolValue `json:"notify_only_default_branch,omitempty"`
	ConfidentialIssueChannel  string    `json:"confidential_issue_channel,omitempty"`
	ConfidentialNoteChannel   string    `json:"confidential_note_channel,omitempty"`
	DeploymentChannel         string    `json:"deployment_channel,omitempty"`
	IssueChannel              string    `json:"issue_channel,omitempty"`
	MergeRequestChannel       string    `json:"merge_request_channel,omitempty"`
	NoteChannel               string    `json:"note_channel,omitempty"`
	TagPushChannel            string    `json:"tag_push_channel,omitempty"`
	PipelineChannel           string    `json:"pipeline_channel,omitempty"`
	PushChannel               string    `json:"push_channel,omitempty"`
	WikiPageChannel           string    `json:"wiki_page_channel,omitempty"`
}

// GetSlackService gets Slack service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-slack-service-settings
func (s *ServicesService) GetSlackService(pid interface{}, options ...OptionFunc) (*SlackService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(SlackService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetSlackServiceOptions represents the available SetSlackService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-slack-service
type SetSlackServiceOptions struct {
	WebHook                   *string `url:"webhook,omitempty" json:"webhook,omitempty"`
	Username                  *string `url:"username,omitempty" json:"username,omitempty"`
	Channel                   *string `url:"channel,omitempty" json:"channel,omitempty"`
	NotifyOnlyBrokenPipelines *bool   `url:"notify_only_broken_pipelines,omitempty" json:"notify_only_broken_pipelines,omitempty"`
	NotifyOnlyDefaultBranch   *bool   `url:"notify_only_default_branch,omitempty" json:"notify_only_default_branch,omitempty"`
	ConfidentialIssueChannel  *string `url:"confidential_issue_channel,omitempty" json:"confidential_issue_channel,omitempty"`
	ConfidentialIssuesEvents  *bool   `url:"confidential_issues_events,omitempty" json:"confidential_issues_events,omitempty"`
	// TODO: Currently, GitLab ignores this option (not implemented yet?), so
	// there is no way to set it. Uncomment when this is fixed.
	// See: https://gitlab.com/gitlab-org/gitlab-ce/issues/49730
	//ConfidentialNoteChannel   *string `json:"confidential_note_channel,omitempty"`
	ConfidentialNoteEvents *bool   `url:"confidential_note_events,omitempty" json:"confidential_note_events,omitempty"`
	DeploymentChannel      *string `url:"deployment_channel,omitempty" json:"deployment_channel,omitempty"`
	DeploymentEvents       *bool   `url:"deployment_events,omitempty" json:"deployment_events,omitempty"`
	IssueChannel           *string `url:"issue_channel,omitempty" json:"issue_channel,omitempty"`
	IssuesEvents           *bool   `url:"issues_events,omitempty" json:"issues_events,omitempty"`
	MergeRequestChannel    *string `url:"merge_request_channel,omitempty" json:"merge_request_channel,omitempty"`
	MergeRequestsEvents    *bool   `url:"merge_requests_events,omitempty" json:"merge_requests_events,omitempty"`
	TagPushChannel         *string `url:"tag_push_channel,omitempty" json:"tag_push_channel,omitempty"`
	TagPushEvents          *bool   `url:"tag_push_events,omitempty" json:"tag_push_events,omitempty"`
	NoteChannel            *string `url:"note_channel,omitempty" json:"note_channel,omitempty"`
	NoteEvents             *bool   `url:"note_events,omitempty" json:"note_events,omitempty"`
	PipelineChannel        *string `url:"pipeline_channel,omitempty" json:"pipeline_channel,omitempty"`
	PipelineEvents         *bool   `url:"pipeline_events,omitempty" json:"pipeline_events,omitempty"`
	PushChannel            *string `url:"push_channel,omitempty" json:"push_channel,omitempty"`
	PushEvents             *bool   `url:"push_events,omitempty" json:"push_events,omitempty"`
	WikiPageChannel        *string `url:"wiki_page_channel,omitempty" json:"wiki_page_channel,omitempty"`
	WikiPageEvents         *bool   `url:"wiki_page_events,omitempty" json:"wiki_page_events,omitempty"`
}

// SetSlackService sets Slack service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-slack-service
func (s *ServicesService) SetSlackService(pid interface{}, opt *SetSlackServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteSlackService deletes Slack service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-slack-service
func (s *ServicesService) DeleteSlackService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/slack", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// JiraService represents Jira service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#jira
type JiraService struct {
	Service
	Properties *JiraServiceProperties `json:"properties"`
}

// JiraServiceProperties represents Jira specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#jira
type JiraServiceProperties struct {
	URL                   string `json:"url,omitempty"`
	APIURL                string `json:"api_url,omitempty"`
	ProjectKey            string `json:"project_key,omitempty" `
	Username              string `json:"username,omitempty" `
	Password              string `json:"password,omitempty" `
	JiraIssueTransitionID string `json:"jira_issue_transition_id,omitempty"`
}

// UnmarshalJSON decodes the Jira Service Properties.
//
// This allows support of JiraIssueTransitionID for both type string (>11.9) and float64 (<11.9)
func (p *JiraServiceProperties) UnmarshalJSON(b []byte) error {
	type Alias JiraServiceProperties
	raw := struct {
		*Alias
		JiraIssueTransitionID interface{} `json:"jira_issue_transition_id"`
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	switch id := raw.JiraIssueTransitionID.(type) {
	case nil:
		// No action needed.
	case string:
		p.JiraIssueTransitionID = id
	case float64:
		p.JiraIssueTransitionID = strconv.Itoa(int(id))
	default:
		return fmt.Errorf("failed to unmarshal JiraTransitionID of type: %T", id)
	}

	return nil
}

// GetJiraService gets Jira service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-jira-service-settings
func (s *ServicesService) GetJiraService(pid interface{}, options ...OptionFunc) (*JiraService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(JiraService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetJiraServiceOptions represents the available SetJiraService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-jira-service
type SetJiraServiceOptions struct {
	URL                   *string `url:"url,omitempty" json:"url,omitempty"`
	APIURL                *string `url:"api_url,omitempty" json:"api_url,omitempty"`
	ProjectKey            *string `url:"project_key,omitempty" json:"project_key,omitempty" `
	Username              *string `url:"username,omitempty" json:"username,omitempty" `
	Password              *string `url:"password,omitempty" json:"password,omitempty" `
	JiraIssueTransitionID *string `url:"jira_issue_transition_id,omitempty" json:"jira_issue_transition_id,omitempty"`
}

// SetJiraService sets Jira service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#edit-jira-service
func (s *ServicesService) SetJiraService(pid interface{}, opt *SetJiraServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteJiraService deletes Jira service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-jira-service
func (s *ServicesService) DeleteJiraService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jira", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// JenkinsCIService represents Jenkins CI service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type JenkinsCIService struct {
	Service
	Properties *JenkinsCIServiceProperties `json:"properties"`
}

// JenkinsCIServiceProperties represents Jenkins CI specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type JenkinsCIServiceProperties struct {
	URL         string `json:"jenkins_url,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Username    string `json:"username,omitempty"`
}

// GetJenkinsCIService gets Jenkins CI service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#get-jenkins-ci-service-settings
func (s *ServicesService) GetJenkinsCIService(pid interface{}, options ...OptionFunc) (*JenkinsCIService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(JenkinsCIService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetJenkinsCIServiceOptions represents the available SetJenkinsCIService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#jenkins-ci
type SetJenkinsCIServiceOptions struct {
	URL         *string `url:"jenkins_url,omitempty" json:"jenkins_url,omitempty"`
	ProjectName *string `url:"project_name,omitempty" json:"project_name,omitempty"`
	Username    *string `url:"username,omitempty" json:"username,omitempty"`
	Password    *string `url:"password,omitempty" json:"password,omitempty"`
}

// SetJenkinsCIService sets Jenkins service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/services.html#create-edit-jenkins-ci-service
func (s *ServicesService) SetJenkinsCIService(pid interface{}, opt *SetJenkinsCIServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteJenkinsCIService deletes Jenkins CI service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-jira-service
func (s *ServicesService) DeleteJenkinsCIService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/jenkins", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// MicrosoftTeamsService represents Microsoft Teams service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#microsoft-teams
type MicrosoftTeamsService struct {
	Service
	Properties *MicrosoftTeamsServiceProperties `json:"properties"`
}

// MicrosoftTeamsServiceProperties represents Microsoft Teams specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#microsoft-teams
type MicrosoftTeamsServiceProperties struct {
	WebHook string `json:"webhook"`
}

// GetMicrosoftTeamsService gets MicrosoftTeams service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-microsoft-teams-service-settings
func (s *ServicesService) GetMicrosoftTeamsService(pid interface{}, options ...OptionFunc) (*MicrosoftTeamsService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/microsoft-teams", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(MicrosoftTeamsService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetMicrosoftTeamsServiceOptions represents the available SetMicrosoftTeamsService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#create-edit-microsoft-teams-service
type SetMicrosoftTeamsServiceOptions struct {
	WebHook *string `url:"webhook,omitempty" json:"webhook,omitempty"`
}

// SetMicrosoftTeamsService sets Microsoft Teams service for a project
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#create-edit-microsoft-teams-service
func (s *ServicesService) SetMicrosoftTeamsService(pid interface{}, opt *SetMicrosoftTeamsServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/microsoft-teams", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}

// DeleteMicrosoftTeamsService deletes Microsoft Teams service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-microsoft-teams-service
func (s *ServicesService) DeleteMicrosoftTeamsService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/microsoft-teams", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ExternalWikiService represents External Wiki service settings.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#external-wiki
type ExternalWikiService struct {
	Service
	Properties *ExternalWikiServiceProperties `json:"properties"`
}

// ExternalWikiServiceProperties represents External Wiki specific properties.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#external-wiki
type ExternalWikiServiceProperties struct {
	ExternalWikiURL string `json:"external_wiki_url"`
}

// GetExternalWikiService gets External Wiki service settings for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#get-external-wiki-service-settings
func (s *ServicesService) GetExternalWikiService(pid interface{}, options ...OptionFunc) (*ExternalWikiService, *Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("projects/%s/services/external-wiki", pathEscape(project))

	req, err := s.client.NewRequest("GET", u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	svc := new(ExternalWikiService)
	resp, err := s.client.Do(req, svc)
	if err != nil {
		return nil, resp, err
	}

	return svc, resp, err
}

// SetExternalWikiServiceOptions represents the available SetExternalWikiService()
// options.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-external-wiki-service
type SetExternalWikiServiceOptions struct {
	ExternalWikiURL *string `url:"external_wiki_url,omitempty" json:"external_wiki_url,omitempty"`
}

// SetExternalWikiService sets External Wiki service for a project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#createedit-external-wiki-service
func (s *ServicesService) SetExternalWikiService(pid interface{}, opt *SetExternalWikiServiceOptions, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/external-wiki", pathEscape(project))

	req, err := s.client.NewRequest("PUT", u, opt, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteExternalWikiService deletes External Wiki service for project.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/services.html#delete-external-wiki-service
func (s *ServicesService) DeleteExternalWikiService(pid interface{}, options ...OptionFunc) (*Response, error) {
	project, err := parseID(pid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("projects/%s/services/external-wiki", pathEscape(project))

	req, err := s.client.NewRequest("DELETE", u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
