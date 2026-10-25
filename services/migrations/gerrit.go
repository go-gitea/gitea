// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
)

var (
	_ base.Downloader        = &GerritDownloader{}
	_ base.DownloaderFactory = &GerritDownloaderFactory{}
)

const (
	gerritMaxPageSize     = 100
	gerritMaxResponseSize = 64 * 1024 * 1024
	gerritMaxAttempts     = 5
	gerritMaxRetryWait    = time.Minute
	gerritMaxPathDepth    = 8
	gerritXSSIPrefix      = ")]}'"
)

func init() {
	RegisterDownloaderFactory(&GerritDownloaderFactory{})
}

// GerritDownloaderFactory defines a downloader factory for Gerrit Code Review
type GerritDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GerritDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	location, err := parseGerritCloneAddr(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	username, password := opts.AuthUsername, opts.AuthPassword
	if location.username != "" && username == "" {
		username, password = location.username, cmp.Or(password, location.password)
	}
	log.Trace("Create Gerrit downloader. Host: %s Path: %s", location.host, location.path)
	return NewGerritDownloader(ctx, location, username, password, opts.AuthToken), nil
}

// GitServiceType returns the type of git service
func (f *GerritDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GerritService
}

// ValidateGerritCloneAddr checks that the Gerrit REST API can be reached from the clone address of a project
func ValidateGerritCloneAddr(cloneAddr string) error {
	_, err := parseGerritCloneAddr(cloneAddr)
	return err
}

// gerritLocation is the parsed HTTP(S) clone URL of a Gerrit project, without credentials
type gerritLocation struct {
	scheme   string
	host     string
	path     []string // path segments, the "/a/" authentication prefix and ".git" suffix included as given
	username string
	password string
}

func (l *gerritLocation) String() string {
	return (&url.URL{Scheme: l.scheme, Host: l.host, Path: "/" + strings.Join(l.path, "/")}).String()
}

var errSSHNotSupported = errors.New("SSH clone URLs are not supported for Gerrit, the address of its REST API cannot be derived from them, use the HTTP(S) clone URL of the project")

func parseGerritCloneAddr(rawURL string) (*gerritLocation, error) {
	rawURL = strings.TrimSpace(rawURL)
	if !strings.Contains(rawURL, "://") {
		return nil, errSSHNotSupported
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, errors.New("invalid Gerrit clone URL")
	}
	if u.Scheme == "ssh" {
		return nil, errSSHNotSupported
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q for Gerrit, the address of its REST API can only be derived from an HTTP(S) clone URL of the project", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, errors.New("the Gerrit clone URL has no host")
	}

	location := &gerritLocation{scheme: u.Scheme, host: u.Host}
	if u.User != nil {
		location.username = u.User.Username()
		location.password, _ = u.User.Password()
	}
	trimmed := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	if trimmed == "" {
		return nil, fmt.Errorf("the Gerrit clone URL %q has no project name", location)
	}
	location.path = strings.Split(trimmed, "/")
	if len(location.path) > gerritMaxPathDepth {
		return nil, fmt.Errorf("the Gerrit clone URL %q has too many path segments", location)
	}
	for _, seg := range location.path {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("the Gerrit clone URL %q has an invalid path", location)
		}
	}
	return location, nil
}

// gerritCandidate is a way to split a clone URL path into the root of the Gerrit web application and a project name
type gerritCandidate struct {
	root    string
	project string
}

// candidates lists the possible splits of the path, as the length of the project name is not part of the URL.
// A segment "a" right after the root is the prefix of authenticated requests, not a part of the project.
func (l *gerritLocation) candidates() []gerritCandidate {
	var result []gerritCandidate
	for i := range l.path {
		root := (&url.URL{Scheme: l.scheme, Host: l.host, Path: "/" + strings.Join(l.path[:i], "/")}).String()
		root = strings.TrimSuffix(root, "/")
		rest := l.path[i:]
		if rest[0] == "a" && len(rest) > 1 {
			result = append(result, gerritCandidate{root: root, project: strings.Join(rest[1:], "/")})
		}
		result = append(result, gerritCandidate{root: root, project: strings.Join(rest, "/")})
	}
	return result
}

