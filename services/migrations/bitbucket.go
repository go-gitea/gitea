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
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

var (
	_ base.Downloader        = &BitbucketDownloader{}
	_ base.DownloaderFactory = &BitbucketDownloaderFactory{}
)

const bitbucketWebURL = "https://bitbucket.org"

func init() {
	RegisterDownloaderFactory(&BitbucketDownloaderFactory{})
}

type BitbucketDownloaderFactory struct{}

func (f *BitbucketDownloaderFactory) New(_ context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	cloneURL, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.Trim(cloneURL.Path, "/"), "/")
	if !strings.EqualFold(cloneURL.Hostname(), "bitbucket.org") || len(fields) < 2 { // the token goes to api.bitbucket.org, so no other host's URL is accepted
		return nil, fmt.Errorf("invalid Bitbucket Cloud URL %q, expected https://bitbucket.org/<workspace>/<repo>", cloneURL.Redacted())
	}
	return NewBitbucketDownloader("https://api.bitbucket.org/2.0", fields[0], strings.TrimSuffix(fields[1], ".git"), cmp.Or(opts.AuthToken, opts.AuthPassword)), nil
}

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
	Draft        bool                   `json:"draft"`
	CommentCount int                    `json:"comment_count"`
	Author       bitbucketUser          `json:"author"`
	Source       bitbucketPRSide        `json:"source"`
	Destination  bitbucketPRSide        `json:"destination"`
	MergeCommit  bitbucketCommit        `json:"merge_commit"`
	Participants []bitbucketParticipant `json:"participants"`
	Created      time.Time              `json:"created_on"`
	Updated      time.Time              `json:"updated_on"`
}

type bitbucketComment struct {
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

// isLineComment is false for file comments, which carry neither line
func (c *bitbucketComment) isLineComment() bool {
	return c.Inline != nil && (c.Inline.From > 0 || c.Inline.To > 0)
}

// bitbucketPRContext carries what GetComments and GetReviews share, so the comments of each pull request are fetched once
type bitbucketPRContext struct {
	pr       bitbucketPullRequest
	comments []bitbucketComment
}

type BitbucketDownloader struct {
	base.NullDownloader
	client     *http.Client
	apiRepoURL string
	workspace  string
	repoSlug   string
}

func NewBitbucketDownloader(apiBaseURL, workspace, repoSlug, token string) *BitbucketDownloader {
	return &BitbucketDownloader{
		// Bitbucket Cloud answers 429 without Retry-After and refills its limits over a rolling hour
		client:     &http.Client{Transport: authTransport(retryAfterTransport{RoundTripper: getMigrationTransport(), backoff: time.Minute}, apiBaseURL, util.Iif(token != "", "Bearer "+token, ""))},
		apiRepoURL: apiBaseURL + "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repoSlug),
		workspace:  workspace,
		repoSlug:   repoSlug,
	}
}

func (b *BitbucketDownloader) String() string {
	return fmt.Sprintf("migration from bitbucket cloud %s/%s", b.workspace, b.repoSlug)
}

func (b *BitbucketDownloader) LogString() string {
	if b == nil {
		return "<BitbucketDownloader nil>"
	}
	return fmt.Sprintf("<BitbucketDownloader %s/%s>", b.workspace, b.repoSlug)
}

// FormatCloneURL adds the token into the clone URL, git accepts API and access tokens with the static user name
func (b *BitbucketDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	if token := cmp.Or(opts.AuthToken, opts.AuthPassword); token != "" {
		opts.AuthUsername, opts.AuthPassword, opts.AuthToken = "x-token-auth", token, ""
	}
	return b.NullDownloader.FormatCloneURL(opts, remoteAddr)
}

func (b *BitbucketDownloader) doAPI(ctx context.Context, path string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.apiRepoURL+path, nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &bbsStatusError{Status: resp.StatusCode, URL: req.URL.String()}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func bitbucketUserID(user bitbucketUser) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(cmp.Or(user.AccountID, user.UUID, bitbucketUserName(user))))
	return int64(hasher.Sum64() & 0x7fffffffffffffff)
}

func bitbucketUserName(user bitbucketUser) string {
	return cmp.Or(user.Nickname, user.DisplayName, user_model.GhostUserName)
}

func bitbucketCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s.git", bitbucketWebURL, url.PathEscape(owner), url.PathEscape(repo))
}

