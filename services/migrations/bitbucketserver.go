// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
)

var (
	_ base.Downloader        = &BitbucketServerDownloader{}
	_ base.DownloaderFactory = &BitbucketServerDownloaderFactory{}
)

const (
	bitbucketServerMaxPageSize     = 100
	bitbucketServerMaxResponseSize = 64 * 1024 * 1024
	bitbucketServerMaxAttempts     = 5
	bitbucketServerMaxRetryWait    = time.Minute
)

func init() {
	RegisterDownloaderFactory(&BitbucketServerDownloaderFactory{})
}

// BitbucketServerDownloaderFactory defines a downloader factory for Bitbucket Server (Data Center)
type BitbucketServerDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *BitbucketServerDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	baseURL, projectKey, repoSlug, err := parseBitbucketServerURL(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	log.Trace("Create Bitbucket Server downloader. BaseURL: %s Project: %s Repo: %s", baseURL, projectKey, repoSlug)
	return NewBitbucketServerDownloader(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, projectKey, repoSlug), nil
}

// GitServiceType returns the type of git service
func (f *BitbucketServerDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.BitbucketServerService
}

// parseBitbucketServerURL splits a clone or browse URL into the base URL (with context path), project key and repo slug
func parseBitbucketServerURL(rawURL string) (baseURL, projectKey, repoSlug string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", "", fmt.Errorf("unsupported scheme %q, an http(s) URL is required", u.Scheme)
	}

	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, seg := range segs {
		rest := segs[i+1:]
		switch {
		case seg == "scm" && len(rest) >= 2:
			projectKey, repoSlug = rest[0], rest[1]
		case seg == "projects" && len(rest) >= 3 && rest[1] == "repos":
			projectKey, repoSlug = rest[0], rest[2]
		case seg == "users" && len(rest) >= 3 && rest[1] == "repos":
			projectKey, repoSlug = "~"+rest[0], rest[2]
		default:
			continue
		}
		repoSlug = strings.TrimSuffix(repoSlug, ".git")
		if projectKey == "" || repoSlug == "" {
			break
		}
		root := url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/" + strings.Join(segs[:i], "/")}
		return strings.TrimSuffix(root.String(), "/"), projectKey, repoSlug, nil
	}
	return "", "", "", fmt.Errorf("invalid Bitbucket Server URL %q, expected .../scm/<project>/<repo>.git or .../projects/<project>/repos/<repo>", u.Redacted())
}

// BitbucketServerDownloader implements a Downloader for Bitbucket Server (Data Center) REST API 1.0.
// Bitbucket Server has no issues, wiki, labels, milestones or releases.
type BitbucketServerDownloader struct {
	base.NullDownloader
	client     *http.Client
	sleep      func(context.Context, time.Duration) error
	baseURL    string
	projectKey string
	repoSlug   string
	username   string
	password   string
	token      string

	tokenUser   string // user name git over HTTP needs together with the token as the basic-auth password
	prNextStart int    // offset of the next page of pull requests, pages are fetched in order

	// activities of the PRs on the last fetched page, shared by comments, reviews and merge info
	activities map[int64][]*bbsActivity
}

// NewBitbucketServerDownloader creates a Bitbucket Server downloader
func NewBitbucketServerDownloader(_ context.Context, baseURL, username, password, token, projectKey, repoSlug string) *BitbucketServerDownloader {
	return &BitbucketServerDownloader{
		client:     &http.Client{Transport: getMigrationTransport()}, // not retrying, callAPI retries rate limits itself
		sleep:      bitbucketServerSleep,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		projectKey: projectKey,
		repoSlug:   repoSlug,
		username:   username,
		password:   password,
		token:      token,
		activities: map[int64][]*bbsActivity{},
	}
}

// String implements Stringer
func (d *BitbucketServerDownloader) String() string {
	return fmt.Sprintf("migration from bitbucket server %s %s/%s", d.baseURL, d.projectKey, d.repoSlug)
}

