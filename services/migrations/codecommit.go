// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"

	git_module "code.gitea.io/gitea/modules/git"
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

func NewCodeCommitDownloader(ctx context.Context, repoName, baseURL, accessKeyID, secretAccessKey, region string) *CodeCommitDownloader {
	downloader := CodeCommitDownloader{
		ctx:      ctx,
		repoName: repoName,
		baseURL:  baseURL,
		codeCommitClient: codecommit.New(codecommit.Options{
			Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
			Region:      region,
		}),
		regionName: region,
	}

	return &downloader
}

// CodeCommitDownloader implements a downloader for AWS CodeCommit
type CodeCommitDownloader struct {
	base.NullDownloader
	ctx               context.Context
	codeCommitClient  *codecommit.Client
	repoName          string
	baseURL           string
	allPullRequestIDs []string
	regionName        string
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
func (c *CodeCommitDownloader) GetComments(commentable base.Commentable) ([]*base.Comment, bool, error) {
	var (
		nextToken *string
		comments  []*base.Comment
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
	now := time.Now().UTC()
	datetime := now.Format("20060102T150405Z")
	signature := generateSigV4AuthPassword(opts.AWSSecretAccessKey, u.Host, u.Path, c.regionName, now)
	u.User = url.UserPassword(opts.AWSAccessKeyID, datetime+signature)
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

func generateSigV4AuthPassword(secretKey, host, path, region string, now time.Time) string {
	amzDate := now.Format("20060102T150405")
	date := now.Format("20060102")

	canonicalRequest := fmt.Sprintf("GIT\n%s\n\nhost:%s\n\nhost\n", path, host)

	stringToSign := "AWS4-HMAC-SHA256\n"
	stringToSign += amzDate + "\n"
	stringToSign += fmt.Sprintf("%s/%s/%s/aws4_request\n", date, region, "codecommit")
	stringToSign += hex.EncodeToString(makeHash(sha256.New(), []byte(canonicalRequest)))

	signKey := sign([]byte("AWS4"+secretKey), date)
	signKey = sign(signKey, region)
	signKey = sign(signKey, "codecommit")
	signKey = sign(signKey, "aws4_request")
	signature := sign(signKey, stringToSign)
	return hex.EncodeToString(signature)
}

func sign(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

func makeHash(hash hash.Hash, b []byte) []byte {
	hash.Reset()
	hash.Write(b)
	return hash.Sum(nil)
}
