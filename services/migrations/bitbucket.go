// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"cmp"
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	user_model "gitea.dev/models/user"
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

const (
	bitbucketWebURL      = "https://bitbucket.org"
	bitbucketMaxPageSize = 100
)

func init() {
	RegisterDownloaderFactory(&BitbucketDownloaderFactory{})
}

// BitbucketDownloaderFactory defines a bitbucket.org downloader factory.
type BitbucketDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions.
func (f *BitbucketDownloaderFactory) New(_ context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(u.Hostname(), "bitbucket.org") { // the credentials go to api.bitbucket.org, so another host must not get them
		return nil, fmt.Errorf("invalid Bitbucket Cloud clone URL, the host must be bitbucket.org: %s", opts.CloneAddr)
	}
	fields := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid Bitbucket clone URL: %s", opts.CloneAddr)
	}
	return NewBitbucketDownloader("https://api.bitbucket.org/2.0", fields[0], strings.TrimSuffix(fields[1], ".git"), opts.AuthUsername, opts.AuthPassword, opts.AuthToken), nil
}

// GitServiceType returns the type of git service.
func (f *BitbucketDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.BitbucketService
}

type bitbucketPage[T any] struct {
	Values []T    `json:"values"`
	Next   string `json:"next"`
}

type bitbucketUser struct {
	UUID        string `json:"uuid"`
	AccountID   string `json:"account_id"`
	Nickname    string `json:"nickname"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

type bitbucketBranch struct {
	Name string `json:"name"`
}

type bitbucketRepository struct {
	Slug        string          `json:"slug"`
	FullName    string          `json:"full_name"`
	Description string          `json:"description"`
	IsPrivate   bool            `json:"is_private"`
	Website     string          `json:"website"`
	MainBranch  bitbucketBranch `json:"mainbranch"`
}

type bitbucketCommit struct {
	Hash string `json:"hash"`
}

type bitbucketPRSide struct {
	Branch     bitbucketBranch     `json:"branch"`
	Commit     bitbucketCommit     `json:"commit"`
	Repository bitbucketRepository `json:"repository"`
}

type bitbucketParticipant struct {
	User           bitbucketUser `json:"user"`
	Approved       bool          `json:"approved"`
	State          string        `json:"state"`
	ParticipatedOn time.Time     `json:"participated_on"`
}

type bitbucketPullRequest struct {
	ID           int64                  `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	State        string                 `json:"state"`
	Author       bitbucketUser          `json:"author"`
	Source       bitbucketPRSide        `json:"source"`
	Destination  bitbucketPRSide        `json:"destination"`
	MergeCommit  bitbucketCommit        `json:"merge_commit"`
	Participants []bitbucketParticipant `json:"participants"`
	Created      time.Time              `json:"created_on"`
	Updated      time.Time              `json:"updated_on"`
}

type bitbucketComment struct {
	ID      int64 `json:"id"`
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	User    bitbucketUser `json:"user"`
	Created time.Time     `json:"created_on"`
	Updated time.Time     `json:"updated_on"`
	Deleted bool          `json:"deleted"`
	Inline  *struct {
		Path string `json:"path"`
		From int    `json:"from"`
		To   int    `json:"to"`
	} `json:"inline"`
	Parent     *struct{} `json:"parent"`
	Resolution *struct{} `json:"resolution"`
}

// bitbucketPRContext carries what GetComments and GetReviews share, so each pull request is fetched once
type bitbucketPRContext struct {
	pr       bitbucketPullRequest
	headSHA  string
	comments []bitbucketComment
}

// BitbucketDownloader implements a Downloader interface to get repository information from bitbucket.org.
type BitbucketDownloader struct {
	base.NullDownloader
	client     *http.Client
	apiBaseURL string
	workspace  string
	repoSlug   string
	commitSHAs map[string]string
}

// NewBitbucketDownloader creates a bitbucket.org Downloader via API v2.
func NewBitbucketDownloader(apiBaseURL, workspace, repoSlug, userName, password, token string) *BitbucketDownloader {
	authorization := util.Iif(token != "", "Bearer "+token, basicAuthorization(userName, password))
	return &BitbucketDownloader{
		client:     newMigrationHTTPClient(apiBaseURL, authorization),
		apiBaseURL: apiBaseURL,
		workspace:  workspace,
		repoSlug:   repoSlug,
		commitSHAs: map[string]string{},
	}
}

// String implements Stringer.
func (b *BitbucketDownloader) String() string {
	return fmt.Sprintf("migration from bitbucket cloud %s/%s", b.workspace, b.repoSlug)
}

func (b *BitbucketDownloader) LogString() string {
	if b == nil {
		return "<BitbucketDownloader nil>"
	}
	return fmt.Sprintf("<BitbucketDownloader %s/%s>", b.workspace, b.repoSlug)
}

