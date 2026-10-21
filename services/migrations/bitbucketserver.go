// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
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

// parseBitbucketServerURL extracts the base URL (including an optional context path), the project key and the
// repository slug from a clone URL (".../scm/KEY/repo.git") or a browse URL (".../projects/KEY/repos/repo",
// ".../users/name/repos/repo").
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
// Bitbucket Server has no issue tracker, wiki, labels, milestones or releases, so only the repository
// (git data including tags and LFS) and pull requests (with comments and reviews) are migrated.
type BitbucketServerDownloader struct {
	base.NullDownloader
	client     *http.Client
	baseURL    string
	username   string
	password   string
	token      string
	tokenUser  string // owner of the token, git over HTTP needs it as the basic-auth user name
	projectKey string
	repoSlug   string

	// comments and reviews are both derived from the activities of a PR, which are also needed
	// to find the merge commit. They are cached for the PRs of the last downloaded page.
	activities map[int64][]*bbsActivity
}

// NewBitbucketServerDownloader creates a Bitbucket Server downloader
func NewBitbucketServerDownloader(_ context.Context, baseURL, username, password, token, projectKey, repoSlug string) *BitbucketServerDownloader {
	return &BitbucketServerDownloader{
		client:     getMigrationHTTPClient(),
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		username:   username,
		password:   password,
		token:      token,
		projectKey: projectKey,
		repoSlug:   repoSlug,
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

type bbsComment struct {
	ID          int64         `json:"id"`
	Text        string        `json:"text"`
	Author      bbsUser       `json:"author"`
	CreatedDate int64         `json:"createdDate"`
	UpdatedDate int64         `json:"updatedDate"`
	Comments    []*bbsComment `json:"comments"` // replies

	ThreadResolved bool `json:"threadResolved"`
}

type bbsActivity struct {
	Action        string      `json:"action"` // COMMENTED, APPROVED, UNAPPROVED, REVIEWED, DECLINED, REOPENED, MERGED, ...
	CommentAction string      `json:"commentAction"`
	CreatedDate   int64       `json:"createdDate"`
	User          bbsUser     `json:"user"`
	Comment       *bbsComment `json:"comment"`
	CommentAnchor *bbsAnchor  `json:"commentAnchor"`
	Commit        *struct {
		ID string `json:"id"`
	} `json:"commit"` // merge commit of a MERGED activity
}

type bbsPage[T any] struct {
	Values        []T  `json:"values"`
	IsLastPage    bool `json:"isLastPage"`
	NextPageStart int  `json:"nextPageStart"`
}

type bbsStatusError struct {
	Status int
	URL    string
}

func (e *bbsStatusError) Error() string {
	return fmt.Sprintf("bitbucket server API %s returned status %d", e.URL, e.Status)
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
	return d.baseURL + "/projects/" + url.PathEscape(d.projectKey) + "/repos/" + url.PathEscape(d.repoSlug)
}

func (d *BitbucketServerDownloader) callAPI(ctx context.Context, path string, query url.Values, result any) error {
	apiURL := d.baseURL + "/rest/api/1.0" + path
	if len(query) > 0 {
		apiURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if d.token != "" {
		req.Header.Set("Authorization", "Bearer "+d.token)
	} else if d.username != "" || d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if d.token != "" {
		d.tokenUser = cmp.Or(resp.Header.Get("X-AUSERNAME"), d.tokenUser)
	}
	if resp.StatusCode != http.StatusOK {
		return &bbsStatusError{Status: resp.StatusCode, URL: d.baseURL + "/rest/api/1.0" + path}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bitbucketServerMaxResponseSize+1))
	if err != nil {
		return err
	}
	if len(data) > bitbucketServerMaxResponseSize {
		return errors.New("bitbucket server API response is too large")
	}
	return json.Unmarshal(data, result)
}

func bbsGetPage[T any](ctx context.Context, d *BitbucketServerDownloader, path string, query url.Values, start, limit int) (*bbsPage[T], error) {
	q := url.Values{}
	for k, v := range query {
		q[k] = v
	}
	q.Set("start", strconv.Itoa(start))
	q.Set("limit", strconv.Itoa(limit))
	var page bbsPage[T]
	if err := d.callAPI(ctx, path, q, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// bbsGetAll follows the pagination until the last page
func bbsGetAll[T any](ctx context.Context, d *BitbucketServerDownloader, path string, query url.Values) ([]T, error) {
	var all []T
	start := 0
	for {
		page, err := bbsGetPage[T](ctx, d, path, query, start, bitbucketServerMaxPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Values...)
		if page.IsLastPage || len(page.Values) == 0 || page.NextPageStart <= start {
			return all, nil
		}
		start = page.NextPageStart
	}
}

// GetRepoInfo returns repository information
func (d *BitbucketServerDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var repo bbsRepository
	if err := d.callAPI(ctx, d.repoPath(), nil, &repo); err != nil {
		return nil, err
	}

	// an empty repository has no default branch and the API answers 404
	var branch struct {
		DisplayID string `json:"displayId"`
	}
	if err := d.callAPI(ctx, d.repoPath()+"/default-branch", nil, &branch); err != nil {
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
	limit := min(perPage, bitbucketServerMaxPageSize)
	if limit <= 0 {
		limit = bitbucketServerMaxPageSize
	}
	result, err := bbsGetPage[bbsPullRequest](ctx, d, d.repoPath()+"/pull-requests",
		url.Values{"state": {"ALL"}, "order": {"oldest"}}, max(page-1, 0)*limit, limit)
	if err != nil {
		return nil, false, err
	}

	clear(d.activities)
	prs := make([]*base.PullRequest, 0, len(result.Values))
	for i := range result.Values {
		pr := d.convertPullRequest(&result.Values[i])
		if pr.Merged {
			d.setMergeCommit(ctx, pr)
		}
		prs = append(prs, pr)
	}
	return prs, result.IsLastPage || len(result.Values) == 0, nil
}

func (d *BitbucketServerDownloader) convertPullRequest(orig *bbsPullRequest) *base.PullRequest {
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
	// Bitbucket keeps the head commit of every PR as refs/pull-requests/<id>/from in the target repository, so the
	// objects of a fork are already migrated. Fetching from the fork would need its own credentials, therefore
	// fork PRs are handed over as if their head lived in the target repository, under a name that cannot clash.
	if !strings.EqualFold(pr.Head.OwnerName, pr.Base.OwnerName) || !strings.EqualFold(pr.Head.RepoName, pr.Base.RepoName) {
		pr.Head.Ref = strings.TrimPrefix(pr.Head.OwnerName, "~") + "/" + pr.Head.Ref
		pr.Head.OwnerName, pr.Head.RepoName, pr.Head.CloneURL = pr.Base.OwnerName, pr.Base.RepoName, pr.Base.CloneURL
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

	_ = CheckAndEnsureSafePR(pr, d.baseURL, d)
	return pr
}

// setMergeCommit takes the merge commit from the MERGED activity, the pull request itself does not carry it
func (d *BitbucketServerDownloader) setMergeCommit(ctx context.Context, pr *base.PullRequest) {
	activities, err := d.getActivities(ctx, pr.Number)
	if err != nil {
		log.Warn("Bitbucket Server: unable to load activities of pull request %d: %v", pr.Number, err)
		return
	}
	for _, activity := range activities {
		if activity.Action == "MERGED" && activity.Commit != nil {
			pr.MergeCommitSHA = activity.Commit.ID
		}
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
	d.activities[prID] = activities
	return activities, nil
}

// walkComments visits a comment thread depth first, each comment only once. Replies do not carry an anchor
// themselves, so the anchor of the thread is handed down.
func walkBitbucketServerComments(c *bbsComment, anchor *bbsAnchor, seen map[int64]bool, visit func(*bbsComment, *bbsAnchor)) {
	if c == nil || seen[c.ID] {
		return
	}
	seen[c.ID] = true
	visit(c, anchor)
	for _, reply := range c.Comments {
		walkBitbucketServerComments(reply, anchor, seen, visit)
	}
}

func (a *bbsAnchor) isInline() bool {
	return a != nil && a.Path != "" && a.Line > 0
}

// GetComments returns the general comments of a pull request together with comments on whole files and decline/reopen events
func (d *BitbucketServerDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	activities, err := d.getActivities(ctx, commentable.GetForeignIndex())
	if err != nil {
		return nil, false, err
	}

	var comments []*base.Comment
	seen := map[int64]bool{}
	for _, activity := range activities {
		switch activity.Action {
		case "COMMENTED":
			if activity.CommentAction == "DELETED" {
				continue
			}
			walkBitbucketServerComments(activity.Comment, activity.CommentAnchor, seen, func(c *bbsComment, anchor *bbsAnchor) {
				if strings.TrimSpace(c.Text) == "" || anchor.isInline() {
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
					Updated:     time.UnixMilli(cmp.Or(c.UpdatedDate, c.CreatedDate)),
					Content:     content,
				})
			})
		case "DECLINED", "REOPENED":
			commentType := "close"
			if activity.Action == "REOPENED" {
				commentType = "reopen"
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
	lastVote := map[string]*bbsActivity{} // the last vote of every reviewer wins
	var voters []string
	seen := map[int64]bool{}
	for _, activity := range activities {
		switch activity.Action {
		case "APPROVED", "REVIEWED", "UNAPPROVED":
			key := activity.User.login()
			if _, ok := lastVote[key]; !ok {
				voters = append(voters, key)
			}
			lastVote[key] = activity
		case "COMMENTED":
			if activity.CommentAction == "DELETED" {
				continue
			}
			walkBitbucketServerComments(activity.Comment, activity.CommentAnchor, seen, func(c *bbsComment, anchor *bbsAnchor) {
				if strings.TrimSpace(c.Text) == "" || !anchor.isInline() {
					return
				}
				// every comment becomes its own review so that replies keep their author
				reviews = append(reviews, &base.Review{
					IssueIndex:   reviewable.GetLocalIndex(),
					ReviewerID:   c.Author.ID,
					ReviewerName: c.Author.login(),
					CreatedAt:    time.UnixMilli(c.CreatedDate),
					State:        base.ReviewStateCommented,
					Comments:     []*base.ReviewComment{convertBitbucketServerInlineComment(c, anchor, c == activity.Comment)},
				})
			})
		}
	}

	for _, key := range voters {
		activity := lastVote[key]
		state := base.ReviewStateApproved
		switch activity.Action {
		case "UNAPPROVED":
			continue
		case "REVIEWED":
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

// convertBitbucketServerInlineComment converts a comment of a code thread. isRoot tells whether it starts the thread,
// which is where the resolved state and the line range of the anchor belong to.
func convertBitbucketServerInlineComment(c *bbsComment, anchor *bbsAnchor, isRoot bool) *base.ReviewComment {
	rc := &base.ReviewComment{
		ID:        c.ID,
		Content:   c.Text,
		TreePath:  anchor.Path,
		Line:      anchor.Line,
		CommitID:  anchor.ToHash,
		PosterID:  c.Author.ID,
		CreatedAt: time.UnixMilli(c.CreatedDate),
		UpdatedAt: time.UnixMilli(cmp.Or(c.UpdatedDate, c.CreatedDate)),
		Resolved:  isRoot && c.ThreadResolved,
	}
	// Gitea anchors a code comment to a single line only, so the range is kept in the text
	if start, end, ok := anchor.lineRange(); isRoot && ok {
		side := ""
		if anchor.FileType == "FROM" {
			side = " (before the change)"
		}
		rc.Content = fmt.Sprintf("Lines %d-%d%s:\n\n%s", start, end, side, c.Text)
	}
	// a negative line addresses the old side of the diff
	if anchor.FileType == "FROM" {
		rc.Line = -anchor.Line
		rc.CommitID = anchor.FromHash
		if anchor.SrcPath != "" {
			rc.TreePath = anchor.SrcPath
		}
	}
	return rc
}
