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

// Package gitlab implements a GitLab API client.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"golang.org/x/oauth2"
)

const (
	defaultBaseURL = "https://gitlab.com/"
	apiVersionPath = "api/v4/"
	userAgent      = "go-gitlab"
)

// authType represents an authentication type within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type authType int

// List of available authentication types.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	basicAuth authType = iota
	oAuthToken
	privateToken
)

// AccessLevelValue represents a permission level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/permissions/permissions.html
type AccessLevelValue int

// List of available access levels
//
// GitLab API docs: https://docs.gitlab.com/ce/permissions/permissions.html
const (
	NoPermissions         AccessLevelValue = 0
	GuestPermissions      AccessLevelValue = 10
	ReporterPermissions   AccessLevelValue = 20
	DeveloperPermissions  AccessLevelValue = 30
	MaintainerPermissions AccessLevelValue = 40
	OwnerPermissions      AccessLevelValue = 50

	// These are deprecated and should be removed in a future version
	MasterPermissions AccessLevelValue = 40
	OwnerPermission   AccessLevelValue = 50
)

// BuildStateValue represents a GitLab build state.
type BuildStateValue string

// These constants represent all valid build states.
const (
	Pending  BuildStateValue = "pending"
	Running  BuildStateValue = "running"
	Success  BuildStateValue = "success"
	Failed   BuildStateValue = "failed"
	Canceled BuildStateValue = "canceled"
	Skipped  BuildStateValue = "skipped"
	Manual   BuildStateValue = "manual"
)

// ISOTime represents an ISO 8601 formatted date
type ISOTime time.Time

// ISO 8601 date format
const iso8601 = "2006-01-02"

// MarshalJSON implements the json.Marshaler interface
func (t ISOTime) MarshalJSON() ([]byte, error) {
	if y := time.Time(t).Year(); y < 0 || y >= 10000 {
		// ISO 8901 uses 4 digits for the years
		return nil, errors.New("json: ISOTime year outside of range [0,9999]")
	}

	b := make([]byte, 0, len(iso8601)+2)
	b = append(b, '"')
	b = time.Time(t).AppendFormat(b, iso8601)
	b = append(b, '"')

	return b, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (t *ISOTime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package
	if string(data) == "null" {
		return nil
	}

	isotime, err := time.Parse(`"`+iso8601+`"`, string(data))
	*t = ISOTime(isotime)

	return err
}

// EncodeValues implements the query.Encoder interface
func (t *ISOTime) EncodeValues(key string, v *url.Values) error {
	if t == nil || (time.Time(*t)).IsZero() {
		return nil
	}
	v.Add(key, t.String())
	return nil
}

// String implements the Stringer interface
func (t ISOTime) String() string {
	return time.Time(t).Format(iso8601)
}

// NotificationLevelValue represents a notification level.
type NotificationLevelValue int

// String implements the fmt.Stringer interface.
func (l NotificationLevelValue) String() string {
	return notificationLevelNames[l]
}

// MarshalJSON implements the json.Marshaler interface.
func (l NotificationLevelValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (l *NotificationLevelValue) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw := raw.(type) {
	case float64:
		*l = NotificationLevelValue(raw)
	case string:
		*l = notificationLevelTypes[raw]
	case nil:
		// No action needed.
	default:
		return fmt.Errorf("json: cannot unmarshal %T into Go value of type %T", raw, *l)
	}

	return nil
}

// List of valid notification levels.
const (
	DisabledNotificationLevel NotificationLevelValue = iota
	ParticipatingNotificationLevel
	WatchNotificationLevel
	GlobalNotificationLevel
	MentionNotificationLevel
	CustomNotificationLevel
)

var notificationLevelNames = [...]string{
	"disabled",
	"participating",
	"watch",
	"global",
	"mention",
	"custom",
}

var notificationLevelTypes = map[string]NotificationLevelValue{
	"disabled":      DisabledNotificationLevel,
	"participating": ParticipatingNotificationLevel,
	"watch":         WatchNotificationLevel,
	"global":        GlobalNotificationLevel,
	"mention":       MentionNotificationLevel,
	"custom":        CustomNotificationLevel,
}

// VisibilityValue represents a visibility level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type VisibilityValue string

// List of available visibility levels.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	PrivateVisibility  VisibilityValue = "private"
	InternalVisibility VisibilityValue = "internal"
	PublicVisibility   VisibilityValue = "public"
)

// VariableTypeValue represents a variable type within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type VariableTypeValue string