// LogString returns a log friendly description
func (d *BitbucketServerDownloader) LogString() string {
	if d == nil {
		return "<BitbucketServerDownloader nil>"
	}
	return fmt.Sprintf("<BitbucketServerDownloader %s %s/%s>", d.baseURL, d.projectKey, d.repoSlug)
}

// FormatCloneURL adds authentication into the remote URL
func (d *BitbucketServerDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	switch {
	case opts.AuthToken != "":
		// HTTP access tokens are accepted as the basic-auth password of their owner
		u.User = url.UserPassword(cmp.Or(opts.AuthUsername, d.tokenUser, "token"), opts.AuthToken)
	case opts.AuthUsername != "" || opts.AuthPassword != "":
		u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
	}
	return u.String(), nil
}

type bbsUser struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	EmailAddress string `json:"emailAddress"`
}

func (u bbsUser) login() string {
	return cmp.Or(u.Name, u.Slug)
}

type bbsRepository struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Project     struct {
		Key string `json:"key"`
	} `json:"project"`
}

type bbsRef struct {
	DisplayID    string         `json:"displayId"`
	LatestCommit string         `json:"latestCommit"`
	Repository   *bbsRepository `json:"repository"`
}

type bbsPullRequest struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"` // OPEN, MERGED, DECLINED
	Draft       bool   `json:"draft"`
	CreatedDate int64  `json:"createdDate"`
	UpdatedDate int64  `json:"updatedDate"`
	ClosedDate  int64  `json:"closedDate"`
	FromRef     bbsRef `json:"fromRef"`
	ToRef       bbsRef `json:"toRef"`
	Author      struct {
		User bbsUser `json:"user"`
	} `json:"author"`
}

type bbsAnchor struct {
	Line     int    `json:"line"`
	LineType string `json:"lineType"` // ADDED, REMOVED, CONTEXT
	FileType string `json:"fileType"` // FROM, TO
	Path     string `json:"path"`
	SrcPath  string `json:"srcPath"`
	FromHash string `json:"fromHash"`
	ToHash   string `json:"toHash"`

	// set when the comment spans several lines, Line is then the last one
	MultilineMarker struct {
		StartLine int `json:"startLine"`
	} `json:"multilineMarker"`
	MultilineSpan struct {
		SrcStart int `json:"srcSpanStart"`
		SrcEnd   int `json:"srcSpanEnd"`
		DstStart int `json:"dstSpanStart"`
		DstEnd   int `json:"dstSpanEnd"`
	} `json:"multilineSpan"`
}

// lineRange returns the first and last line of a multi-line comment, in the numbering of the commented side
func (a *bbsAnchor) lineRange() (start, end int, ok bool) {
	start, end = a.MultilineSpan.DstStart, a.MultilineSpan.DstEnd
	if a.FileType == "FROM" {
		start, end = a.MultilineSpan.SrcStart, a.MultilineSpan.SrcEnd
	}
	if start == 0 || end == 0 {
		start, end = a.MultilineMarker.StartLine, a.Line
	}
	return start, end, start > 0 && start < end
}

func (a *bbsAnchor) isInline() bool {
	return a != nil && a.Path != "" && a.Line > 0
}

type bbsComment struct {
	ID          int64         `json:"id"`
	Text        string        `json:"text"`
	Author      bbsUser       `json:"author"`
	CreatedDate int64         `json:"createdDate"`
	UpdatedDate int64         `json:"updatedDate"`
	Comments    []*bbsComment `json:"comments"` // replies

	ThreadResolved bool `json:"threadResolved"`
}

func (c *bbsComment) updated() time.Time {
	return time.UnixMilli(cmp.Or(c.UpdatedDate, c.CreatedDate))
}

type bbsActivity struct {
	Action        string      `json:"action"` // COMMENTED, APPROVED, UNAPPROVED, REVIEWED, DECLINED, REOPENED, MERGED, ...
	CommentAction string      `json:"commentAction"`
	CreatedDate   int64       `json:"createdDate"`
	User          bbsUser     `json:"user"`
	Comment       *bbsComment `json:"comment"`
	CommentAnchor *bbsAnchor  `json:"commentAnchor"`
	Commit        *bbsCommit  `json:"commit"` // merge commit of a MERGED activity
}