// FormatCloneURL adds authentication into the clone URL, with the fixed git usernames Bitbucket Cloud requires for tokens
func (b *BitbucketDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	if opts.AuthToken != "" {
		opts.AuthUsername, opts.AuthPassword, opts.AuthToken = "x-token-auth", opts.AuthToken, ""
	} else if strings.Contains(opts.AuthUsername, "@") && opts.AuthPassword != "" {
		opts.AuthUsername = "x-bitbucket-api-token-auth"
	}
	return b.NullDownloader.FormatCloneURL(opts, remoteAddr)
}

func (b *BitbucketDownloader) apiPath(format string, args ...any) string {
	return fmt.Sprintf("/repositories/%s/%s"+format, append([]any{url.PathEscape(b.workspace), url.PathEscape(b.repoSlug)}, args...)...)
}

func (b *BitbucketDownloader) doAPI(ctx context.Context, path string, query url.Values, out any) error {
	reqURL := b.apiBaseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &bbsStatusError{Status: resp.StatusCode, URL: reqURL}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func bitbucketUserID(user bitbucketUser) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(cmp.Or(user.AccountID, user.UUID, bitbucketUserName(user))))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func bitbucketUserName(user bitbucketUser) string {
	return cmp.Or(user.Nickname, user.Username, user.DisplayName, user.AccountID, user.UUID, user_model.GhostUserName)
}

func bitbucketCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s.git", bitbucketWebURL, url.PathEscape(owner), url.PathEscape(repo))
}

func bitbucketOwnerAndRepo(repo bitbucketRepository) (string, string) {
	owner, name, _ := strings.Cut(repo.FullName, "/")
	return owner, name
}

// GetRepoInfo returns a repository information.
func (b *BitbucketDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var repo bitbucketRepository
	if err := b.doAPI(ctx, b.apiPath(""), nil, &repo); err != nil {
		return nil, err
	}
	owner, name := bitbucketOwnerAndRepo(repo)
	return &base.Repository{
		Owner:         owner,
		Name:          repo.Slug,
		IsPrivate:     repo.IsPrivate,
		Description:   repo.Description,
		Website:       repo.Website,
		OriginalURL:   fmt.Sprintf("%s/%s/%s", bitbucketWebURL, url.PathEscape(owner), url.PathEscape(name)),
		CloneURL:      bitbucketCloneURL(owner, name),
		DefaultBranch: repo.MainBranch.Name,
	}, nil
}

func (b *BitbucketDownloader) pullRequestComments(ctx context.Context, reviewable base.Reviewable) ([]bitbucketComment, error) {
	prCtx, _ := reviewable.GetContext().(*bitbucketPRContext)
	if prCtx != nil && prCtx.comments != nil {
		return prCtx.comments, nil
	}
	all := []bitbucketComment{}
	for page := 1; ; page++ {
		var resp bitbucketPage[bitbucketComment]
		query := url.Values{"pagelen": {strconv.Itoa(bitbucketMaxPageSize)}, "page": {strconv.Itoa(page)}}
		if err := b.doAPI(ctx, b.apiPath("/pullrequests/%d/comments", reviewable.GetForeignIndex()), query, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.Values...)
		if resp.Next == "" {
			break
		}
	}
	if prCtx != nil {
		prCtx.comments = all
	}
	return all, nil
}

// GetComments returns comments of a pull request; inline comments are returned by GetReviews.
func (b *BitbucketDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	all, err := b.pullRequestComments(ctx, commentable)
	if err != nil {
		return nil, false, err
	}
	comments := make([]*base.Comment, 0, len(all))
	for _, comment := range all {
		if comment.Deleted || comment.Inline != nil {
			continue
		}
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
	return comments, true, nil
}

// GetReviews returns the approvals, change requests and inline code comments of a pull request.
func (b *BitbucketDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	all, err := b.pullRequestComments(ctx, reviewable)
	if err != nil {
		return nil, err
	}
	prCtx, _ := reviewable.GetContext().(*bitbucketPRContext)
	if prCtx == nil {
		prCtx = &bitbucketPRContext{}
	}

	var reviews []*base.Review
	for _, c := range all {
		if c.Deleted || c.Inline == nil {
			continue
		}
		line := c.Inline.To
		if line == 0 {
			line = -c.Inline.From // a negative line addresses the old side of the diff
		}
		// every comment becomes its own review so that replies keep their author
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   bitbucketUserID(c.User),
			ReviewerName: bitbucketUserName(c.User),
			CreatedAt:    c.Created,
			State:        base.ReviewStateCommented,
			CommitID:     prCtx.headSHA,
			Comments: []*base.ReviewComment{{
				ID:        c.ID,
				Content:   c.Content.Raw,
				TreePath:  c.Inline.Path,
				Line:      line,
				CommitID:  prCtx.headSHA,
				PosterID:  bitbucketUserID(c.User),
				CreatedAt: c.Created,
				UpdatedAt: cmp.Or(c.Updated, c.Created),
				Resolved:  c.Parent == nil && c.Resolution != nil,
			}},
		})
	}

	for _, participant := range prCtx.pr.Participants {
		var state string
		switch {
		case participant.State == "changes_requested":
			state = base.ReviewStateChangesRequested
		case participant.Approved || participant.State == "approved":
			state = base.ReviewStateApproved
		default:
			continue
		}
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   bitbucketUserID(participant.User),
			ReviewerName: bitbucketUserName(participant.User),
			CreatedAt:    cmp.Or(participant.ParticipatedOn, prCtx.pr.Updated),
			State:        state,
			CommitID:     prCtx.headSHA,
		})
	}
	slices.SortStableFunc(reviews, func(a, b *base.Review) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return reviews, nil
}