// GerritDownloader implements a Downloader for the REST API of Gerrit Code Review.
// A Gerrit change is a pull request, its patch sets are fetched through refs/changes with the repository.
// Gerrit has no issues, wiki, labels, milestones or releases.
type GerritDownloader struct {
	base.NullDownloader
	client   *http.Client
	sleep    func(context.Context, time.Duration) error
	location *gerritLocation
	username string
	password string
	token    string

	resolved bool
	root     string // URL of the Gerrit web application, without the authentication prefix
	project  string

	changeNumbers []int64 // all changes of the project, ascending
	numbersLoaded bool

	activities map[int64]*gerritActivity // review data by change number, of the current page
}

// NewGerritDownloader creates a Gerrit downloader
func NewGerritDownloader(_ context.Context, location *gerritLocation, username, password, token string) *GerritDownloader {
	return &GerritDownloader{
		client:     &http.Client{Transport: getMigrationTransport()}, // not retrying, callAPI retries rate limits itself
		sleep:      gerritSleep,
		activities: map[int64]*gerritActivity{},
		location:   location,
		username:   username,
		password:   password,
		token:      token,
	}
}

func (d *GerritDownloader) isAuthenticated() bool {
	user, password := d.credentials()
	return user != "" || password != ""
}

// credentials are used for REST and git alike, a token is a Gerrit HTTP password
func (d *GerritDownloader) credentials() (user, password string) {
	if d.token != "" {
		return cmp.Or(d.username, "token"), d.token
	}
	return d.username, d.password
}

// String implements Stringer
func (d *GerritDownloader) String() string {
	return fmt.Sprintf("migration from gerrit %s", d.location)
}

// LogString returns a log friendly description
func (d *GerritDownloader) LogString() string {
	if d == nil {
		return "<GerritDownloader nil>"
	}
	return fmt.Sprintf("<GerritDownloader %s>", d.location)
}

// FormatCloneURL adds authentication into the remote URL
func (d *GerritDownloader) FormatCloneURL(_ base.MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	if d.isAuthenticated() {
		u.User = url.UserPassword(d.credentials())
	}
	return u.String(), nil
}

type gerritTime struct {
	time.Time
}

func (t *gerritTime) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", text, time.UTC)
	if err != nil {
		return fmt.Errorf("invalid Gerrit timestamp %q", text)
	}
	t.Time = parsed
	return nil
}

