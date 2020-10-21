// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/repository"

	"gopkg.in/yaml.v2"
)

var (
	_ base.Uploader = &RepositoryDumper{}
)

// RepositoryDumper implements an Uploader to the local directory
type RepositoryDumper struct {
	ctx                  context.Context
	baseDir              string
	repoOwner            string
	repoName             string
	milestoneFile        *os.File
	labelFile            *os.File
	releaseFile          *os.File
	issueFile            *os.File
	commentFiles         map[int64]*os.File
	pullrequestFile      *os.File
	reviewFiles          map[int64]*os.File
	migrateReleaseAssets bool

	gitRepo     *git.Repository
	prHeadCache map[string]struct{}
}

// NewRepositoryDumper creates an gitea Uploader
func NewRepositoryDumper(ctx context.Context, baseDir, repoOwner, repoName string, migrateReleaseAssets bool) *RepositoryDumper {
	baseDir = filepath.Join(baseDir, repoOwner, repoName)
	return &RepositoryDumper{
		ctx:                  ctx,
		baseDir:              baseDir,
		repoOwner:            repoOwner,
		repoName:             repoName,
		prHeadCache:          make(map[string]struct{}),
		commentFiles: make(map[int64]*os.File),
		reviewFiles: make(map[int64]*os.File),
		migrateReleaseAssets: migrateReleaseAssets,
	}
}

// MaxBatchInsertSize returns the table's max batch insert size
func (g *RepositoryDumper) MaxBatchInsertSize(tp string) int {
	return 1000
}

func (g *RepositoryDumper) gitPath() string {
	return filepath.Join(g.baseDir, "git")
}

func (g *RepositoryDumper) wikiPath() string {
	return filepath.Join(g.baseDir, "wiki")
}

func (g *RepositoryDumper) topicDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) milestoneDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) labelDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) releaseDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) issueDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) commentDir() string {
	return filepath.Join(g.baseDir, "comments")
}

func (g *RepositoryDumper) pullrequestDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryDumper) reviewDir() string {
	return filepath.Join(g.baseDir, "reviews")
}

// CreateRepo creates a repository
func (g *RepositoryDumper) CreateRepo(repo *base.Repository, opts base.MigrateOptions) error {
	if err := os.MkdirAll(g.baseDir, os.ModePerm); err != nil {
		return err
	}

	var remoteAddr = repo.CloneURL
	if len(opts.AuthToken) > 0 || len(opts.AuthUsername) > 0 {
		u, err := url.Parse(repo.CloneURL)
		if err != nil {
			return err
		}
		u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
		if len(opts.AuthToken) > 0 {
			u.User = url.UserPassword("oauth2", opts.AuthToken)
		}
		remoteAddr = u.String()
	}

	f, err := os.Create(filepath.Join(g.baseDir, "repo.yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := yaml.Marshal(map[string]interface{}{
		"name": repo.Name,
		"owner": repo.Owner,
		"description": repo.Description,
		"clone_addr": remoteAddr,
		"original_url": repo.OriginalURL,
		"is_private": opts.Private,
		"auth_username": opts.AuthUsername,
		"auth_password": opts.AuthPassword,
		"auth_token": opts.AuthToken,
		"service_type": opts.GitServiceType,
		"wiki": opts.Wiki,
		"issues": opts.Issues,
		"milestones": opts.Milestones,
		"labels": opts.Labels,
		"releases": opts.Releases,
		"comments": opts.Comments,
		"pulls": opts.PullRequests,
		"assets": opts.ReleaseAssets,
	})
	if err != nil {
		return err
	}

	if _, err := f.Write(bs); err != nil {
		return err
	}

	repoPath := g.gitPath()
	if err := os.MkdirAll(repoPath, os.ModePerm); err != nil {
		return err
	}

	migrateTimeout := 2 * time.Hour

	err = git.Clone(remoteAddr, repoPath, git.CloneRepoOptions{
		Mirror:  true,
		Quiet:   true,
		Timeout: migrateTimeout,
	})
	if err != nil {
		return fmt.Errorf("Clone: %v", err)
	}

	if opts.Wiki {
		wikiPath := g.wikiPath()
		wikiRemotePath := repository.WikiRemoteURL(remoteAddr)
		if len(wikiRemotePath) > 0 {
			if err := os.MkdirAll(wikiPath, os.ModePerm); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
			}

			if err := git.Clone(wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:  true,
				Quiet:   true,
				Timeout: migrateTimeout,
				Branch:  "master",
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := os.RemoveAll(wikiPath); err != nil {
					return fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
				}
			}
		}
	}

	g.gitRepo, err = git.OpenRepository(g.gitPath())
	return err
}

// Close closes this uploader
func (g *RepositoryDumper) Close() {
	if g.gitRepo != nil {
		g.gitRepo.Close()
	}
	if g.milestoneFile != nil {
		g.milestoneFile.Close()
	}
	if g.labelFile != nil {
		g.labelFile.Close()
	}
	if g.releaseFile != nil {
		g.releaseFile.Close()
	}
	if g.issueFile != nil {
		g.issueFile.Close()
	}
	for _, f := range g.commentFiles {
		f.Close()
	}
	if g.pullrequestFile != nil {
		g.pullrequestFile.Close()
	}
	for _, f := range g.reviewFiles {
		f.Close()
	}
}