// fullCommitSHA expands the 12-char hashes the API returns, "" lets the uploader recover the SHA from the branch
func (b *BitbucketDownloader) fullCommitSHA(ctx context.Context, repo bitbucketRepository, hash string) string {
	if len(hash) >= 40 {
		return hash
	}
	owner, name := bitbucketOwnerAndRepo(repo)
	if hash == "" || owner == "" || name == "" {
		return ""
	}
	path := fmt.Sprintf("/repositories/%s/%s/commit/%s", url.PathEscape(owner), url.PathEscape(name), url.PathEscape(hash))
	if sha, ok := b.commitSHAs[path]; ok {
		return sha
	}
	var commit bitbucketCommit
	if err := b.doAPI(ctx, path, nil, &commit); err != nil {
		log.Warn("Bitbucket: unable to resolve commit %s in %s/%s: %v", hash, owner, name, err)
	}
	b.commitSHAs[path] = commit.Hash
	return commit.Hash
}

// GetPullRequests returns pull requests according page and perPage.
func (b *BitbucketDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	var resp bitbucketPage[bitbucketPullRequest]
	query := url.Values{
		"fields":  {"+values.participants"},
		"pagelen": {strconv.Itoa(perPage)},
		"page":    {strconv.Itoa(page)},
		"sort":    {"created_on"},
		"state":   {"OPEN", "MERGED", "DECLINED", "SUPERSEDED"},
	}
	if err := b.doAPI(ctx, b.apiPath("/pullrequests"), query, &resp); err != nil {
		return nil, false, err
	}

	pullRequests := make([]*base.PullRequest, 0, len(resp.Values))
	for _, pr := range resp.Values {
		state := util.Iif(strings.EqualFold(pr.State, "open"), "open", "closed")
		merged := strings.EqualFold(pr.State, "merged")
		var closed, mergedTime *time.Time
		if state == "closed" {
			closed = &pr.Updated
		}
		if merged {
			mergedTime = &pr.Updated
		}
		headOwner, headRepo := bitbucketOwnerAndRepo(pr.Source.Repository)
		baseOwner, baseRepo := bitbucketOwnerAndRepo(pr.Destination.Repository)
		headSHA := b.fullCommitSHA(ctx, pr.Source.Repository, pr.Source.Commit.Hash)
		var headCloneURL string
		if headOwner != "" && headRepo != "" {
			headCloneURL = bitbucketCloneURL(headOwner, headRepo)
		}
		pullRequests = append(pullRequests, &base.PullRequest{
			Number:         pr.ID,
			Title:          pr.Title,
			PosterID:       bitbucketUserID(pr.Author),
			PosterName:     bitbucketUserName(pr.Author),
			Content:        pr.Description,
			State:          state,
			Created:        pr.Created,
			Updated:        pr.Updated,
			Closed:         closed,
			Merged:         merged,
			MergedTime:     mergedTime,
			MergeCommitSHA: b.fullCommitSHA(ctx, pr.Destination.Repository, pr.MergeCommit.Hash),
			Head: base.PullRequestBranch{
				CloneURL:  headCloneURL,
				Ref:       pr.Source.Branch.Name,
				SHA:       headSHA,
				OwnerName: headOwner,
				RepoName:  headRepo,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.Destination.Branch.Name,
				SHA:       b.fullCommitSHA(ctx, pr.Destination.Repository, pr.Destination.Commit.Hash),
				OwnerName: baseOwner,
				RepoName:  baseRepo,
			},
			ForeignIndex: pr.ID,
			Context:      &bitbucketPRContext{pr: pr, headSHA: headSHA},
		})

		_ = CheckAndEnsureSafePR(pullRequests[len(pullRequests)-1], bitbucketWebURL, b)
	}
	return pullRequests, resp.Next == "", nil
}