// List of available variable types.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	EnvVariableType  VariableTypeValue = "env_var"
	FileVariableType VariableTypeValue = "file"
)

// MergeMethodValue represents a project merge type within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html#project-merge-method
type MergeMethodValue string

// List of available merge type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html#project-merge-method
const (
	NoFastForwardMerge MergeMethodValue = "merge"
	FastForwardMerge   MergeMethodValue = "ff"
	RebaseMerge        MergeMethodValue = "rebase_merge"
)

// EventTypeValue represents actions type for contribution events
type EventTypeValue string

// List of available action type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/events.html#action-types
const (
	CreatedEventType   EventTypeValue = "created"
	UpdatedEventType   EventTypeValue = "updated"
	ClosedEventType    EventTypeValue = "closed"
	ReopenedEventType  EventTypeValue = "reopened"
	PushedEventType    EventTypeValue = "pushed"
	CommentedEventType EventTypeValue = "commented"
	MergedEventType    EventTypeValue = "merged"
	JoinedEventType    EventTypeValue = "joined"
	LeftEventType      EventTypeValue = "left"
	DestroyedEventType EventTypeValue = "destroyed"
	ExpiredEventType   EventTypeValue = "expired"
)

// EventTargetTypeValue represents actions type value for contribution events
type EventTargetTypeValue string

// List of available action type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/events.html#target-types
const (
	IssueEventTargetType        EventTargetTypeValue = "issue"
	MilestoneEventTargetType    EventTargetTypeValue = "milestone"
	MergeRequestEventTargetType EventTargetTypeValue = "merge_request"
	NoteEventTargetType         EventTargetTypeValue = "note"
	ProjectEventTargetType      EventTargetTypeValue = "project"
	SnippetEventTargetType      EventTargetTypeValue = "snippet"
	UserEventTargetType         EventTargetTypeValue = "user"
)

// A Client manages communication with the GitLab API.
type Client struct {
	// HTTP client used to communicate with the API.
	client *http.Client

	// Base URL for API requests. Defaults to the public GitLab API, but can be
	// set to a domain endpoint to use with a self hosted GitLab server. baseURL
	// should always be specified with a trailing slash.
	baseURL *url.URL

	// Token type used to make authenticated API calls.
	authType authType

	// Username and password used for basix authentication.
	username, password string

	// Token used to make authenticated API calls.
	token string

	// User agent used when communicating with the GitLab API.
	UserAgent string

	// Services used for talking to different parts of the GitLab API.
	AccessRequests        *AccessRequestsService
	AwardEmoji            *AwardEmojiService
	Boards                *IssueBoardsService
	Branches              *BranchesService
	BroadcastMessage      *BroadcastMessagesService
	CIYMLTemplate         *CIYMLTemplatesService
	Commits               *CommitsService
	ContainerRegistry     *ContainerRegistryService
	CustomAttribute       *CustomAttributesService
	DeployKeys            *DeployKeysService
	Deployments           *DeploymentsService
	Discussions           *DiscussionsService
	Environments          *EnvironmentsService
	Epics                 *EpicsService
	Events                *EventsService
	Features              *FeaturesService
	GitIgnoreTemplates    *GitIgnoreTemplatesService
	GroupBadges           *GroupBadgesService
	GroupCluster          *GroupClustersService
	GroupIssueBoards      *GroupIssueBoardsService
	GroupLabels           *GroupLabelsService
	GroupMembers          *GroupMembersService
	GroupMilestones       *GroupMilestonesService
	GroupVariables        *GroupVariablesService
	Groups                *GroupsService
	IssueLinks            *IssueLinksService
	Issues                *IssuesService
	Jobs                  *JobsService
	Keys                  *KeysService
	Labels                *LabelsService
	License               *LicenseService
	LicenseTemplates      *LicenseTemplatesService
	MergeRequestApprovals *MergeRequestApprovalsService
	MergeRequests         *MergeRequestsService
	Milestones            *MilestonesService
	Namespaces            *NamespacesService
	Notes                 *NotesService
	NotificationSettings  *NotificationSettingsService
	PagesDomains          *PagesDomainsService
	PipelineSchedules     *PipelineSchedulesService
	PipelineTriggers      *PipelineTriggersService
	Pipelines             *PipelinesService
	ProjectBadges         *ProjectBadgesService
	ProjectCluster        *ProjectClustersService
	ProjectImportExport   *ProjectImportExportService
	ProjectMembers        *ProjectMembersService
	ProjectSnippets       *ProjectSnippetsService
	ProjectVariables      *ProjectVariablesService
	Projects              *ProjectsService
	ProtectedBranches     *ProtectedBranchesService
	ProtectedTags         *ProtectedTagsService
	ReleaseLinks          *ReleaseLinksService
	Releases              *ReleasesService
	Repositories          *RepositoriesService
	RepositoryFiles       *RepositoryFilesService
	ResourceLabelEvents   *ResourceLabelEventsService
	Runners               *RunnersService
	Search                *SearchService
	Services              *ServicesService
	Settings              *SettingsService
	Sidekiq               *SidekiqService
	Snippets              *SnippetsService
	SystemHooks           *SystemHooksService
	Tags                  *TagsService
	Todos                 *TodosService
	Users                 *UsersService
	Validate              *ValidateService
	Version               *VersionService
	Wikis                 *WikisService
}