// CreateTopics creates topics
func (g *RepositoryDumper) CreateTopics(topics ...string) error {
	if err := os.MkdirAll(g.topicDir(), os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(g.topicDir(), "topic.yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := yaml.Marshal(map[string]interface{}{
		"topics": topics,
	})
	if err != nil {
		return err
	}

	if _, err := f.Write(bs); err != nil {
		return err
	}

	return nil
}

// CreateMilestones creates milestones
func (g *RepositoryDumper) CreateMilestones(milestones ...*base.Milestone) error {
	var err error
	if g.milestoneFile == nil {
		if err := os.MkdirAll(g.milestoneDir(), os.ModePerm); err != nil {
			return err
		}
		g.milestoneFile, err = os.Create(filepath.Join(g.milestoneDir(), "milestone.yml"))
		if err != nil {
			return err
		}
	}

	bs, err := yaml.Marshal(milestones)
	if err != nil {
		return err
	}

	if _, err := g.milestoneFile.Write(bs); err != nil {
		return err
	}

	return nil
}

// CreateLabels creates labels
func (g *RepositoryDumper) CreateLabels(labels ...*base.Label) error {
	var err error
	if g.labelFile == nil {
		if err := os.MkdirAll(g.labelDir(), os.ModePerm); err != nil {
			return err
		}
		g.labelFile, err = os.Create(filepath.Join(g.labelDir(), "label.yml"))
		if err != nil {
			return err
		}
	}

	bs, err := yaml.Marshal(labels)
	if err != nil {
		return err
	}

	if _, err := g.labelFile.Write(bs); err != nil {
		return err
	}

	return nil
}

// CreateReleases creates releases
func (g *RepositoryDumper) CreateReleases(downloader base.Downloader, releases ...*base.Release) error {
	if g.migrateReleaseAssets {
		for _, release := range releases {
			attachDir := filepath.Join(g.releaseDir(), "release_assets", release.TagName)
			if err := os.MkdirAll(attachDir, os.ModePerm); err != nil {
				return err
			}
			for _, asset := range release.Assets {
				attachLocalPath := filepath.Join(attachDir, asset.Name)
				// download attachment

				err := func(attachLocalPath string) error {
					// FIXME: release ID
					rc, err := downloader.GetAsset(release.TagName, 0, asset.ID)
					if err != nil {
						return err
					}
					defer rc.Close()

					fw, err := os.Create(attachLocalPath)
					if err != nil {
						return fmt.Errorf("Create: %v", err)
					}
					defer fw.Close()

					_, err = io.Copy(fw, rc)
					return err
				}(attachLocalPath)
				if err != nil {
					return err
				}
			}
		}
	}

	var err error
	if g.releaseFile == nil {
		if err := os.MkdirAll(g.releaseDir(), os.ModePerm); err != nil {
			return err
		}
		g.releaseFile, err = os.Create(filepath.Join(g.releaseDir(), "release.yml"))
		if err != nil {
			return err
		}
	}

	bs, err := yaml.Marshal(releases)
	if err != nil {
		return err
	}

	if _, err := g.releaseFile.Write(bs); err != nil {
		return err
	}

	return nil
}

// SyncTags syncs releases with tags in the database
func (g *RepositoryDumper) SyncTags() error {
	return nil
}

// CreateIssues creates issues
func (g *RepositoryDumper) CreateIssues(issues ...*base.Issue) error {
	var err error
	if g.issueFile == nil {
		if err := os.MkdirAll(g.issueDir(), os.ModePerm); err != nil {
			return err
		}
		g.issueFile, err = os.Create(filepath.Join(g.issueDir(), "issue.yml"))
		if err != nil {
			return err
		}
	}

	bs, err := yaml.Marshal(issues)
	if err != nil {
		return err
	}

	if _, err := g.issueFile.Write(bs); err != nil {
		return err
	}

	return nil
}

// CreateComments creates comments of issues
func (g *RepositoryDumper) CreateComments(comments ...*base.Comment) error {
	var commentsMap = make(map[int64][]*base.Comment, len(comments))
	for _, comment := range comments {
		commentsMap[comment.IssueIndex] = append(commentsMap[comment.IssueIndex], comment)
	}

	if err := os.MkdirAll(g.commentDir(), os.ModePerm); err != nil {
		return err
	}

	for issueNumber, comments := range commentsMap {
		var err error
		commentFile := g.commentFiles[issueNumber]
		if commentFile == nil {
			commentFile, err = os.Create(filepath.Join(g.commentDir(), fmt.Sprintf("%d.yml", issueNumber)))
			if err != nil {
				return err
			}
			g.commentFiles[issueNumber] = commentFile
		}

		bs, err := yaml.Marshal(comments)
		if err != nil {
			return err
		}

		if _, err := commentFile.Write(bs); err != nil {
			return err
		}
	}

	return nil
}

