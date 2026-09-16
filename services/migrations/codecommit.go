// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	git_module "gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
)

var (
	_ base.Downloader        = &CodeCommitDownloader{}
	_ base.DownloaderFactory = &CodeCommitDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&CodeCommitDownloaderFactory{})
}

// CodeCommitDownloaderFactory defines a codecommit downloader factory
type CodeCommitDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (c *CodeCommitDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	hostElems := strings.Split(u.Host, ".")
	if len(hostElems) != 4 {
		return nil, errors.New("cannot get the region from clone URL")
	}
	region := hostElems[1]

	pathElems := strings.Split(u.Path, "/")
	if len(pathElems) == 0 {
		return nil, errors.New("cannot get the repo name from clone URL")
	}
	repoName := pathElems[len(pathElems)-1]

	baseURL := u.Scheme + "://" + u.Host

	return NewCodeCommitDownloader(ctx, repoName, baseURL, opts.AWSAccessKeyID, opts.AWSSecretAccessKey, region), nil
}

// GitServiceType returns the type of git service
func (c *CodeCommitDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.CodeCommitService
}

func NewCodeCommitDownloader(_ context.Context, repoName, baseURL, accessKeyID, secretAccessKey, region string) *CodeCommitDownloader {
	downloader := CodeCommitDownloader{
		repoName:        repoName,
		baseURL:         baseURL,
		client:          http.DefaultClient,
		endpoint:        "https://codecommit." + region + ".amazonaws.com",
		region:          region,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
	}

	return &downloader
}

// CodeCommitDownloader implements a downloader for AWS CodeCommit
type CodeCommitDownloader struct {
	base.NullDownloader
	client            *http.Client
	endpoint          string
	region            string
	accessKeyID       string
	secretAccessKey   string
	repoName          string
	baseURL           string
	allPullRequestIDs []string
}

// GetRepoInfo returns a repository information
func (c *CodeCommitDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var output struct {
		RepositoryMetadata struct {
			AccountID             string `json:"accountId"`
			CloneURLHTTP          string `json:"cloneUrlHttp"`
			DefaultBranch         string `json:"defaultBranch"`
			RepositoryDescription string `json:"repositoryDescription"`
			RepositoryName        string `json:"repositoryName"`
		} `json:"repositoryMetadata"`
	}
	if err := c.callAPI(ctx, "GetRepository", map[string]string{"repositoryName": c.repoName}, &output); err != nil {
		return nil, err
	}
	repoMeta := output.RepositoryMetadata

	return &base.Repository{
		Name:          repoMeta.RepositoryName,
		Owner:         repoMeta.AccountID,
		IsPrivate:     true, // CodeCommit repos are always private
		CloneURL:      repoMeta.CloneURLHTTP,
		DefaultBranch: repoMeta.DefaultBranch,
		Description:   repoMeta.RepositoryDescription,
	}, nil
}

// GetComments returns comments of an issue or PR
func (c *CodeCommitDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	var comments []*base.Comment
	input := map[string]string{"pullRequestId": strconv.FormatInt(commentable.GetForeignIndex(), 10)}

	for {
		var resp struct {
			CommentsForPullRequestData []struct {
				Comments []struct {
					AuthorArn        string  `json:"authorArn"`
					Content          string  `json:"content"`
					CreationDate     float64 `json:"creationDate"`
					LastModifiedDate float64 `json:"lastModifiedDate"`
				} `json:"comments"`
			} `json:"commentsForPullRequestData"`
			NextToken string `json:"nextToken"`
		}
		if err := c.callAPI(ctx, "GetCommentsForPullRequest", input, &resp); err != nil {
			return nil, false, err
		}

		for _, prComment := range resp.CommentsForPullRequestData {
			for _, ccComment := range prComment.Comments {
				comment := &base.Comment{
					IssueIndex: commentable.GetForeignIndex(),
					PosterName: c.getUsernameFromARN(ccComment.AuthorArn),
					Content:    ccComment.Content,
					Created:    time.Unix(int64(ccComment.CreationDate), 0),
					Updated:    time.Unix(int64(ccComment.LastModifiedDate), 0),
				}
				comments = append(comments, comment)
			}
		}

		if resp.NextToken == "" {
			break
		}
		input["nextToken"] = resp.NextToken
	}

	return comments, true, nil
}