// ListOptions specifies the optional parameters to various List methods that
// support pagination.
type ListOptions struct {
	// For paginated result sets, page of results to retrieve.
	Page int `url:"page,omitempty" json:"page,omitempty"`

	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty" json:"per_page,omitempty"`
}

// NewClient returns a new GitLab API client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use API methods which require
// authentication, provide a valid private or personal token.
func NewClient(httpClient *http.Client, token string) *Client {
	client := newClient(httpClient)
	client.authType = privateToken
	client.token = token
	return client
}

// NewBasicAuthClient returns a new GitLab API client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use API methods which require
// authentication, provide a valid username and password.
func NewBasicAuthClient(httpClient *http.Client, endpoint, username, password string) (*Client, error) {
	client := newClient(httpClient)
	client.authType = basicAuth
	client.username = username
	client.password = password
	client.SetBaseURL(endpoint)

	err := client.requestOAuthToken(context.TODO())
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) requestOAuthToken(ctx context.Context) error {
	config := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s://%s/oauth/authorize", c.BaseURL().Scheme, c.BaseURL().Host),
			TokenURL: fmt.Sprintf("%s://%s/oauth/token", c.BaseURL().Scheme, c.BaseURL().Host),
		},
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.client)
	t, err := config.PasswordCredentialsToken(ctx, c.username, c.password)
	if err != nil {
		return err
	}
	c.token = t.AccessToken
	return nil
}

// NewOAuthClient returns a new GitLab API client. If a nil httpClient is
// provided, http.DefaultClient will be used. To use API methods which require
// authentication, provide a valid oauth token.
func NewOAuthClient(httpClient *http.Client, token string) *Client {
	client := newClient(httpClient)
	client.authType = oAuthToken
	client.token = token
	return client
}

func newClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	c := &Client{client: httpClient, UserAgent: userAgent}
	if err := c.SetBaseURL(defaultBaseURL); err != nil {
		// Should never happen since defaultBaseURL is our constant.
		panic(err)
	}

	// Create the internal timeStats service.
	timeStats := &timeStatsService{client: c}

	// Create all the public services.
	c.AccessRequests = &AccessRequestsService{client: c}
	c.AwardEmoji = &AwardEmojiService{client: c}
	c.Boards = &IssueBoardsService{client: c}
	c.Branches = &BranchesService{client: c}
	c.BroadcastMessage = &BroadcastMessagesService{client: c}
	c.CIYMLTemplate = &CIYMLTemplatesService{client: c}
	c.Commits = &CommitsService{client: c}
	c.ContainerRegistry = &ContainerRegistryService{client: c}
	c.CustomAttribute = &CustomAttributesService{client: c}
	c.DeployKeys = &DeployKeysService{client: c}
	c.Deployments = &DeploymentsService{client: c}
	c.Discussions = &DiscussionsService{client: c}
	c.Environments = &EnvironmentsService{client: c}
	c.Epics = &EpicsService{client: c}
	c.Events = &EventsService{client: c}
	c.Features = &FeaturesService{client: c}
	c.GitIgnoreTemplates = &GitIgnoreTemplatesService{client: c}
	c.GroupBadges = &GroupBadgesService{client: c}
	c.GroupCluster = &GroupClustersService{client: c}
	c.GroupIssueBoards = &GroupIssueBoardsService{client: c}
	c.GroupLabels = &GroupLabelsService{client: c}
	c.GroupMembers = &GroupMembersService{client: c}
	c.GroupMilestones = &GroupMilestonesService{client: c}
	c.GroupVariables = &GroupVariablesService{client: c}
	c.Groups = &GroupsService{client: c}
	c.IssueLinks = &IssueLinksService{client: c}
	c.Issues = &IssuesService{client: c, timeStats: timeStats}
	c.Jobs = &JobsService{client: c}
	c.Keys = &KeysService{client: c}
	c.Labels = &LabelsService{client: c}
	c.License = &LicenseService{client: c}
	c.LicenseTemplates = &LicenseTemplatesService{client: c}
	c.MergeRequestApprovals = &MergeRequestApprovalsService{client: c}
	c.MergeRequests = &MergeRequestsService{client: c, timeStats: timeStats}
	c.Milestones = &MilestonesService{client: c}
	c.Namespaces = &NamespacesService{client: c}
	c.Notes = &NotesService{client: c}
	c.NotificationSettings = &NotificationSettingsService{client: c}
	c.PagesDomains = &PagesDomainsService{client: c}
	c.PipelineSchedules = &PipelineSchedulesService{client: c}
	c.PipelineTriggers = &PipelineTriggersService{client: c}
	c.Pipelines = &PipelinesService{client: c}
	c.ProjectBadges = &ProjectBadgesService{client: c}
	c.ProjectCluster = &ProjectClustersService{client: c}
	c.ProjectImportExport = &ProjectImportExportService{client: c}
	c.ProjectMembers = &ProjectMembersService{client: c}
	c.ProjectSnippets = &ProjectSnippetsService{client: c}
	c.ProjectVariables = &ProjectVariablesService{client: c}
	c.Projects = &ProjectsService{client: c}
	c.ProtectedBranches = &ProtectedBranchesService{client: c}
	c.ProtectedTags = &ProtectedTagsService{client: c}
	c.ReleaseLinks = &ReleaseLinksService{client: c}
	c.Releases = &ReleasesService{client: c}
	c.Repositories = &RepositoriesService{client: c}
	c.RepositoryFiles = &RepositoryFilesService{client: c}
	c.ResourceLabelEvents = &ResourceLabelEventsService{client: c}
	c.Runners = &RunnersService{client: c}
	c.Search = &SearchService{client: c}
	c.Services = &ServicesService{client: c}
	c.Settings = &SettingsService{client: c}
	c.Sidekiq = &SidekiqService{client: c}
	c.Snippets = &SnippetsService{client: c}
	c.SystemHooks = &SystemHooksService{client: c}
	c.Tags = &TagsService{client: c}
	c.Todos = &TodosService{client: c}
	c.Users = &UsersService{client: c}
	c.Validate = &ValidateService{client: c}
	c.Version = &VersionService{client: c}
	c.Wikis = &WikisService{client: c}

	return c
}

// BaseURL return a copy of the baseURL.
func (c *Client) BaseURL() *url.URL {
	u := *c.baseURL
	return &u
}

// SetBaseURL sets the base URL for API requests to a custom endpoint. urlStr
// should always be specified with a trailing slash.
func (c *Client) SetBaseURL(urlStr string) error {
	// Make sure the given URL end with a slash
	if !strings.HasSuffix(urlStr, "/") {
		urlStr += "/"
	}

	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(baseURL.Path, apiVersionPath) {
		baseURL.Path += apiVersionPath
	}

	// Update the base URL of the client.
	c.baseURL = baseURL

	return nil
}

// NewRequest creates an API request. A relative URL path can be provided in
// urlStr, in which case it is resolved relative to the base URL of the Client.
// Relative URL paths should always be specified without a preceding slash. If
// specified, the value pointed to by body is JSON encoded and included as the
// request body.
func (c *Client) NewRequest(method, path string, opt interface{}, options []OptionFunc) (*http.Request, error) {
	u := *c.baseURL
	unescaped, err := url.PathUnescape(path)
	if err != nil {
		return nil, err
	}

	// Set the encoded path data
	u.RawPath = c.baseURL.Path + path
	u.Path = c.baseURL.Path + unescaped

	if opt != nil {
		q, err := query.Values(opt)
		if err != nil {
			return nil, err
		}
		u.RawQuery = q.Encode()
	}

	req := &http.Request{
		Method:     method,
		URL:        &u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       u.Host,
	}

	for _, fn := range options {
		if fn == nil {
			continue
		}

		if err := fn(req); err != nil {
			return nil, err
		}
	}

	if method == "POST" || method == "PUT" {
		bodyBytes, err := json.Marshal(opt)
		if err != nil {
			return nil, err
		}
		bodyReader := bytes.NewReader(bodyBytes)

		u.RawQuery = ""
		req.Body = ioutil.NopCloser(bodyReader)
		req.GetBody = func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bodyReader), nil
		}
		req.ContentLength = int64(bodyReader.Len())
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")

	switch c.authType {
	case basicAuth, oAuthToken:
		req.Header.Set("Authorization", "Bearer "+c.token)
	case privateToken:
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}

	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	return req, nil
}