// CreatePullRequests creates pull requests
func (g *RepositoryDumper) CreatePullRequests(prs ...*base.PullRequest) error {
	for _, pr := range prs {
		// download patch file
		err := func() error {
			resp, err := http.Get(pr.PatchURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			pullDir := filepath.Join(g.gitPath(), "pulls")
			if err = os.MkdirAll(pullDir, os.ModePerm); err != nil {
				return err
			}
			f, err := os.Create(filepath.Join(pullDir, fmt.Sprintf("%d.patch", pr.Number)))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, resp.Body)
			return err
		}()
		if err != nil {
			return err
		}

		// set head information
		pullHead := filepath.Join(g.gitPath(), "refs", "pull", fmt.Sprintf("%d", pr.Number))
		if err := os.MkdirAll(pullHead, os.ModePerm); err != nil {
			return err
		}
		p, err := os.Create(filepath.Join(pullHead, "head"))
		if err != nil {
			return err
		}
		_, err = p.WriteString(pr.Head.SHA)
		p.Close()
		if err != nil {
			return err
		}

		if pr.IsForkPullRequest() && pr.State != "closed" {
			if pr.Head.OwnerName != "" {
				remote := pr.Head.OwnerName
				_, ok := g.prHeadCache[remote]
				if !ok {
					// git remote add
					err := g.gitRepo.AddRemote(remote, pr.Head.CloneURL, true)
					if err != nil {
						log.Error("AddRemote failed: %s", err)
					} else {
						g.prHeadCache[remote] = struct{}{}
						ok = true
					}
				}

				if ok {
					_, err = git.NewCommand("fetch", remote, pr.Head.Ref).RunInDir(g.gitPath())
					if err != nil {
						log.Error("Fetch branch from %s failed: %v", pr.Head.CloneURL, err)
					} else {
						headBranch := filepath.Join(g.gitPath(), "refs", "heads", pr.Head.OwnerName, pr.Head.Ref)
						if err := os.MkdirAll(filepath.Dir(headBranch), os.ModePerm); err != nil {
							return err
						}
						b, err := os.Create(headBranch)
						if err != nil {
							return err
						}
						_, err = b.WriteString(pr.Head.SHA)
						b.Close()
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	var err error
	if g.pullrequestFile == nil {
		if err := os.MkdirAll(g.pullrequestDir(), os.ModePerm); err != nil {
			return err
		}
		g.pullrequestFile, err = os.Create(filepath.Join(g.pullrequestDir(), "pull_request.yml"))
		if err != nil {
			return err
		}
	}

	bs, err := yaml.Marshal(prs)
	if err != nil {
		return err
	}

	if _, err := g.pullrequestFile.Write(bs); err != nil {
		return err
	}

	return nil
}

// CreateReviews create pull request reviews
func (g *RepositoryDumper) CreateReviews(reviews ...*base.Review) error {
	var reviewsMap = make(map[int64][]*base.Review, len(reviews))
	for _, review := range reviews {
		reviewsMap[review.IssueIndex] = append(reviewsMap[review.IssueIndex], review)
	}

	if err := os.MkdirAll(g.reviewDir(), os.ModePerm); err != nil {
		return err
	}

	for prNumber, reviews := range reviewsMap {
		var err error
		reviewFile := g.reviewFiles[prNumber]
		if reviewFile == nil {
			reviewFile, err = os.Create(filepath.Join(g.reviewDir(), fmt.Sprintf("%d.yml", prNumber)))
			if err != nil {
				return err
			}
			g.reviewFiles[prNumber] = reviewFile
		}

		bs, err := yaml.Marshal(reviews)
		if err != nil {
			return err
		}

		if _, err := reviewFile.Write(bs); err != nil {
			return err
		}
	}

	return nil
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *RepositoryDumper) Rollback() error {
	g.Close()
	return os.RemoveAll(g.baseDir)
}

func (g *RepositoryDumper) Finish() error {
	return nil
}

// DumpRepository dump repository according MigrateOptions to a local directory
func DumpRepository(ctx context.Context, baseDir, ownerName string, opts base.MigrateOptions) error {
	var uploader = NewRepositoryDumper(ctx, baseDir, ownerName, opts.RepoName, opts.ReleaseAssets)
	return MigrateRepositoryWithUploader(ctx, ownerName, opts, uploader)
}

// RestoreRepository restore a repository from the disk directory
func RestoreRepository(ctx context.Context, baseDir string, ownerName, repoName string) error {
	doer, err := models.GetAdminUser()
	if err != nil {
		return err
	}
	var uploader = NewGiteaLocalUploader(ctx, doer, ownerName, repoName)
	var downloader = NewRepositoryRestorer(ctx, baseDir, ownerName, repoName)
	return DoMigrateRepository(downloader, uploader, base.MigrateOptions{
		Wiki: true,
		Issues: true,
		Milestones: true,
		Labels: true,
		Releases: true,
		Comments: true,
		PullRequests: true,
		ReleaseAssets: true,
	})
}