// GetPullRequests returns pull requests according page and perPage
func (c *CodeCommitDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	allPullRequestIDs, err := c.getAllPullRequestIDs(ctx)
	if err != nil {
		return nil, false, err
	}

	startIndex := (page - 1) * perPage
	endIndex := min(page*perPage, len(allPullRequestIDs))
	batch := allPullRequestIDs[startIndex:endIndex]

	prs := make([]*base.PullRequest, 0, len(batch))
	for _, id := range batch {
		var output struct {
			PullRequest struct {
				AuthorArn          string  `json:"authorArn"`
				CreationDate       float64 `json:"creationDate"`
				Description        string  `json:"description"`
				LastActivityDate   float64 `json:"lastActivityDate"`
				PullRequestID      string  `json:"pullRequestId"`
				PullRequestStatus  string  `json:"pullRequestStatus"`
				PullRequestTargets []struct {
					DestinationCommit    string `json:"destinationCommit"`
					DestinationReference string `json:"destinationReference"`
					MergeMetadata        struct {
						IsMerged      bool   `json:"isMerged"`
						MergeCommitID string `json:"mergeCommitId"`
					} `json:"mergeMetadata"`
					SourceCommit    string `json:"sourceCommit"`
					SourceReference string `json:"sourceReference"`
				} `json:"pullRequestTargets"`
				Title string `json:"title"`
			} `json:"pullRequest"`
		}
		if err := c.callAPI(ctx, "GetPullRequest", map[string]string{"pullRequestId": id}, &output); err != nil {
			return nil, false, err
		}
		orig := output.PullRequest
		number, err := strconv.ParseInt(orig.PullRequestID, 10, 64)
		if err != nil {
			log.Error("CodeCommit pull request id is not a number: %s", orig.PullRequestID)
			continue
		}
		if len(orig.PullRequestTargets) == 0 {
			log.Error("CodeCommit pull request %s does not contain targets", orig.PullRequestID)
			continue
		}
		target := orig.PullRequestTargets[0]
		lastActivity := time.Unix(int64(orig.LastActivityDate), 0)
		pr := &base.PullRequest{
			Number:     number,
			Title:      orig.Title,
			PosterName: c.getUsernameFromARN(orig.AuthorArn),
			Content:    orig.Description,
			State:      "open",
			Created:    time.Unix(int64(orig.CreationDate), 0),
			Updated:    lastActivity,
			Merged:     target.MergeMetadata.IsMerged,
			Head: base.PullRequestBranch{
				Ref:      strings.TrimPrefix(target.SourceReference, git_module.BranchPrefix),
				SHA:      target.SourceCommit,
				RepoName: c.repoName,
			},
			Base: base.PullRequestBranch{
				Ref:      strings.TrimPrefix(target.DestinationReference, git_module.BranchPrefix),
				SHA:      target.DestinationCommit,
				RepoName: c.repoName,
			},
			ForeignIndex: number,
		}

		if orig.PullRequestStatus == "CLOSED" {
			pr.State = "closed"
			pr.Closed = &lastActivity
		}
		if pr.Merged {
			pr.MergeCommitSHA = target.MergeMetadata.MergeCommitID
			pr.MergedTime = &lastActivity
		}

		_ = CheckAndEnsureSafePR(pr, c.baseURL, c)
		prs = append(prs, pr)
	}

	return prs, len(prs) < perPage, nil
}

// FormatCloneURL add authentication into remote URLs
func (c *CodeCommitDownloader) FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
	return u.String(), nil
}

func (c *CodeCommitDownloader) getAllPullRequestIDs(ctx context.Context) ([]string, error) {
	if len(c.allPullRequestIDs) > 0 {
		return c.allPullRequestIDs, nil
	}

	var prIDs []string
	input := map[string]string{"repositoryName": c.repoName}

	for {
		var output struct {
			NextToken      string   `json:"nextToken"`
			PullRequestIDs []string `json:"pullRequestIds"`
		}
		if err := c.callAPI(ctx, "ListPullRequests", input, &output); err != nil {
			return nil, err
		}
		prIDs = append(prIDs, output.PullRequestIDs...)
		if output.NextToken == "" {
			break
		}
		input["nextToken"] = output.NextToken
	}

	c.allPullRequestIDs = prIDs
	return c.allPullRequestIDs, nil
}

func (c *CodeCommitDownloader) getUsernameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func (c *CodeCommitDownloader) callAPI(ctx context.Context, operation string, input map[string]string, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	amzDate, target := time.Now().UTC().Format("20060102T150405Z"), "CodeCommit_20150413."+operation
	scope := amzDate[:8] + "/" + c.region + "/codecommit/aws4_request"
	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/x-amz-json-1.1\nhost:%s\nx-amz-date:%s\nx-amz-target:%s\n\ncontent-type;host;x-amz-date;x-amz-target\n%x", req.URL.Host, amzDate, target, sha256.Sum256(body))
	signature := []byte("AWS4" + c.secretAccessKey)
	for _, data := range []string{amzDate[:8], c.region, "codecommit", "aws4_request", fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%x", amzDate, scope, sha256.Sum256([]byte(canonicalRequest)))} { // SigV4 key derivation, the last round signs
		mac := hmac.New(sha256.New, signature)
		_, _ = mac.Write([]byte(data))
		signature = mac.Sum(nil)
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Target", target)
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host;x-amz-date;x-amz-target, Signature=%x", c.accessKeyID, scope, signature))
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return json.NewDecoder(resp.Body).Decode(output)
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("CodeCommit %s: %s: %s", operation, resp.Status, respBody)
}
