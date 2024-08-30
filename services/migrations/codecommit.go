// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/aws/aws-sdk-go/aws"
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
	return NewCodeCommitDownloader(ctx, opts.CodeCommitRepoName, opts.AWSAccessKeyID, opts.AWSSecretAccessKey, opts.AWSRegion), nil
}

// GitServiceType returns the type of git service
func (c *CodeCommitDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.CodeCommitService
}

func NewCodeCommitDownloader(ctx context.Context, repoName, accessKeyID, secretAccessKey, region string) *CodeCommitDownloader {
	downloader := CodeCommitDownloader{
		ctx:      ctx,
		repoName: repoName,
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
	ctx               context.Context
	codeCommitClient  *codecommit.Client
	repoName          string
	allPullRequestIDs []string
}

// SetContext set context
func (c *CodeCommitDownloader) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// GetRepoInfo returns a repository information
func (c *CodeCommitDownloader) GetRepoInfo() (*base.Repository, error) {
	output, err := c.codeCommitClient.GetRepository(c.ctx, &codecommit.GetRepositoryInput{
		RepositoryName: aws.String(c.repoName),
	})
	if err != nil {
		return nil, err
	}
	repoMeta := output.RepositoryMetadata
	return &base.Repository{
		Name:          *repoMeta.RepositoryName,
		Owner:         *repoMeta.AccountId,
		IsPrivate:     true, // CodeCommit repos are always private
		DefaultBranch: *repoMeta.DefaultBranch,
		Description:   *repoMeta.RepositoryDescription,
	}, nil
}

// GetComments returns comments of an issue or PR
func (c *CodeCommitDownloader) GetComments(commentable base.Commentable) ([]*base.Comment, bool, error) {
	var (
		nextToken  *string
		ccComments []types.Comment
	)

	for {
		resp, err := c.codeCommitClient.GetCommentsForPullRequest(c.ctx, &codecommit.GetCommentsForPullRequestInput{
			NextToken:     nextToken,
			PullRequestId: aws.String(strconv.FormatInt(commentable.GetForeignIndex(), 10)),
		})
		if err != nil {
			return nil, false, err
		}

		for _, prComment := range resp.CommentsForPullRequestData {
			ccComments = append(ccComments, prComment.Comments...)
		}

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}

	comments := make([]*base.Comment, 0, len(ccComments))
	for _, ccComment := range ccComments {
		comment := &base.Comment{
			IssueIndex: commentable.GetForeignIndex(),
			PosterName: c.getUsernameFromARN(*ccComment.AuthorArn),
			Content:    *ccComment.Content,
			Created:    *ccComment.CreationDate,
			Updated:    *ccComment.LastModifiedDate,
		}
		comments = append(comments, comment)
	}

	return comments, true, nil
}

// GetPullRequests returns pull requests according page and perPage
func (c *CodeCommitDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	allPullRequestIDs, err := c.getAllPullRequestIDs()
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
		output, err := c.codeCommitClient.GetPullRequest(c.ctx, &codecommit.GetPullRequestInput{
			PullRequestId: aws.String(id),
		})
		if err != nil {
			return nil, false, err
		}
		ccpr := output.PullRequest
		number, err := strconv.ParseInt(*ccpr.PullRequestId, 10, 64)
		if err != nil {
			log.Error("CodeCommit pull request id is not a number: %s", *ccpr.PullRequestId)
			continue
		}
		if len(ccpr.PullRequestTargets) == 0 {
			log.Error("CodeCommit pull request does not contain targets", *ccpr.PullRequestId)
			continue
		}
		target := ccpr.PullRequestTargets[0]
		prState := "closed"
		if ccpr.PullRequestStatus == types.PullRequestStatusEnumOpen {
			prState = "open"
		}
		pr := &base.PullRequest{
			Number:     number,
			Title:      *ccpr.Title,
			PosterName: c.getUsernameFromARN(*ccpr.AuthorArn),
			Content:    *ccpr.Description,
			State:      prState,
			Created:    *ccpr.CreationDate,
			Updated:    *ccpr.LastActivityDate,
			Merged:     target.MergeMetadata.IsMerged,
			Head: base.PullRequestBranch{
				Ref:      *target.SourceReference,
				SHA:      *target.SourceCommit,
				RepoName: c.repoName,
			},
			Base: base.PullRequestBranch{
				Ref:      *target.DestinationReference,
				SHA:      *target.DestinationCommit,
				RepoName: c.repoName,
			},
			ForeignIndex: number,
		}
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
	u.User = url.UserPassword(opts.CodeCommitGitCredUsername, opts.CodeCommitGitCredPassword)
	return u.String(), nil
}

func (c *CodeCommitDownloader) getAllPullRequestIDs() ([]string, error) {
	if len(c.allPullRequestIDs) > 0 {
		return c.allPullRequestIDs, nil
	}

	var (
		nextToken *string
		prIDs     []string
	)

	for {
		output, err := c.codeCommitClient.ListPullRequests(c.ctx, &codecommit.ListPullRequestsInput{
			RepositoryName: aws.String(c.repoName),
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