// Response is a GitLab API response. This wraps the standard http.Response
// returned from GitLab and provides convenient access to things like
// pagination links.
type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of
	// results. Any or all of these may be set to the zero value for
	// responses that are not part of a paginated set, or for which there
	// are no additional pages.
	TotalItems   int
	TotalPages   int
	ItemsPerPage int
	CurrentPage  int
	NextPage     int
	PreviousPage int
}

// newResponse creates a new Response for the provided http.Response.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.populatePageValues()
	return response
}

const (
	xTotal      = "X-Total"
	xTotalPages = "X-Total-Pages"
	xPerPage    = "X-Per-Page"
	xPage       = "X-Page"
	xNextPage   = "X-Next-Page"
	xPrevPage   = "X-Prev-Page"
)

// populatePageValues parses the HTTP Link response headers and populates the
// various pagination link values in the Response.
func (r *Response) populatePageValues() {
	if totalItems := r.Response.Header.Get(xTotal); totalItems != "" {
		r.TotalItems, _ = strconv.Atoi(totalItems)
	}
	if totalPages := r.Response.Header.Get(xTotalPages); totalPages != "" {
		r.TotalPages, _ = strconv.Atoi(totalPages)
	}
	if itemsPerPage := r.Response.Header.Get(xPerPage); itemsPerPage != "" {
		r.ItemsPerPage, _ = strconv.Atoi(itemsPerPage)
	}
	if currentPage := r.Response.Header.Get(xPage); currentPage != "" {
		r.CurrentPage, _ = strconv.Atoi(currentPage)
	}
	if nextPage := r.Response.Header.Get(xNextPage); nextPage != "" {
		r.NextPage, _ = strconv.Atoi(nextPage)
	}
	if previousPage := r.Response.Header.Get(xPrevPage); previousPage != "" {
		r.PreviousPage, _ = strconv.Atoi(previousPage)
	}
}

// Do sends an API request and returns the API response. The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred. If v implements the io.Writer
// interface, the raw response body will be written to v, without attempting to
// first decode it.
func (c *Client) Do(req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && c.authType == basicAuth {
		err = c.requestOAuthToken(req.Context())
		if err != nil {
			return nil, err
		}
		return c.Do(req, v)
	}

	response := newResponse(resp)

	err = CheckResponse(resp)
	if err != nil {
		// even though there was an error, we still return the response
		// in case the caller wants to inspect it further
		return response, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			_, err = io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
	}

	return response, err
}

// Helper function to accept and format both the project ID or name as project
// identifier for all API calls.
func parseID(id interface{}) (string, error) {
	switch v := id.(type) {
	case int:
		return strconv.Itoa(v), nil
	case string:
		return v, nil
	default:
		return "", fmt.Errorf("invalid ID type %#v, the ID must be an int or a string", id)
	}
}

// Helper function to escape a project identifier.
func pathEscape(s string) string {
	return strings.Replace(url.PathEscape(s), ".", "%2E", -1)
}

// An ErrorResponse reports one or more errors caused by an API request.
//
// GitLab API docs:
// https://docs.gitlab.com/ce/api/README.html#data-validation-and-error-reporting
type ErrorResponse struct {
	Body     []byte
	Response *http.Response
	Message  string
}

func (e *ErrorResponse) Error() string {
	path, _ := url.QueryUnescape(e.Response.Request.URL.Path)
	u := fmt.Sprintf("%s://%s%s", e.Response.Request.URL.Scheme, e.Response.Request.URL.Host, path)
	return fmt.Sprintf("%s %s: %d %s", e.Response.Request.Method, u, e.Response.StatusCode, e.Message)
}

// CheckResponse checks the API response for errors, and returns them if present.
func CheckResponse(r *http.Response) error {
	switch r.StatusCode {
	case 200, 201, 202, 204, 304:
		return nil
	}

	errorResponse := &ErrorResponse{Response: r}
	data, err := ioutil.ReadAll(r.Body)
	if err == nil && data != nil {
		errorResponse.Body = data

		var raw interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			errorResponse.Message = "failed to parse unknown error format"
		} else {
			errorResponse.Message = parseError(raw)
		}
	}

	return errorResponse
}