func (b *BitbucketDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var repo bitbucketRepository
	if err := b.doAPI(ctx, "", nil, &repo); err != nil {
		return nil, err
	}
	owner, name, _ := strings.Cut(repo.FullName, "/")
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

func (b *BitbucketDownloader) pullRequestComments(ctx context.Context, reviewable base.Reviewable) (*bitbucketPRContext, error) {
	prCtx, ok := reviewable.GetContext().(*bitbucketPRContext)
	if !ok {
		return nil, fmt.Errorf("pullRequestComments: unexpected context %+v", reviewable.GetContext())
	}
	if prCtx.comments != nil || prCtx.pr.CommentCount == 0 {
		return prCtx, nil
	}
	comments := make([]bitbucketComment, 0, prCtx.pr.CommentCount)
	for page := 1; ; page++ {
		var resp bitbucketPage[bitbucketComment]
		query := url.Values{"pagelen": {"100"}, "page": {strconv.Itoa(page)}}
		if err := b.doAPI(ctx, fmt.Sprintf("/pullrequests/%d/comments", prCtx.pr.ID), query, &resp); err != nil {
			return nil, err
		}
		comments = append(comments, resp.Values...)
		if resp.Next == "" {
			prCtx.comments = comments
			return prCtx, nil
		}
	}
}

func (b *BitbucketDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	prCtx, err := b.pullRequestComments(ctx, commentable)
	if err != nil {
		return nil, false, err
	}
	comments := make([]*base.Comment, 0, len(prCtx.comments))
	for _, comment := range prCtx.comments {
		if comment.Deleted || comment.isLineComment() {
			continue
		}
		content := comment.Content.Raw
		if comment.Inline != nil {
			content = fmt.Sprintf("Comment on `%s`:\n\n%s", comment.Inline.Path, content)
		}
		comments = append(comments, &base.Comment{
			IssueIndex: commentable.GetLocalIndex(),
			PosterID:   bitbucketUserID(comment.User),
			PosterName: bitbucketUserName(comment.User),
			Created:    comment.Created,
			Updated:    comment.Updated,
			Content:    content,
		})
	}
	return comments, true, nil
}

func (b *BitbucketDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	prCtx, err := b.pullRequestComments(ctx, reviewable)
	if err != nil {
		return nil, err
	}

	var reviews []*base.Review
	for _, comment := range prCtx.comments {
		if comment.Deleted || !comment.isLineComment() {
			continue
		}
		// every comment becomes its own review so that replies keep their author
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   bitbucketUserID(comment.User),
			ReviewerName: bitbucketUserName(comment.User),
			CreatedAt:    comment.Created,
			State:        base.ReviewStateCommented,
			Comments: []*base.ReviewComment{{
				Content:   comment.Content.Raw,
				TreePath:  comment.Inline.Path,
				Line:      cmp.Or(comment.Inline.To, -comment.Inline.From), // a negative line addresses the old side of the diff
				CommitID:  prCtx.pr.Source.Commit.Hash,
				UpdatedAt: comment.Updated,
				Resolved:  comment.Parent == nil && comment.Resolution != nil,
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
		})
	}
	slices.SortStableFunc(reviews, func(a, b *base.Review) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return reviews, nil
}

func (b *BitbucketDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	var resp bitbucketPage[bitbucketPullRequest]
	query := url.Values{
		"fields":  {"+values.participants,+values.*.commit.date,+values.merge_commit.date"}, // expanding the commits gives full instead of 12-char hashes
		"pagelen": {strconv.Itoa(perPage)},
		"page":    {strconv.Itoa(page)},
		"sort":    {"created_on"},
		"state":   {"OPEN", "MERGED", "DECLINED", "SUPERSEDED"},
	}
	if err := b.doAPI(ctx, "/pullrequests", query, &resp); err != nil {
		return nil, false, err
	}

	pullRequests := make([]*base.PullRequest, 0, len(resp.Values))
	for _, pr := range resp.Values {
		state := util.Iif(pr.State == "OPEN", "open", "closed")
		merged := pr.State == "MERGED"
		headOwner, headRepo, _ := strings.Cut(pr.Source.Repository.FullName, "/")
		baseOwner, baseRepo, _ := strings.Cut(pr.Destination.Repository.FullName, "/")
		pullRequests = append(pullRequests, &base.PullRequest{
			Number:         pr.ID,
			Title:          pr.Title,
			PosterID:       bitbucketUserID(pr.Author),
			PosterName:     bitbucketUserName(pr.Author),
			Content:        pr.Description,
			State:          state,
			Created:        pr.Created,
			Updated:        pr.Updated,
			IsDraft:        pr.Draft,
			Closed:         util.Iif(state == "closed", &pr.Updated, nil),
			Merged:         merged,
			MergedTime:     util.Iif(merged, &pr.Updated, nil),
			MergeCommitSHA: pr.MergeCommit.Hash,
			Head: base.PullRequestBranch{
				CloneURL:  util.Iif(headOwner != "" && headRepo != "", bitbucketCloneURL(headOwner, headRepo), ""),
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
			Context:      &bitbucketPRContext{pr: pr},
		})

		_ = CheckAndEnsureSafePR(pullRequests[len(pullRequests)-1], bitbucketWebURL, b)
	}
	return pullRequests, resp.Next == "", nil
}