type gerritAccount struct {
	ID       int64  `json:"_account_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (a gerritAccount) displayName() string {
	return cmp.Or(a.Name, a.Username, a.Email, "gerrit-user-"+strconv.FormatInt(a.ID, 10))
}

type gerritCommit struct {
	Parents []struct {
		Commit string `json:"commit"`
	} `json:"parents"`
	Message string `json:"message"`
}

type gerritRevision struct {
	Number  int          `json:"_number"`
	Ref     string       `json:"ref"`
	Created gerritTime   `json:"created"`
	Commit  gerritCommit `json:"commit"`
}

type gerritMessage struct {
	ID             string        `json:"id"`
	Author         gerritAccount `json:"author"`
	Date           gerritTime    `json:"date"`
	Message        string        `json:"message"`
	Tag            string        `json:"tag"`
	RevisionNumber int           `json:"_revision_number"`
}

type gerritChangeInfo struct {
	Project         string                     `json:"project"`
	Branch          string                     `json:"branch"`
	ChangeID        string                     `json:"change_id"`
	Subject         string                     `json:"subject"`
	Status          string                     `json:"status"`
	Created         gerritTime                 `json:"created"`
	Updated         gerritTime                 `json:"updated"`
	Submitted       gerritTime                 `json:"submitted"`
	Owner           gerritAccount              `json:"owner"`
	Number          int64                      `json:"_number"`
	WorkInProgress  bool                       `json:"work_in_progress"`
	CurrentRevision string                     `json:"current_revision"`
	Revisions       map[string]*gerritRevision `json:"revisions"`
	Messages        []gerritMessage            `json:"messages"`
}

type gerritCommentInfo struct {
	ID         string        `json:"id"`
	PatchSet   int           `json:"patch_set"`
	Line       int           `json:"line"`
	InReplyTo  string        `json:"in_reply_to"`
	Message    string        `json:"message"`
	Updated    gerritTime    `json:"updated"`
	Author     gerritAccount `json:"author"`
	Side       string        `json:"side"`
	Parent     int           `json:"parent"`
	Unresolved *bool         `json:"unresolved"`
	Range      *struct {
		StartLine int `json:"start_line"`
		EndLine   int `json:"end_line"`
	} `json:"range"`
}

// gerritPatchSet is a revision of a change, fetched with the repository through its ref
type gerritPatchSet struct {
	Number  int
	SHA     string
	Ref     string
	Created time.Time
}

// gerritChange is a validated change, it is the downloader context of the pull request
type gerritChange struct {
	gerritChangeInfo
	patchSets []gerritPatchSet // ascending
	current   gerritPatchSet
}

func (c *gerritChange) patchSet(number int) *gerritPatchSet {
	for i := range c.patchSets {
		if c.patchSets[i].Number == number {
			return &c.patchSets[i]
		}
	}
	return nil
}

func gerritChangeRef(changeNumber int64, patchSet int) string {
	return fmt.Sprintf("refs/changes/%02d/%d/%d", changeNumber%100, changeNumber, patchSet)
}

type gerritStatusError struct {
	Status int
	URL    string
}

func (e *gerritStatusError) isAuthError() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

func (e *gerritStatusError) Error() string {
	msg := fmt.Sprintf("gerrit API %s returned status %d", e.URL, e.Status)
	if e.isAuthError() {
		msg += ", check the credentials and the permission to read the project"
	}
	return msg
}

type gerritDecodeError struct {
	URL string
	Err error
}

func (e *gerritDecodeError) Error() string {
	return fmt.Sprintf("gerrit API %s returned an invalid response: %v", e.URL, e.Err)
}

func (e *gerritDecodeError) Unwrap() error { return e.Err }

func (d *GerritDownloader) apiRoot() string {
	if d.isAuthenticated() {
		return d.root + "/a"
	}
	return d.root
}

func (d *GerritDownloader) projectPath() string {
	return "/projects/" + url.PathEscape(d.project)
}

func (d *GerritDownloader) changePath(number int64) string {
	return "/changes/" + url.PathEscape(d.project) + "~" + strconv.FormatInt(number, 10)
}

func (d *GerritDownloader) cloneURL() string {
	segments := strings.Split(d.project, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return d.apiRoot() + "/" + strings.Join(segments, "/")
}

func (d *GerritDownloader) changeURL(number int64) string {
	return d.root + "/c/" + d.project + "/+/" + strconv.FormatInt(number, 10)
}

// callAPI sends a GET request to the REST API of a resolved root, a rate limited or unavailable
// server is asked again after the time it names
func (d *GerritDownloader) callAPI(ctx context.Context, root, path string, query url.Values, result any) error {
	endpoint := root + path
	apiURL := endpoint
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}

	resp, err := d.doRequest(ctx, apiURL)
	for attempt := 1; err == nil && isGerritRetryable(resp.StatusCode) && attempt < gerritMaxAttempts; attempt++ {
		wait := gerritRetryAfter(resp.Header.Get("Retry-After"), attempt)
		resp.Body.Close()
		log.Warn("Gerrit answered the request to %s with a temporary error, retrying in %s", endpoint, wait)
		if err := d.sleep(ctx, wait); err != nil {
			return err
		}
		resp, err = d.doRequest(ctx, apiURL)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &gerritStatusError{Status: resp.StatusCode, URL: endpoint}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, gerritMaxResponseSize+1))
	if err != nil {
		return err
	}
	if len(data) > gerritMaxResponseSize {
		return errors.New("gerrit API response is too large")
	}
	data = bytes.TrimPrefix(bytes.TrimSpace(data), []byte(gerritXSSIPrefix))
	if err := json.Unmarshal(data, result); err != nil {
		return &gerritDecodeError{URL: endpoint, Err: err}
	}
	return nil
}

func isGerritRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (d *GerritDownloader) doRequest(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if d.isAuthenticated() {
		req.SetBasicAuth(d.credentials())
	}
	return d.client.Do(req)
}

func (d *GerritDownloader) get(ctx context.Context, path string, query url.Values, result any) error {
	if err := d.resolve(ctx); err != nil {
		return err
	}
	return d.callAPI(ctx, d.apiRoot(), path, query, result)
}

// resolve finds out which part of the clone URL path is the project by asking the server
func (d *GerritDownloader) resolve(ctx context.Context) error {
	if d.resolved {
		return nil
	}
	for _, candidate := range d.location.candidates() {
		var info struct {
			Name string `json:"name"`
		}
		apiRoot := candidate.root
		if d.isAuthenticated() {
			apiRoot += "/a"
		}
		err := d.callAPI(ctx, apiRoot, "/projects/"+url.PathEscape(candidate.project), nil, &info)
		if statusErr, ok := errors.AsType[*gerritStatusError](err); ok && statusErr.Status == http.StatusNotFound {
			continue
		}
		if _, ok := errors.AsType[*gerritDecodeError](err); ok {
			continue // another web application, such as an HTML page, answers on this path
		}
		if err != nil {
			return err
		}
		if info.Name != candidate.project {
			continue
		}
		d.root, d.project, d.resolved = candidate.root, candidate.project, true
		return nil
	}
	return fmt.Errorf("no Gerrit project found for %s, check the clone URL and the permission to read the project", d.location)
}

func gerritRetryAfter(header string, attempt int) time.Duration {
	wait := time.Duration(1<<(attempt-1)) * time.Second
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		wait = time.Duration(seconds) * time.Second
	}
	return min(wait, gerritMaxRetryWait)
}

func gerritSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// GetRepoInfo returns the information of the project
func (d *GerritDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	if err := d.resolve(ctx); err != nil {
		return nil, err
	}
	var project struct {
		Description string `json:"description"`
	}
	if err := d.get(ctx, d.projectPath(), nil, &project); err != nil {
		return nil, err
	}

	// an empty project has no HEAD to resolve and the API answers 404
	var head string
	if err := d.get(ctx, d.projectPath()+"/HEAD", nil, &head); err != nil {
		if statusErr, ok := errors.AsType[*gerritStatusError](err); !ok || statusErr.Status != http.StatusNotFound {
			return nil, err
		}
	}

	return &base.Repository{
		Name:          d.project[strings.LastIndex(d.project, "/")+1:],
		Owner:         "",
		IsPrivate:     d.isAuthenticated(),
		Description:   project.Description,
		CloneURL:      d.cloneURL(),
		OriginalURL:   d.root + "/admin/repos/" + url.PathEscape(d.project),
		DefaultBranch: strings.TrimPrefix(head, "refs/heads/"),
	}, nil
}

func (d *GerritDownloader) loadChangeNumbers(ctx context.Context) error {
	if d.numbersLoaded {
		return nil
	}
	seen := map[int64]bool{}
	var numbers []int64
	query := url.Values{"q": {"project:" + strconv.Quote(d.project)}, "n": {strconv.Itoa(gerritMaxPageSize)}}
	for start := 0; ; {
		query.Set("S", strconv.Itoa(start))
		var page []struct {
			Number      int64 `json:"_number"`
			MoreChanges bool  `json:"_more_changes"`
		}
		if err := d.get(ctx, "/changes/", query, &page); err != nil {
			return err
		}
		for _, change := range page {
			if change.Number <= 0 {
				return fmt.Errorf("gerrit returned a change with the invalid number %d", change.Number)
			}
			if !seen[change.Number] {
				seen[change.Number] = true
				numbers = append(numbers, change.Number)
			}
		}
		start += len(page)
		if len(page) == 0 || !page[len(page)-1].MoreChanges {
			break
		}
	}
	slices.Sort(numbers)
	d.changeNumbers, d.numbersLoaded = numbers, true
	return nil
}

// GetPullRequests returns a page of changes, oldest first
func (d *GerritDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	d.activities = map[int64]*gerritActivity{}
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = gerritMaxPageSize
	}
	if page == 1 {
		d.numbersLoaded = false
	}
	if err := d.loadChangeNumbers(ctx); err != nil {
		return nil, false, err
	}

	start := min((page-1)*perPage, len(d.changeNumbers))
	end := min(start+perPage, len(d.changeNumbers))
	prs := make([]*base.PullRequest, 0, end-start)
	for _, number := range d.changeNumbers[start:end] {
		pr, err := d.getPullRequest(ctx, number)
		if err != nil {
			return nil, false, err
		}
		prs = append(prs, pr)
	}
	return prs, end >= len(d.changeNumbers), nil
}

func (d *GerritDownloader) getPullRequest(ctx context.Context, number int64) (*base.PullRequest, error) {
	var info gerritChangeInfo
	query := url.Values{"o": {"ALL_REVISIONS", "CURRENT_REVISION", "ALL_COMMITS", "MESSAGES", "DETAILED_ACCOUNTS"}}
	if err := d.get(ctx, d.changePath(number), query, &info); err != nil {
		return nil, err
	}
	change, err := d.validateChange(number, &info)
	if err != nil {
		return nil, err
	}
	return d.convertChange(change)
}

func (d *GerritDownloader) validateChange(number int64, info *gerritChangeInfo) (*gerritChange, error) {
	if info.Number != number {
		return nil, fmt.Errorf("gerrit returned change %d for the request of change %d", info.Number, number)
	}
	if info.Project != d.project {
		return nil, fmt.Errorf("gerrit change %d belongs to the project %q, not %q", number, info.Project, d.project)
	}
	if info.Subject == "" || info.Created.IsZero() || info.Updated.IsZero() {
		return nil, fmt.Errorf("gerrit change %d has no subject or no timestamps", number)
	}
	info.Branch = strings.TrimPrefix(info.Branch, "refs/heads/")
	if info.Branch == "" || !git.IsValidRefPattern(git.BranchPrefix+info.Branch) {
		return nil, fmt.Errorf("gerrit change %d has the invalid target branch %q", number, info.Branch)
	}

	change := &gerritChange{gerritChangeInfo: *info}
	for sha, revision := range info.Revisions {
		if revision == nil || revision.Number <= 0 {
			return nil, fmt.Errorf("gerrit change %d has a revision without patch set number", number)
		}
		if !git.IsStringValidObjectID(nil, sha) {
			return nil, fmt.Errorf("gerrit change %d has the invalid revision %q", number, sha)
		}
		ref := gerritChangeRef(number, revision.Number)
		if revision.Ref != "" && revision.Ref != ref {
			return nil, fmt.Errorf("gerrit change %d reports the unexpected ref %q for patch set %d", number, revision.Ref, revision.Number)
		}
		if change.patchSet(revision.Number) != nil {
			return nil, fmt.Errorf("gerrit change %d has the patch set %d twice", number, revision.Number)
		}
		change.patchSets = append(change.patchSets, gerritPatchSet{Number: revision.Number, SHA: sha, Ref: ref, Created: revision.Created.Time})
	}
	slices.SortFunc(change.patchSets, func(a, b gerritPatchSet) int { return cmp.Compare(a.Number, b.Number) })

	currentRevision, ok := info.Revisions[info.CurrentRevision]
	if !ok {
		return nil, fmt.Errorf("gerrit change %d has no valid current patch set", number)
	}
	change.current = *change.patchSet(currentRevision.Number)
	return change, nil
}

func (d *GerritDownloader) convertChange(change *gerritChange) (*base.PullRequest, error) {
	// the target branch tip moves on after a merge, the parent of the patch set is the base the reviewers saw
	baseSHA := ""
	if parents := change.Revisions[change.current.SHA].Commit.Parents; len(parents) > 0 {
		baseSHA = parents[0].Commit
	}
	branch := func(ref, sha string) base.PullRequestBranch {
		return base.PullRequestBranch{Ref: ref, SHA: sha, RepoName: d.project}
	}

	pr := &base.PullRequest{
		Number:       change.Number,
		ForeignIndex: change.Number,
		Title:        change.Subject,
		Content:      d.changeDescription(change),
		PosterID:     change.Owner.ID,
		PosterName:   change.Owner.displayName(),
		PosterEmail:  change.Owner.Email,
		State:        "open",
		Created:      change.Created.Time,
		Updated:      change.Updated.Time,
		IsDraft:      change.WorkInProgress || change.Status == "DRAFT",
		Head:         branch(change.current.Ref, change.current.SHA),
		Base:         branch(change.Branch, baseSHA),
		Context:      change,
	}

	switch change.Status {
	case "NEW", "DRAFT":
	case "MERGED":
		merged := cmp.Or(change.Submitted.Time, change.Updated.Time)
		pr.State, pr.Merged, pr.MergedTime, pr.Closed = "closed", true, &merged, &merged
		pr.MergeCommitSHA = change.current.SHA
	case "ABANDONED":
		closed := change.Updated.Time
		pr.State, pr.Closed = "closed", &closed
	default:
		return nil, fmt.Errorf("gerrit change %d has the unknown status %q", change.Number, change.Status)
	}

	if !CheckAndEnsureSafePR(pr, d.root, d) {
		return nil, fmt.Errorf("gerrit change %d has an unsafe ref or revision", change.Number)
	}
	return pr, nil
}

func (d *GerritDownloader) changeDescription(change *gerritChange) string {
	message := change.Revisions[change.current.SHA].Commit.Message
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(message), change.Subject))
	footer := fmt.Sprintf("Imported from Gerrit change %s\nChange-Id: %s\nBranch: %s\nStatus: %s",
		d.changeURL(change.Number), change.ChangeID, change.Branch, change.Status)
	if body == "" {
		return footer
	}
	return body + "\n\n---\n" + footer
}

// gerritActivity is what the messages and comments of a change are converted to
type gerritActivity struct {
	number   int64
	reviews  []*base.Review
	comments []*base.Comment
}

func (d *GerritDownloader) getActivity(ctx context.Context, foreignIndex int64, downloaderContext base.DownloaderContext) (*gerritActivity, error) {
	if activity, ok := d.activities[foreignIndex]; ok {
		return activity, nil
	}
	change, ok := downloaderContext.(*gerritChange)
	if !ok || change.Number != foreignIndex {
		return nil, fmt.Errorf("the downloader context of gerrit change %d is missing", foreignIndex)
	}
	var byPath map[string][]gerritCommentInfo
	if err := d.get(ctx, d.changePath(foreignIndex)+"/comments", nil, &byPath); err != nil {
		return nil, err
	}
	activity := buildGerritActivity(change, byPath)
	d.activities[foreignIndex] = activity
	return activity, nil
}

// GetComments returns the messages without a patch set and the inline comments that cannot be anchored
func (d *GerritDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	activity, err := d.getActivity(ctx, commentable.GetForeignIndex(), commentable.GetContext())
	if err != nil {
		return nil, false, err
	}
	for _, comment := range activity.comments {
		comment.IssueIndex = commentable.GetLocalIndex()
	}
	return activity.comments, true, nil
}

// GetReviews returns a review for each message of a patch set, with the inline comments anchored to the current patch set
func (d *GerritDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	activity, err := d.getActivity(ctx, reviewable.GetForeignIndex(), reviewable.GetContext())
	if err != nil {
		return nil, err
	}
	for _, review := range activity.reviews {
		review.IssueIndex = reviewable.GetLocalIndex()
	}
	return activity.reviews, nil
}

type gerritVote struct {
	label string
	value int
}

// parseGerritVotes reads the header of a message of a patch set, "Patch Set 3: Code-Review+1 Verified-1"
func parseGerritVotes(message string) (patchSet int, votes []gerritVote) {
	header, _, _ := strings.Cut(message, "\n")
	rest, ok := strings.CutPrefix(header, "Patch Set ")
	if !ok {
		return 0, nil
	}
	number, labels, _ := strings.Cut(rest, ":")
	patchSet, err := strconv.Atoi(number)
	if err != nil {
		return 0, nil
	}
	for token := range strings.FieldsSeq(labels) {
		cut := strings.LastIndexAny(token, "+-")
		if cut <= 0 {
			continue
		}
		value, err := strconv.Atoi(token[cut:])
		if err != nil {
			continue
		}
		votes = append(votes, gerritVote{label: token[:cut], value: value})
	}
	return patchSet, votes
}

func gerritReviewState(votes []gerritVote) string {
	state := base.ReviewStateCommented
	for _, vote := range votes {
		if vote.label != "Code-Review" {
			continue
		}
		switch {
		case vote.value > 0:
			state = base.ReviewStateApproved
		case vote.value < 0:
			state = base.ReviewStateChangesRequested
		}
	}
	return state
}

// warnUnmappedGerritVotes reports the votes that are not Code-Review, they only stay in the text of the review
func warnUnmappedGerritVotes(changeNumber int64, patchSet int, votes []gerritVote) {
	for _, vote := range votes {
		if vote.label != "Code-Review" {
			log.Warn("Gerrit change %d has the vote label %s with the value %+d on patch set %d which cannot be mapped to a review state, it is kept in the text of the review",
				changeNumber, strconv.Quote(vote.label), vote.value, patchSet)
		}
	}
}

// isSafeGerritPath tells whether a path of a comment can be used as the tree path of a code comment
func isSafeGerritPath(path string) bool {
	if path == "" || len(path) > 4096 || !utf8.ValidString(path) || strings.HasPrefix(path, "/") {
		return false
	}
	for _, r := range path {
		if unicode.IsControl(r) || r == '`' || r == '\\' {
			return false
		}
	}
	for seg := range strings.SplitSeq(path, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

func describeGerritPath(number int64, path string) string {
	if isSafeGerritPath(path) || path == "/COMMIT_MSG" || path == "/MERGE_LIST" {
		return "`" + path + "`"
	}
	log.Warn("Gerrit change %d has a comment on a path that is not safe to be used as a file path", number)
	return strconv.QuoteToGraphic(path)
}

type gerritReviewKey struct {
	accountID int64
	patchSet  int
	at        int64
}

func buildGerritActivity(change *gerritChange, byPath map[string][]gerritCommentInfo) *gerritActivity {
	activity := &gerritActivity{number: change.Number}
	reviewsByKey := map[gerritReviewKey]*base.Review{}

	for _, message := range change.Messages {
		at := cmp.Or(message.Date.Time, change.Updated.Time)
		headerPatchSet, votes := parseGerritVotes(message.Message)
		patchSet := change.patchSet(cmp.Or(message.RevisionNumber, headerPatchSet))
		warnUnmappedGerritVotes(change.Number, headerPatchSet, votes)
		autogenerated := strings.HasPrefix(message.Tag, "autogenerated:")
		if patchSet == nil || (autogenerated && len(votes) == 0) {
			activity.comments = append(activity.comments, &base.Comment{
				PosterID:    message.Author.ID,
				PosterName:  message.Author.displayName(),
				PosterEmail: message.Author.Email,
				Created:     at,
				Updated:     at,
				Content:     message.Message,
			})
			continue
		}
		review := &base.Review{
			ReviewerID:   message.Author.ID,
			ReviewerName: message.Author.displayName(),
			CommitID:     patchSet.SHA,
			Content:      message.Message,
			CreatedAt:    at,
			State:        gerritReviewState(votes),
		}
		activity.reviews = append(activity.reviews, review)
		reviewsByKey[gerritReviewKey{message.Author.ID, patchSet.Number, at.Unix()}] = review
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		for i := range byPath[path] {
			activity.addComment(change, path, &byPath[path][i], reviewsByKey)
		}
	}

	slices.SortStableFunc(activity.reviews, func(a, b *base.Review) int { return a.CreatedAt.Compare(b.CreatedAt) })
	slices.SortStableFunc(activity.comments, func(a, b *base.Comment) int { return a.Created.Compare(b.Created) })
	return activity
}

func (a *gerritActivity) addComment(change *gerritChange, path string, c *gerritCommentInfo, reviewsByKey map[gerritReviewKey]*base.Review) {
	at := cmp.Or(c.Updated.Time, change.Updated.Time)
	text := cmp.Or(c.Message, "(no text)")
	patchSet := change.patchSet(c.PatchSet)

	// the diff of a pull request is the current patch set against its parent
	anchored := patchSet != nil && patchSet.Number == change.current.Number && isSafeGerritPath(path) &&
		c.Line > 0 && c.Parent == 0
	if anchored {
		line := c.Line
		if c.Side == "PARENT" {
			line = -line
		}
		if c.Range != nil && c.Range.StartLine > 0 && c.Range.StartLine < c.Range.EndLine {
			text = fmt.Sprintf("Lines %d-%d:\n\n%s", c.Range.StartLine, c.Range.EndLine, text)
		}
		reviewComment := &base.ReviewComment{
			Content:   text,
			TreePath:  path,
			Line:      line,
			CommitID:  patchSet.SHA,
			PosterID:  c.Author.ID,
			CreatedAt: at,
			UpdatedAt: at,
		}
		key := gerritReviewKey{c.Author.ID, patchSet.Number, at.Unix()}
		review, ok := reviewsByKey[key]
		if !ok {
			review = &base.Review{
				ReviewerID:   c.Author.ID,
				ReviewerName: c.Author.displayName(),
				CommitID:     patchSet.SHA,
				CreatedAt:    at,
				State:        base.ReviewStateCommented,
			}
			reviewsByKey[key] = review
			a.reviews = append(a.reviews, review)
		}
		review.Comments = append(review.Comments, reviewComment)
		return
	}

	var where strings.Builder
	if path == "/PATCHSET_LEVEL" {
		where.WriteString("Patch set level comment")
	} else {
		where.WriteString("Comment on " + describeGerritPath(change.Number, path))
		switch {
		case c.Range != nil && c.Range.StartLine > 0 && c.Range.StartLine < c.Range.EndLine:
			fmt.Fprintf(&where, " lines %d-%d", c.Range.StartLine, c.Range.EndLine)
		case c.Line > 0:
			fmt.Fprintf(&where, " line %d", c.Line)
		}
		if c.Side == "PARENT" || c.Parent > 0 {
			where.WriteString(" of the parent")
		}
	}
	if patchSet != nil {
		fmt.Fprintf(&where, " (patch set %d, revision %s)", patchSet.Number, patchSet.SHA)
	} else {
		fmt.Fprintf(&where, " (unknown patch set %d)", c.PatchSet)
	}
	if c.InReplyTo != "" {
		where.WriteString(", reply to comment " + strconv.Quote(c.InReplyTo))
	}
	a.comments = append(a.comments, &base.Comment{
		PosterID:    c.Author.ID,
		PosterName:  c.Author.displayName(),
		PosterEmail: c.Author.Email,
		Created:     at,
		Updated:     at,
		Content:     where.String() + ":\n\n" + text,
	})
}
