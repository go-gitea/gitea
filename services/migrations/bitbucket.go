// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

var (
	_ base.Downloader        = &BitbucketDownloader{}
	_ base.DownloaderFactory = &BitbucketDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&BitbucketDownloaderFactory{})
}

// BitbucketDownloaderFactory defines a bitbucket.org downloader factory.
type BitbucketDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions.
func (f *BitbucketDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	fields := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid Bitbucket clone URL: %s", opts.CloneAddr)
	}

	workspace := fields[0]
	repoSlug := strings.TrimSuffix(fields[1], ".git")
	webBaseURL := u.Scheme + "://" + u.Host
	apiBaseURL := "https://api.bitbucket.org/2.0"

	return NewBitbucketDownloader(ctx, apiBaseURL, webBaseURL, workspace, repoSlug, opts.AuthUsername, opts.AuthPassword, opts.AuthToken)
}

// GitServiceType returns the type of git service.
func (f *BitbucketDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.BitbucketService
}

type bitbucketAPIError struct {
	StatusCode int
	Body       string
}

func (e *bitbucketAPIError) Error() string {
	if e.StatusCode == http.StatusTooManyRequests {
		return fmt.Sprintf("bitbucket API rate limit exceeded (status 429): %s; authenticate with a username and app password (or an access token) to raise the limit — anonymous access is capped at 60 requests/hour", e.Body)
	}
	return fmt.Sprintf("bitbucket API returned status %d: %s", e.StatusCode, e.Body)
}

const (
	// bitbucketMaxRetries bounds how many times a rate-limited or transiently failing
	// request is retried before giving up.
	bitbucketMaxRetries = 5
	// bitbucketDefaultRetryBackoff is the base delay used for exponential backoff when the
	// server does not tell us how long to wait.
	bitbucketDefaultRetryBackoff = 2 * time.Second
	// bitbucketMaxRetryBackoff caps a single wait so a migration never blocks unreasonably long.
	bitbucketMaxRetryBackoff = 60 * time.Second
	// bitbucketMaxRetryAfter caps an explicit Retry-After / rate-reset wait.
	bitbucketMaxRetryAfter = 1 * time.Hour
	// bitbucketResetEpochThreshold distinguishes an X-RateLimit-Reset expressed as seconds
	// remaining (Bitbucket Cloud's format) from one expressed as an absolute Unix epoch.
	bitbucketResetEpochThreshold = 1_000_000_000
)

type bitbucketPage[T any] struct {
	Values  []T    `json:"values"`
	Size    int    `json:"size"`
	Page    int    `json:"page"`
	Pagelen int    `json:"pagelen"`
	Next    string `json:"next"`
}

type bitbucketContent struct {
	Raw    string `json:"raw"`
	Markup string `json:"markup"`
}