type bbsCommit struct {
	ID      string `json:"id"`
	Parents []struct {
		ID string `json:"id"`
	} `json:"parents"`
}

type bbsPage[T any] struct {
	Values        []T  `json:"values"`
	IsLastPage    bool `json:"isLastPage"`
	NextPageStart int  `json:"nextPageStart"`
}

func (p *bbsPage[T]) isLast(start int) bool {
	return p.IsLastPage || len(p.Values) == 0 || p.NextPageStart <= start
}

type bbsStatusError struct {
	Status int
	URL    string
}

func (e *bbsStatusError) isAuthError() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

func (e *bbsStatusError) Error() string {
	msg := fmt.Sprintf("bitbucket server API %s returned status %d", e.URL, e.Status)
	if e.isAuthError() {
		msg += ", check the credentials and the permission to read the repository"
	}
	return msg
}

func (d *BitbucketServerDownloader) repoPath() string {
	return "/projects/" + url.PathEscape(d.projectKey) + "/repos/" + url.PathEscape(d.repoSlug)
}

func (d *BitbucketServerDownloader) cloneURL(projectKey, repoSlug string) string {
	return d.baseURL + "/scm/" + url.PathEscape(projectKey) + "/" + url.PathEscape(repoSlug) + ".git"
}

func (d *BitbucketServerDownloader) browseURL() string {
	if name, ok := strings.CutPrefix(d.projectKey, "~"); ok {
		return d.baseURL + "/users/" + url.PathEscape(name) + "/repos/" + url.PathEscape(d.repoSlug)
	}
	return d.baseURL + d.repoPath()
}

// callAPI sends a GET request and returns the response headers, a rate limited (429) request is repeated after the
// time the server asks for
func (d *BitbucketServerDownloader) callAPI(ctx context.Context, path string, query url.Values, result any) (http.Header, error) {
	endpoint := d.baseURL + "/rest/api/1.0" + path
	apiURL := endpoint
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}

	resp, err := d.doRequest(ctx, apiURL)
	for attempt := 1; err == nil && resp.StatusCode == http.StatusTooManyRequests && attempt < bitbucketServerMaxAttempts; attempt++ {
		wait := bitbucketServerRetryAfter(resp.Header.Get("Retry-After"), attempt)
		resp.Body.Close()
		log.Warn("Bitbucket Server rate limited the request to %s, retrying in %s", endpoint, wait)
		if err := d.sleep(ctx, wait); err != nil {
			return nil, err
		}
		resp, err = d.doRequest(ctx, apiURL)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &bbsStatusError{Status: resp.StatusCode, URL: endpoint}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bitbucketServerMaxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bitbucketServerMaxResponseSize {
		return nil, errors.New("bitbucket server API response is too large")
	}
	return resp.Header, json.Unmarshal(data, result)
}

func (d *BitbucketServerDownloader) doRequest(ctx context.Context, apiURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	} else if d.username != "" || d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}

	return d.client.Do(req)
}

// bitbucketServerGitUser returns the user name to clone with for the user name an API response reports.
// Project and repository access tokens act as a technical user, which git does not accept, but "x-token-auth".
func bitbucketServerGitUser(reported string) string {
	name, err := url.PathUnescape(reported)
	if err != nil || name == "" {
		return ""
	}
	if strings.HasPrefix(name, "access-token-user/") {
		return "x-token-auth"
	}
	return name
}

// bitbucketServerRetryAfter returns the time to wait for a rate limited request: the Retry-After header,
// otherwise an exponential backoff.
func bitbucketServerRetryAfter(header string, attempt int) time.Duration {
	wait := time.Duration(1<<(attempt-1)) * time.Second
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		wait = time.Duration(seconds) * time.Second
	}
	return min(wait, bitbucketServerMaxRetryWait)
}

func bitbucketServerSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func bbsGetPage[T any](ctx context.Context, d *BitbucketServerDownloader, path string, query url.Values, start, limit int) (*bbsPage[T], error) {
	q := url.Values{}
	maps.Copy(q, query)
	q.Set("start", strconv.Itoa(start))
	q.Set("limit", strconv.Itoa(limit))
	var page bbsPage[T]
	if _, err := d.callAPI(ctx, path, q, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func bbsGetAll[T any](ctx context.Context, d *BitbucketServerDownloader, path string, query url.Values) ([]T, error) {
	var all []T
	start := 0
	for {
		page, err := bbsGetPage[T](ctx, d, path, query, start, bitbucketServerMaxPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Values...)
		if page.isLast(start) {
			return all, nil
		}
		start = page.NextPageStart
	}
}

// GetRepoInfo returns repository information
func (d *BitbucketServerDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var repo bbsRepository
	header, err := d.callAPI(ctx, d.repoPath(), nil, &repo)
	if err != nil {
		return nil, err
	}
	if d.token != "" {
		d.tokenUser = bitbucketServerGitUser(header.Get("X-AUSERNAME"))
	}

	// an empty repository has no default branch and the API answers 404
	var branch struct {
		DisplayID string `json:"displayId"`
	}
	if _, err := d.callAPI(ctx, d.repoPath()+"/default-branch", nil, &branch); err != nil {
		if statusErr, ok := errors.AsType[*bbsStatusError](err); !ok || statusErr.Status != http.StatusNotFound {
			return nil, err
		}
	}

	return &base.Repository{
		Name:          repo.Slug,
		Owner:         repo.Project.Key,
		IsPrivate:     !repo.Public,
		Description:   repo.Description,
		CloneURL:      d.cloneURL(d.projectKey, d.repoSlug),
		OriginalURL:   d.browseURL(),
		DefaultBranch: branch.DisplayID,
	}, nil
}

// GetPullRequests returns a page of pull requests, oldest first
func (d *BitbucketServerDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if page <= 1 {
		d.prNextStart = 0
	}
	limit := min(perPage, bitbucketServerMaxPageSize)
	if limit <= 0 {
		limit = bitbucketServerMaxPageSize
	}
	// the server may cap the page size, so the offset is taken from the previous page
	start := d.prNextStart
	result, err := bbsGetPage[bbsPullRequest](ctx, d, d.repoPath()+"/pull-requests",
		url.Values{"state": {"ALL"}, "order": {"oldest"}}, start, limit)
	if err != nil {
		return nil, false, err
	}
	d.prNextStart = result.NextPageStart

	clear(d.activities)
	prs := make([]*base.PullRequest, 0, len(result.Values))
	for i := range result.Values {
		prs = append(prs, d.convertPullRequest(ctx, &result.Values[i]))
	}
	return prs, result.isLast(start), nil
}

func (d *BitbucketServerDownloader) convertPullRequest(ctx context.Context, orig *bbsPullRequest) *base.PullRequest {
	pr := &base.PullRequest{
		Number:       orig.ID,
		ForeignIndex: orig.ID,
		Title:        orig.Title,
		Content:      orig.Description,
		PosterID:     orig.Author.User.ID,
		PosterName:   orig.Author.User.login(),
		PosterEmail:  orig.Author.User.EmailAddress,
		State:        "open",
		Created:      time.UnixMilli(orig.CreatedDate),
		Updated:      time.UnixMilli(orig.UpdatedDate),
		IsDraft:      orig.Draft,
		Merged:       orig.State == "MERGED",
		Head:         d.convertRef(&orig.FromRef),
		Base:         d.convertRef(&orig.ToRef),
	}
	// the merge base is computed by the uploader, the target branch tip given by the API may have moved on
	pr.Base.SHA = ""

	if orig.State != "OPEN" {
		pr.State = "closed"
		closed := time.UnixMilli(cmp.Or(orig.ClosedDate, orig.UpdatedDate))
		pr.Closed = &closed
		if pr.Merged {
			pr.MergedTime = &closed
		}
	}

	if pr.Merged {
		d.completeMergedPullRequest(ctx, pr)
	}

	_ = CheckAndEnsureSafePR(pr, d.baseURL, d)
	return pr
}

// completeMergedPullRequest adds the merge commit and the merge base, which the uploader cannot derive from
// the target branch as it already contains the merged head
func (d *BitbucketServerDownloader) completeMergedPullRequest(ctx context.Context, pr *base.PullRequest) {
	activities, err := d.getActivities(ctx, pr.Number)
	if err != nil {
		log.Warn("Bitbucket Server: unable to load activities of pull request %d: %v", pr.Number, err)
	}
	for _, activity := range activities {
		if activity.Action == "MERGED" && activity.Commit != nil {
			pr.MergeCommitSHA = activity.Commit.ID
		}
	}

	commits, err := bbsGetAll[bbsCommit](ctx, d, d.repoPath()+"/pull-requests/"+strconv.FormatInt(pr.Number, 10)+"/commits", nil)
	if err != nil {
		log.Warn("Bitbucket Server: unable to load commits of pull request %d: %v", pr.Number, err)
		return
	}
	// listed newest first
	if len(commits) > 0 && len(commits[len(commits)-1].Parents) > 0 {
		pr.Base.SHA = commits[len(commits)-1].Parents[0].ID
	}
}

func (d *BitbucketServerDownloader) convertRef(ref *bbsRef) base.PullRequestBranch {
	branch := base.PullRequestBranch{Ref: ref.DisplayID, SHA: ref.LatestCommit}
	if ref.Repository != nil {
		branch.OwnerName = ref.Repository.Project.Key
		branch.RepoName = ref.Repository.Slug
		// built from the user supplied base URL instead of trusting the links of the response
		branch.CloneURL = d.cloneURL(branch.OwnerName, branch.RepoName)
	}
	return branch
}

func (d *BitbucketServerDownloader) getActivities(ctx context.Context, prID int64) ([]*bbsActivity, error) {
	if activities, ok := d.activities[prID]; ok {
		return activities, nil
	}
	activities, err := bbsGetAll[*bbsActivity](ctx, d, d.repoPath()+"/pull-requests/"+strconv.FormatInt(prID, 10)+"/activities", nil)
	if err != nil {
		return nil, err
	}
	slices.Reverse(activities) // listed newest first
	d.activities[prID] = activities
	return activities, nil
}

// walkBitbucketServerComments visits each non-empty comment once, with the anchor of its thread.
// Deleted comments are skipped, also in the activity that added them.
func walkBitbucketServerComments(activities []*bbsActivity, visit func(c *bbsComment, anchor *bbsAnchor, isRoot bool)) {
	seen := map[int64]bool{}
	for _, activity := range activities {
		if activity.CommentAction == "DELETED" && activity.Comment != nil {
			seen[activity.Comment.ID] = true
		}
	}
	var walk func(c *bbsComment, anchor *bbsAnchor, isRoot bool)
	walk = func(c *bbsComment, anchor *bbsAnchor, isRoot bool) {
		if c == nil || seen[c.ID] {
			return
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Text) != "" {
			visit(c, anchor, isRoot)
		}
		for _, reply := range c.Comments {
			walk(reply, anchor, false)
		}
	}
	for _, activity := range activities {
		if activity.Action == "COMMENTED" {
			walk(activity.Comment, activity.CommentAnchor, true)
		}
	}
}

// GetComments returns the general and file comments and the decline/reopen events of a pull request
func (d *BitbucketServerDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	activities, err := d.getActivities(ctx, commentable.GetForeignIndex())
	if err != nil {
		return nil, false, err
	}

	var comments []*base.Comment
	walkBitbucketServerComments(activities, func(c *bbsComment, anchor *bbsAnchor, _ bool) {
		if anchor.isInline() {
			return
		}
		content := c.Text
		if anchor != nil && anchor.Path != "" {
			content = fmt.Sprintf("Comment on `%s`:\n\n%s", anchor.Path, c.Text)
		}
		comments = append(comments, &base.Comment{
			IssueIndex:  commentable.GetLocalIndex(),
			Index:       c.ID,
			PosterID:    c.Author.ID,
			PosterName:  c.Author.login(),
			PosterEmail: c.Author.EmailAddress,
			Created:     time.UnixMilli(c.CreatedDate),
			Updated:     c.updated(),
			Content:     content,
		})
	})
	for _, activity := range activities {
		var commentType string
		switch activity.Action {
		case "DECLINED":
			commentType = "close"
		case "REOPENED":
			commentType = "reopen"
		default:
			continue
		}
		comments = append(comments, &base.Comment{
			IssueIndex:  commentable.GetLocalIndex(),
			CommentType: commentType,
			PosterID:    activity.User.ID,
			PosterName:  activity.User.login(),
			PosterEmail: activity.User.EmailAddress,
			Created:     time.UnixMilli(activity.CreatedDate),
			Updated:     time.UnixMilli(activity.CreatedDate),
		})
	}
	slices.SortStableFunc(comments, func(a, b *base.Comment) int { return a.Created.Compare(b.Created) })
	return comments, true, nil
}

// GetReviews returns the approvals, "needs work" votes and the inline code comments of a pull request
func (d *BitbucketServerDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	activities, err := d.getActivities(ctx, reviewable.GetForeignIndex())
	if err != nil {
		return nil, err
	}

	var reviews []*base.Review
	walkBitbucketServerComments(activities, func(c *bbsComment, anchor *bbsAnchor, isRoot bool) {
		if !anchor.isInline() {
			return
		}
		// every comment becomes its own review so that replies keep their author
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   c.Author.ID,
			ReviewerName: c.Author.login(),
			CreatedAt:    time.UnixMilli(c.CreatedDate),
			State:        base.ReviewStateCommented,
			Comments:     []*base.ReviewComment{c.toReviewComment(anchor, isRoot)},
		})
	})

	lastVote := map[string]*bbsActivity{} // the last vote of every reviewer wins
	for _, activity := range activities {
		if activity.Action == "APPROVED" || activity.Action == "REVIEWED" || activity.Action == "UNAPPROVED" {
			lastVote[activity.User.login()] = activity
		}
	}
	for _, key := range slices.Sorted(maps.Keys(lastVote)) {
		activity := lastVote[key]
		if activity.Action == "UNAPPROVED" {
			continue
		}
		state := base.ReviewStateApproved
		if activity.Action == "REVIEWED" {
			state = base.ReviewStateChangesRequested
		}
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   activity.User.ID,
			ReviewerName: activity.User.login(),
			CreatedAt:    time.UnixMilli(activity.CreatedDate),
			State:        state,
		})
	}
	slices.SortStableFunc(reviews, func(a, b *base.Review) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return reviews, nil
}

// toReviewComment converts a comment of a code thread, isRoot marks the first one, which carries the resolved state and line range
func (c *bbsComment) toReviewComment(anchor *bbsAnchor, isRoot bool) *base.ReviewComment {
	rc := &base.ReviewComment{
		ID:        c.ID,
		Content:   c.Text,
		TreePath:  anchor.Path,
		Line:      anchor.Line,
		CommitID:  anchor.ToHash,
		PosterID:  c.Author.ID,
		CreatedAt: time.UnixMilli(c.CreatedDate),
		UpdatedAt: c.updated(),
		Resolved:  isRoot && c.ThreadResolved,
	}
	side := ""
	if anchor.FileType == "FROM" {
		side = " (before the change)"
		rc.Line = -anchor.Line // a negative line addresses the old side of the diff
		rc.CommitID = anchor.FromHash
	}
	// Gitea anchors a code comment to a single line only, so the range is kept in the text
	if start, end, ok := anchor.lineRange(); isRoot && ok {
		rc.Content = fmt.Sprintf("Lines %d-%d%s:\n\n%s", start, end, side, c.Text)
	}
	return rc
}
