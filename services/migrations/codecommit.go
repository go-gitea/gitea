// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	git_module "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/aws/aws-sdk-go-v2/service/codecommit/types"
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
		return nil, fmt.Errorf("cannot get the region from clone URL")
	}
	region := hostElems[1]

	pathElems := strings.Split(u.Path, "/")
	if len(pathElems) == 0 {
		return nil, fmt.Errorf("cannot get the repo name from clone URL")
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
		repoName: repoName,
		baseURL:  baseURL,
		codeCommitClient: codecommit.New(codecommit.Options{
			Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
			Region:      region,
		}),
	}

	return &downloader
}

// CodeCommitDownloader implements a downloader for AWS CodeCommit
type CodeCommitDownloader struct {
	base.NullDownloader
	codeCommitClient  *codecommit.Client
	repoName          string
	baseURL           string
	allPullRequestIDs []string
}

// GetRepoInfo returns a repository information
func (c *CodeCommitDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	output, err := c.codeCommitClient.GetRepository(ctx, &codecommit.GetRepositoryInput{
		RepositoryName: util.ToPointer(c.repoName),
	})
	if err != nil {
		return nil, err
	}
	repoMeta := output.RepositoryMetadata

	repo := &base.Repository{
		Name:      *repoMeta.RepositoryName,
		Owner:     *repoMeta.AccountId,
		IsPrivate: true, // CodeCommit repos are always private
		CloneURL:  *repoMeta.CloneUrlHttp,
	}
	if repoMeta.DefaultBranch != nil {
		repo.DefaultBranch = *repoMeta.DefaultBranch
	}
	if repoMeta.RepositoryDescription != nil {
		repo.DefaultBranch = *repoMeta.RepositoryDescription
	}
	return repo, nil
}

// GetComments returns comments of an issue or PR
func (c *CodeCommitDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	var (
		nextToken *string
		comments  []*base.Comment
	)

	for {
		resp, err := c.codeCommitClient.GetCommentsForPullRequest(ctx, &codecommit.GetCommentsForPullRequestInput{
			NextToken:     nextToken,
			PullRequestId: util.ToPointer(strconv.FormatInt(commentable.GetForeignIndex(), 10)),
		})
		if err != nil {
			return nil, false, err
		}

		for _, prComment := range resp.CommentsForPullRequestData {
			for _, ccComment := range prComment.Comments {
				comment := &base.Comment{
					IssueIndex: commentable.GetForeignIndex(),
					PosterName: c.getUsernameFromARN(*ccComment.AuthorArn),
					Content:    *ccComment.Content,
					Created:    *ccComment.CreationDate,
					Updated:    *ccComment.LastModifiedDate,
				}
				comments = append(comments, comment)
			}
		}

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
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
	endIndex := page * perPage
	if endIndex > len(allPullRequestIDs) {
		endIndex = len(allPullRequestIDs)
	}
	batch := allPullRequestIDs[startIndex:endIndex]

	prs := make([]*base.PullRequest, 0, len(batch))
	for _, id := range batch {
		output, err := c.codeCommitClient.GetPullRequest(ctx, &codecommit.GetPullRequestInput{
			PullRequestId: util.ToPointer(id),
		})
		if err != nil {
			return nil, false, err
		}
		orig := output.PullRequest
		number, err := strconv.ParseInt(*orig.PullRequestId, 10, 64)
		if err != nil {
			log.Error("CodeCommit pull request id is not a number: %s", *orig.PullRequestId)
			continue
		}
		if len(orig.PullRequestTargets) == 0 {
			log.Error("CodeCommit pull request does not contain targets", *orig.PullRequestId)
			continue
		}
		target := orig.PullRequestTargets[0]
		pr := &base.PullRequest{
			Number:     number,
			Title:      *orig.Title,
			PosterName: c.getUsernameFromARN(*orig.AuthorArn),
			Content:    *orig.Description,
			State:      "open",
			Created:    *orig.CreationDate,
			Updated:    *orig.LastActivityDate,
			Merged:     target.MergeMetadata.IsMerged,
			Head: base.PullRequestBranch{
				Ref:      strings.TrimPrefix(*target.SourceReference, git_module.BranchPrefix),
				SHA:      *target.SourceCommit,
				RepoName: c.repoName,
			},
			Base: base.PullRequestBranch{
				Ref:      strings.TrimPrefix(*target.DestinationReference, git_module.BranchPrefix),
				SHA:      *target.DestinationCommit,
				RepoName: c.repoName,
			},
			ForeignIndex: number,
		}

		if orig.PullRequestStatus == types.PullRequestStatusEnumClosed {
			pr.State = "closed"
			pr.Closed = orig.LastActivityDate
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

	var (
		nextToken *string
		prIDs     []string
	)

	for {
		output, err := c.codeCommitClient.ListPullRequests(ctx, &codecommit.ListPullRequestsInput{
			RepositoryName: util.ToPointer(c.repoName),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, err
		}
		prIDs = append(prIDs, output.PullRequestIds...)
		nextToken = output.NextToken
		if nextToken == nil {
			break
		}
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