// Format:
// {
//     "message": {
//         "<property-name>": [
//             "<error-message>",
//             "<error-message>",
//             ...
//         ],
//         "<embed-entity>": {
//             "<property-name>": [
//                 "<error-message>",
//                 "<error-message>",
//                 ...
//             ],
//         }
//     },
//     "error": "<error-message>"
// }
func parseError(raw interface{}) string {
	switch raw := raw.(type) {
	case string:
		return raw

	case []interface{}:
		var errs []string
		for _, v := range raw {
			errs = append(errs, parseError(v))
		}
		return fmt.Sprintf("[%s]", strings.Join(errs, ", "))

	case map[string]interface{}:
		var errs []string
		for k, v := range raw {
			errs = append(errs, fmt.Sprintf("{%s: %s}", k, parseError(v)))
		}
		sort.Strings(errs)
		return strings.Join(errs, ", ")

	default:
		return fmt.Sprintf("failed to parse unexpected error type: %T", raw)
	}
}

// OptionFunc can be passed to all API requests to make the API call as if you were
// another user, provided your private token is from an administrator account.
//
// GitLab docs: https://docs.gitlab.com/ce/api/README.html#sudo
type OptionFunc func(*http.Request) error

// WithSudo takes either a username or user ID and sets the SUDO request header
func WithSudo(uid interface{}) OptionFunc {
	return func(req *http.Request) error {
		user, err := parseID(uid)
		if err != nil {
			return err
		}
		req.Header.Set("SUDO", user)
		return nil
	}
}

// WithContext runs the request with the provided context
func WithContext(ctx context.Context) OptionFunc {
	return func(req *http.Request) error {
		*req = *req.WithContext(ctx)
		return nil
	}
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
func Bool(v bool) *bool {
	p := new(bool)
	*p = v
	return p
}

// Int is a helper routine that allocates a new int32 value
// to store v and returns a pointer to it, but unlike Int32
// its argument value is an int.
func Int(v int) *int {
	p := new(int)
	*p = v
	return p
}

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string {
	p := new(string)
	*p = v
	return p
}

// Time is a helper routine that allocates a new time.Time value
// to store v and returns a pointer to it.
func Time(v time.Time) *time.Time {
	p := new(time.Time)
	*p = v
	return p
}

// AccessLevel is a helper routine that allocates a new AccessLevelValue
// to store v and returns a pointer to it.
func AccessLevel(v AccessLevelValue) *AccessLevelValue {
	p := new(AccessLevelValue)
	*p = v
	return p
}

// BuildState is a helper routine that allocates a new BuildStateValue
// to store v and returns a pointer to it.
func BuildState(v BuildStateValue) *BuildStateValue {
	p := new(BuildStateValue)
	*p = v
	return p
}

// NotificationLevel is a helper routine that allocates a new NotificationLevelValue
// to store v and returns a pointer to it.
func NotificationLevel(v NotificationLevelValue) *NotificationLevelValue {
	p := new(NotificationLevelValue)
	*p = v
	return p
}

// VariableType is a helper routine that allocates a new VariableTypeValue
// to store v and returns a pointer to it.
func VariableType(v VariableTypeValue) *VariableTypeValue {
	p := new(VariableTypeValue)
	*p = v
	return p
}

// Visibility is a helper routine that allocates a new VisibilityValue
// to store v and returns a pointer to it.
func Visibility(v VisibilityValue) *VisibilityValue {
	p := new(VisibilityValue)
	*p = v
	return p
}

// MergeMethod is a helper routine that allocates a new MergeMethod
// to sotre v and returns a pointer to it.
func MergeMethod(v MergeMethodValue) *MergeMethodValue {
	p := new(MergeMethodValue)
	*p = v
	return p
}

// BoolValue is a boolean value with advanced json unmarshaling features.
type BoolValue bool

// UnmarshalJSON allows 1 and 0 to be considered as boolean values
// Needed for https://gitlab.com/gitlab-org/gitlab-ce/issues/50122
func (t *BoolValue) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"1"`:
		*t = true
		return nil
	case `"0"`:
		*t = false
		return nil
	default:
		var v bool
		err := json.Unmarshal(b, &v)
		*t = BoolValue(v)
		return err
	}
}