type bitbucketUser struct {
	UUID        string `json:"uuid"`
	AccountID   string `json:"account_id"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

type bitbucketNamedValue struct {
	Name string `json:"name"`
}

type bitbucketRepoLink struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type bitbucketRepoLinks struct {
	HTML  bitbucketRepoLink   `json:"html"`
	Clone []bitbucketRepoLink `json:"clone"`
}

type bitbucketRepository struct {
	Name        string              `json:"name"`
	Slug        string              `json:"slug"`
	FullName    string              `json:"full_name"`
	Description string              `json:"description"`
	IsPrivate   bool                `json:"is_private"`
	Website     string              `json:"website"`
	Links       bitbucketRepoLinks  `json:"links"`
	MainBranch  bitbucketNamedValue `json:"mainbranch"`
}

type bitbucketIssue struct {
	ID        int64                `json:"id"`
	Title     string               `json:"title"`
	Content   bitbucketContent     `json:"content"`
	State     string               `json:"state"`
	Kind      string               `json:"kind"`
	Priority  string               `json:"priority"`
	Reporter  bitbucketUser        `json:"reporter"`
	Assignee  *bitbucketUser       `json:"assignee"`
	Component *bitbucketNamedValue `json:"component"`
	Version   *bitbucketNamedValue `json:"version"`
	Milestone *bitbucketNamedValue `json:"milestone"`
	Created   time.Time            `json:"created_on"`
	Updated   time.Time            `json:"updated_on"`
}

type bitbucketBranch struct {
	Name string `json:"name"`
}

type bitbucketCommit struct {
	Hash string `json:"hash"`
}

type bitbucketPRSide struct {
	Branch     bitbucketBranch      `json:"branch"`
	Commit     bitbucketCommit      `json:"commit"`
	Repository *bitbucketRepository `json:"repository"`
}

type bitbucketPullRequestLinks struct {
	HTML  bitbucketRepoLink `json:"html"`
	Patch bitbucketRepoLink `json:"patch"`
}

type bitbucketPullRequest struct {
	ID          int64                     `json:"id"`
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	State       string                    `json:"state"`
	Author      bitbucketUser             `json:"author"`
	Source      bitbucketPRSide           `json:"source"`
	Destination bitbucketPRSide           `json:"destination"`
	MergeCommit *bitbucketCommit          `json:"merge_commit"`
	Links       bitbucketPullRequestLinks `json:"links"`
	Created     time.Time                 `json:"created_on"`
	Updated     time.Time                 `json:"updated_on"`
}

type bitbucketComment struct {
	ID      int64            `json:"id"`
	Content bitbucketContent `json:"content"`
	User    bitbucketUser    `json:"user"`
	Created time.Time        `json:"created_on"`
	Updated time.Time        `json:"updated_on"`
}

// BitbucketDownloader implements a Downloader interface to get repository information from bitbucket.org.
type BitbucketDownloader struct {
	base.NullDownloader
	client     *http.Client
	apiBaseURL string
	webBaseURL string
	workspace  string
	repoSlug   string
	userName   string
	password   string
	token      string
	maxPerPage int
	maxRetries int
	maxIssueID int64
	allIssues  []bitbucketIssue
	issuesRead bool
	prIDFrozen bool
}

// NewBitbucketDownloader creates a bitbucket.org Downloader via API v2.
func NewBitbucketDownloader(_ context.Context, apiBaseURL, webBaseURL, workspace, repoSlug, userName, password, token string) (*BitbucketDownloader, error) {
	return &BitbucketDownloader{
		client:     newMigrationHTTPClient(),
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		webBaseURL: strings.TrimRight(webBaseURL, "/"),
		workspace:  workspace,
		repoSlug:   repoSlug,
		userName:   userName,
		password:   password,
		token:      token,
		maxPerPage: 100,
		maxRetries: bitbucketMaxRetries,
	}, nil
}

// String implements Stringer.
func (b *BitbucketDownloader) String() string {
	return fmt.Sprintf("migration from bitbucket server %s %s/%s", b.webBaseURL, b.workspace, b.repoSlug)
}

func (b *BitbucketDownloader) LogString() string {
	if b == nil {
		return "<BitbucketDownloader nil>"
	}
	return fmt.Sprintf("<BitbucketDownloader %s %s/%s>", b.webBaseURL, b.workspace, b.repoSlug)
}

// FormatCloneURL adds authentication into the clone URL. Bitbucket Cloud requires the
// username "x-token-auth" when cloning with an access token; app passwords use the normal
// username/password basic-auth form.
func (b *BitbucketDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	if opts.AuthToken != "" {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", err
		}
		u.User = url.UserPassword("x-token-auth", opts.AuthToken)
		return u.String(), nil
	}
	return b.NullDownloader.FormatCloneURL(opts, remoteAddr)
}

func (b *BitbucketDownloader) apiPath(format string, args ...any) string {
	parts := []any{url.PathEscape(b.workspace), url.PathEscape(b.repoSlug)}
	parts = append(parts, args...)
	return fmt.Sprintf("/repositories/%s/%s"+format, parts...)
}

func (b *BitbucketDownloader) doAPI(ctx context.Context, path string, query url.Values, out any) error {
	u, err := url.Parse(b.apiBaseURL + path)
	if err != nil {
		return err
	}
	u.RawQuery = query.Encode()

	// Bitbucket Cloud enforces per-hour rate limits and answers with HTTP 429 once they are
	// exceeded. Only idempotent GET requests are issued here, so it is safe to transparently
	// wait-and-retry on rate limiting and a handful of transient server-side failures.
	//
	// totalWait bounds the cumulative time a single doAPI call may spend sleeping across all
	// its retries. Each individual wait is already capped at bitbucketMaxRetryAfter, but a
	// per-attempt cap still allows several long sleeps to stack up (and this downloader is
	// additionally wrapped by base.NewRetryDownloader). Reusing bitbucketMaxRetryAfter as the
	// total budget ensures a single call fails fast instead of blocking for hours.
	var totalWait time.Duration
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if b.token != "" {
			req.Header.Set("Authorization", "Bearer "+b.token)
		} else if b.userName != "" || b.password != "" {
			req.SetBasicAuth(b.userName, b.password)
		}

		resp, err := b.client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			if strings.EqualFold(resp.Header.Get("X-RateLimit-NearLimit"), "true") {
				log.Trace("Bitbucket rate limit near threshold on %s", path)
			}
			err = json.NewDecoder(resp.Body).Decode(out)
			resp.Body.Close()
			return err
		}

		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			if attempt < b.maxRetries {
				wait := bitbucketRetryWait(resp, attempt)
				// Only retry while the cumulative sleep stays within the total budget; otherwise
				// fall through to drain the body and return the bitbucketAPIError (fail fast).
				if totalWait+wait <= bitbucketMaxRetryAfter {
					totalWait += wait
					// Drain and close the body so the underlying connection can be reused.
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()

					log.Warn("Bitbucket rate limited (status %d) on %s, waiting %s before retry %d/%d", resp.StatusCode, path, wait, attempt+1, b.maxRetries)

					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						return ctx.Err()
					case <-timer.C:
					}
					continue
				}
			}
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return &bitbucketAPIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
}

// bitbucketRetryWait decides how long to wait before retrying a rate-limited or transient
// response. It prefers the server-provided Retry-After header, then X-RateLimit-Reset, and
// finally falls back to exponential backoff with jitter. All waits are capped.
func bitbucketRetryWait(resp *http.Response, attempt int) time.Duration {
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil {
			return clampDuration(time.Duration(seconds)*time.Second, 0, bitbucketMaxRetryAfter)
		}
		if date, err := http.ParseTime(retryAfter); err == nil {
			return clampDuration(time.Until(date), 0, bitbucketMaxRetryAfter)
		}
	}

	if reset := strings.TrimSpace(resp.Header.Get("X-RateLimit-Reset")); reset != "" {
		var wait time.Duration
		if t, err := time.Parse(time.RFC3339, reset); err == nil {
			wait = time.Until(t)
		} else if v, err := strconv.ParseInt(reset, 10, 64); err == nil {
			// Bitbucket Cloud returns the number of seconds remaining until the rate-limit
			// window resets (a delta), e.g. "2959". Values large enough to be a Unix epoch
			// are treated as an absolute timestamp for robustness across Atlassian products.
			if v >= bitbucketResetEpochThreshold {
				wait = time.Until(time.Unix(v, 0))
			} else {
				wait = time.Duration(v) * time.Second
			}
		}
		if wait > 0 {
			return clampDuration(wait, 0, bitbucketMaxRetryAfter)
		}
	}

	// Exponential backoff with jitter, capped so a migration never blocks for too long.
	backoff := bitbucketDefaultRetryBackoff << attempt
	if backoff <= 0 || backoff > bitbucketMaxRetryBackoff {
		backoff = bitbucketMaxRetryBackoff
	}
	backoff += rand.N(time.Second)
	return clampDuration(backoff, time.Millisecond, bitbucketMaxRetryBackoff+time.Second)
}

// clampDuration keeps d within the inclusive [minWait, maxWait] range.
func clampDuration(d, minWait, maxWait time.Duration) time.Duration {
	if d < minWait {
		return minWait
	}
	if d > maxWait {
		return maxWait
	}
	return d
}

func bitbucketUserID(user bitbucketUser) int64 {
	identifier := user.AccountID
	if identifier == "" {
		identifier = user.UUID
	}
	if identifier == "" {
		identifier = bitbucketUserName(user)
	}
	if identifier == "" {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(identifier))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func bitbucketUserName(user bitbucketUser) string {
	for _, value := range []string{user.Nickname, user.Username, user.DisplayName, user.AccountID, user.UUID} {
		if value != "" {
			return value
		}
	}
	return "Ghost"
}

func bitbucketCloneURL(repo *bitbucketRepository) string {
	if repo == nil {
		return ""
	}
	for _, clone := range repo.Links.Clone {
		if clone.Name == "https" && clone.Href != "" {
			return clone.Href
		}
	}
	if len(repo.Links.Clone) > 0 {
		return repo.Links.Clone[0].Href
	}
	return ""
}

func bitbucketOwnerAndRepo(repo *bitbucketRepository) (string, string) {
	if repo == nil {
		return "", ""
	}
	owner, name, ok := strings.Cut(repo.FullName, "/")
	if ok {
		return owner, name
	}
	return "", repo.Slug
}

func bitbucketIssueState(state string) string {
	switch strings.ToLower(state) {
	case "resolved", "closed", "invalid", "duplicate", "wontfix":
		return "closed"
	default:
		return "open"
	}
}

func bitbucketPRState(state string) string {
	if strings.EqualFold(state, "open") {
		return "open"
	}
	return "closed"
}

func bitbucketClosedTime(state string, updated time.Time) *time.Time {
	if state == "closed" && !updated.IsZero() {
		return &updated
	}
	return nil
}

func bitbucketIssueLabels(issue bitbucketIssue) []*base.Label {
	labels := make([]*base.Label, 0, 4)
	for _, label := range []string{
		util.Iif(issue.Kind == "", "", "kind/"+issue.Kind),
		util.Iif(issue.Priority == "", "", "priority/"+issue.Priority),
		util.Iif(issue.Component == nil || issue.Component.Name == "", "", "component/"+issue.Component.Name),
		util.Iif(issue.Version == nil || issue.Version.Name == "", "", "version/"+issue.Version.Name),
	} {
		if label != "" {
			labels = append(labels, &base.Label{Name: label, Color: bitbucketLabelColor(label)})
		}
	}
	return labels
}

func bitbucketLabelColor(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return fmt.Sprintf("%06x", h.Sum32()&0xffffff)
}

func (b *BitbucketDownloader) recordIssueID(issueID int64) {
	if !b.prIDFrozen {
		b.maxIssueID = max(b.maxIssueID, issueID)
	}
}

// fetchAllIssues pages through the full /issues endpoint once and caches the result.
// A 404 means issues are disabled for the repository, which is treated as an empty set.
func (b *BitbucketDownloader) fetchAllIssues(ctx context.Context) ([]bitbucketIssue, error) {
	if b.issuesRead {
		return b.allIssues, nil
	}
	var all []bitbucketIssue
	for page := 1; ; page++ {
		var resp bitbucketPage[bitbucketIssue]
		query := url.Values{
			"pagelen": []string{strconv.Itoa(b.maxPerPage)},
			"page":    []string{strconv.Itoa(page)},
			"sort":    []string{"created_on"},
		}
		if err := b.doAPI(ctx, b.apiPath("/issues"), query, &resp); err != nil {
			if apiErr, ok := err.(*bitbucketAPIError); ok && apiErr.StatusCode == http.StatusNotFound {
				b.allIssues = []bitbucketIssue{}
				b.issuesRead = true
				return b.allIssues, nil
			}
			return nil, err
		}
		all = append(all, resp.Values...)
		if resp.Next == "" {
			break
		}
	}
	b.allIssues = all
	b.issuesRead = true
	for _, issue := range all {
		b.recordIssueID(issue.ID)
	}
	return all, nil
}

func (b *BitbucketDownloader) bitbucketPRNumber(ctx context.Context, prID int64) (int64, error) {
	if _, err := b.fetchAllIssues(ctx); err != nil {
		return 0, err
	}
	b.prIDFrozen = true
	return b.maxIssueID + prID, nil
}

// GetRepoInfo returns a repository information.
func (b *BitbucketDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var repo bitbucketRepository
	if err := b.doAPI(ctx, b.apiPath(""), nil, &repo); err != nil {
		return nil, err
	}

	owner, _ := bitbucketOwnerAndRepo(&repo)
	return &base.Repository{
		Owner:         owner,
		Name:          repo.Slug,
		IsPrivate:     repo.IsPrivate,
		Description:   repo.Description,
		Website:       repo.Website,
		OriginalURL:   repo.Links.HTML.Href,
		CloneURL:      bitbucketCloneURL(&repo),
		DefaultBranch: repo.MainBranch.Name,
	}, nil
}

// GetLabels returns labels synthesized from Bitbucket issue metadata.
func (b *BitbucketDownloader) GetLabels(ctx context.Context) ([]*base.Label, error) {
	allIssues, err := b.fetchAllIssues(ctx)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]*base.Label)
	for _, issue := range allIssues {
		for _, label := range bitbucketIssueLabels(issue) {
			labels[label.Name] = label
		}
	}

	result := make([]*base.Label, 0, len(labels))
	for _, label := range labels {
		result = append(result, label)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// GetMilestones returns milestones collected from Bitbucket issues.
func (b *BitbucketDownloader) GetMilestones(ctx context.Context) ([]*base.Milestone, error) {
	allIssues, err := b.fetchAllIssues(ctx)
	if err != nil {
		return nil, err
	}
	milestones := make(map[string]*base.Milestone)
	for _, issue := range allIssues {
		if issue.Milestone != nil && issue.Milestone.Name != "" {
			milestones[issue.Milestone.Name] = &base.Milestone{
				Title:   issue.Milestone.Name,
				Created: time.Unix(0, 0),
				State:   "open",
			}
		}
	}

	result := make([]*base.Milestone, 0, len(milestones))
	for _, milestone := range milestones {
		result = append(result, milestone)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Title < result[j].Title })
	return result, nil
}

// GetIssues returns issues according start and limit.
func (b *BitbucketDownloader) GetIssues(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
	var resp bitbucketPage[bitbucketIssue]
	query := url.Values{
		"pagelen": []string{strconv.Itoa(perPage)},
		"page":    []string{strconv.Itoa(page)},
		"sort":    []string{"created_on"},
	}
	if err := b.doAPI(ctx, b.apiPath("/issues"), query, &resp); err != nil {
		if apiErr, ok := err.(*bitbucketAPIError); ok && apiErr.StatusCode == http.StatusNotFound {
			return nil, false, base.ErrNotSupported{Entity: "Issues"}
		}
		return nil, false, err
	}

	issues := make([]*base.Issue, 0, len(resp.Values))
	for _, issue := range resp.Values {
		b.recordIssueID(issue.ID)
		state := bitbucketIssueState(issue.State)
		assignees := []string{}
		if issue.Assignee != nil {
			assignees = append(assignees, bitbucketUserName(*issue.Assignee))
		}
		milestone := ""
		if issue.Milestone != nil {
			milestone = issue.Milestone.Name
		}
		issues = append(issues, &base.Issue{
			Number:       issue.ID,
			PosterID:     bitbucketUserID(issue.Reporter),
			PosterName:   bitbucketUserName(issue.Reporter),
			Title:        issue.Title,
			Content:      issue.Content.Raw,
			Milestone:    milestone,
			State:        state,
			Created:      issue.Created,
			Updated:      issue.Updated,
			Closed:       bitbucketClosedTime(state, issue.Updated),
			Labels:       bitbucketIssueLabels(issue),
			Assignees:    assignees,
			ForeignIndex: issue.ID,
		})
	}
	if resp.Next == "" {
		b.issuesRead = true
	}
	return issues, resp.Next == "", nil
}

// GetComments returns comments of an issue or PR.
func (b *BitbucketDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	var path string
	switch commentable.(type) {
	case *base.PullRequest:
		path = b.apiPath("/pullrequests/%d/comments", commentable.GetForeignIndex())
	default:
		path = b.apiPath("/issues/%d/comments", commentable.GetForeignIndex())
	}

	comments := make([]*base.Comment, 0)
	for page := 1; ; page++ {
		var resp bitbucketPage[bitbucketComment]
		query := url.Values{
			"pagelen": []string{strconv.Itoa(b.maxPerPage)},
			"page":    []string{strconv.Itoa(page)},
		}
		if err := b.doAPI(ctx, path, query, &resp); err != nil {
			return nil, false, err
		}
		for _, comment := range resp.Values {
			comments = append(comments, &base.Comment{
				IssueIndex: commentable.GetLocalIndex(),
				Index:      comment.ID,
				PosterID:   bitbucketUserID(comment.User),
				PosterName: bitbucketUserName(comment.User),
				Created:    comment.Created,
				Updated:    comment.Updated,
				Content:    comment.Content.Raw,
			})
		}
		if resp.Next == "" {
			break
		}
	}
	return comments, true, nil
}

// GetPullRequests returns pull requests according page and perPage.
func (b *BitbucketDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	var resp bitbucketPage[bitbucketPullRequest]
	query := url.Values{
		"pagelen": []string{strconv.Itoa(perPage)},
		"page":    []string{strconv.Itoa(page)},
		"sort":    []string{"created_on"},
		"state":   []string{"OPEN", "MERGED", "DECLINED", "SUPERSEDED"},
	}
	if err := b.doAPI(ctx, b.apiPath("/pullrequests"), query, &resp); err != nil {
		return nil, false, err
	}

	pullRequests := make([]*base.PullRequest, 0, len(resp.Values))
	for _, pr := range resp.Values {
		number, err := b.bitbucketPRNumber(ctx, pr.ID)
		if err != nil {
			return nil, false, err
		}
		state := bitbucketPRState(pr.State)
		merged := strings.EqualFold(pr.State, "merged")
		var mergedTime *time.Time
		if merged {
			mergedTime = &pr.Updated
		}
		mergeCommitSHA := ""
		if pr.MergeCommit != nil {
			mergeCommitSHA = pr.MergeCommit.Hash
		}
		headOwner, headRepo := bitbucketOwnerAndRepo(pr.Source.Repository)
		baseOwner, baseRepo := bitbucketOwnerAndRepo(pr.Destination.Repository)
		pullRequests = append(pullRequests, &base.PullRequest{
			Number:         number,
			Title:          pr.Title,
			PosterID:       bitbucketUserID(pr.Author),
			PosterName:     bitbucketUserName(pr.Author),
			Content:        pr.Description,
			State:          state,
			Created:        pr.Created,
			Updated:        pr.Updated,
			Closed:         bitbucketClosedTime(state, pr.Updated),
			PatchURL:       pr.Links.Patch.Href,
			Merged:         merged,
			MergedTime:     mergedTime,
			MergeCommitSHA: mergeCommitSHA,
			Head: base.PullRequestBranch{
				CloneURL:  bitbucketCloneURL(pr.Source.Repository),
				Ref:       pr.Source.Branch.Name,
				SHA:       pr.Source.Commit.Hash,
				OwnerName: headOwner,
				RepoName:  headRepo,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.Destination.Branch.Name,
				SHA:       pr.Destination.Commit.Hash,
				OwnerName: baseOwner,
				RepoName:  baseRepo,
			},
			ForeignIndex: pr.ID,
		})

		_ = CheckAndEnsureSafePR(pullRequests[len(pullRequests)-1], b.webBaseURL, b)
	}
	return pullRequests, resp.Next == "", nil
}
