// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var _ base.Uploader = &RepositoryDumper{}

// RepositoryDumper implements an Uploader to the local directory
type RepositoryDumper struct {
	baseDir         string
	repoOwner       string
	repoName        string
	opts            base.MigrateOptions
	milestoneFile   *os.File
	labelFile       *os.File
	releaseFile     *os.File
	issueFile       *os.File
	commentFiles    map[int64]*os.File
	pullrequestFile *os.File
	reviewFiles     map[int64]*os.File

	gitRepo     *git.Repository
	prHeadCache map[string]string
}

// NewRepositoryDumper creates an gitea Uploader
func NewRepositoryDumper(ctx context.Context, baseDir, repoOwner, repoName string, opts base.MigrateOptions) (*RepositoryDumper, error) {
	baseDir = filepath.Join(baseDir, repoOwner, repoName)
	if err := os.MkdirAll(baseDir, os.ModePerm); err != nil {
		return nil, err
	}
	return &RepositoryDumper{
		opts:         opts,
		baseDir:      baseDir,
		repoOwner:    repoOwner,
		repoName:     repoName,
		prHeadCache:  make(map[string]string),
		commentFiles: make(map[int64]*os.File),
		reviewFiles:  make(map[int64]*os.File),
	}, nil
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

func (g *RepositoryDumper) commentDir() string {
	return filepath.Join(g.baseDir, "comments")
}

func (g *RepositoryDumper) reviewDir() string {
	return filepath.Join(g.baseDir, "reviews")
}

func (g *RepositoryDumper) setURLToken(remoteAddr string) (string, error) {
	if len(g.opts.AuthToken) > 0 || len(g.opts.AuthUsername) > 0 {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", err
		}
		u.User = url.UserPassword(g.opts.AuthUsername, g.opts.AuthPassword)
		if len(g.opts.AuthToken) > 0 {
			u.User = url.UserPassword("oauth2", g.opts.AuthToken)
		}
		remoteAddr = u.String()
	}

	return remoteAddr, nil
}

// CreateRepo creates a repository
func (g *RepositoryDumper) CreateRepo(ctx context.Context, repo *base.Repository, opts base.MigrateOptions) error {
	f, err := os.Create(filepath.Join(g.baseDir, "repo.yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := yaml.Marshal(map[string]any{
		"name":         repo.Name,
		"owner":        repo.Owner,
		"description":  repo.Description,
		"clone_addr":   opts.CloneAddr,
		"original_url": repo.OriginalURL,
		"is_private":   opts.Private,
		"service_type": opts.GitServiceType,
		"wiki":         opts.Wiki,
		"issues":       opts.Issues,
		"milestones":   opts.Milestones,
		"labels":       opts.Labels,
		"releases":     opts.Releases,
		"comments":     opts.Comments,
		"pulls":        opts.PullRequests,
		"assets":       opts.ReleaseAssets,
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

	remoteAddr, err := g.setURLToken(repo.CloneURL)
	if err != nil {
		return err
	}

	err = git.Clone(ctx, remoteAddr, repoPath, git.CloneRepoOptions{
		Mirror:        true,
		Quiet:         true,
		Timeout:       migrateTimeout,
		SkipTLSVerify: setting.Migrations.SkipTLSVerify,
	})
	if err != nil {
		return fmt.Errorf("Clone: %w", err)
	}
	if err := git.WriteCommitGraph(ctx, repoPath); err != nil {
		return err
	}

	if opts.Wiki {
		wikiPath := g.wikiPath()
		wikiRemotePath := repository.WikiRemoteURL(ctx, remoteAddr)
		if len(wikiRemotePath) > 0 {
			if err := os.MkdirAll(wikiPath, os.ModePerm); err != nil {
				return fmt.Errorf("Failed to remove %s: %w", wikiPath, err)
			}

			if err := git.Clone(ctx, wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:        true,
				Quiet:         true,
				Timeout:       migrateTimeout,
				Branch:        "master",
				SkipTLSVerify: setting.Migrations.SkipTLSVerify,
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := os.RemoveAll(wikiPath); err != nil {
					return fmt.Errorf("Failed to remove %s: %w", wikiPath, err)
				}
			} else if err := git.WriteCommitGraph(ctx, wikiPath); err != nil {
				return err
			}
		}
	}

	g.gitRepo, err = git.OpenRepository(ctx, g.gitPath())
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
func (g *RepositoryDumper) CreateTopics(_ context.Context, topics ...string) error {
	f, err := os.Create(filepath.Join(g.baseDir, "topic.yml"))
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := yaml.Marshal(map[string]any{
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
func (g *RepositoryDumper) CreateMilestones(_ context.Context, milestones ...*base.Milestone) error {
	var err error
	if g.milestoneFile == nil {
		g.milestoneFile, err = os.Create(filepath.Join(g.baseDir, "milestone.yml"))
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
func (g *RepositoryDumper) CreateLabels(_ context.Context, labels ...*base.Label) error {
	var err error
	if g.labelFile == nil {
		g.labelFile, err = os.Create(filepath.Join(g.baseDir, "label.yml"))
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
func (g *RepositoryDumper) CreateReleases(_ context.Context, releases ...*base.Release) error {
	if g.opts.ReleaseAssets {
		for _, release := range releases {
			attachDir := filepath.Join("release_assets", release.TagName)
			if err := os.MkdirAll(filepath.Join(g.baseDir, attachDir), os.ModePerm); err != nil {
				return err
			}
			for _, asset := range release.Assets {
				attachLocalPath := filepath.Join(attachDir, asset.Name)

				// SECURITY: We cannot check the DownloadURL and DownloadFunc are safe here
				// ... we must assume that they are safe and simply download the attachment
				// download attachment
				err := func(attachPath string) error {
					var rc io.ReadCloser
					var err error
					if asset.DownloadURL == nil {
						rc, err = asset.DownloadFunc()
						if err != nil {
							return err
						}
					} else {
						resp, err := http.Get(*asset.DownloadURL)
						if err != nil {
							return err
						}
						rc = resp.Body
					}
					defer rc.Close()

					fw, err := os.Create(attachPath)
					if err != nil {
						return fmt.Errorf("create: %w", err)
					}
					defer fw.Close()

					_, err = io.Copy(fw, rc)
					return err
				}(filepath.Join(g.baseDir, attachLocalPath))
				if err != nil {
					return err
				}
				asset.DownloadURL = &attachLocalPath // to save the filepath on the yml file, change the source
			}
		}
	}

	var err error
	if g.releaseFile == nil {
		g.releaseFile, err = os.Create(filepath.Join(g.baseDir, "release.yml"))
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
func (g *RepositoryDumper) SyncTags(ctx context.Context) error {
	return nil
}

// CreateIssues creates issues
func (g *RepositoryDumper) CreateIssues(_ context.Context, issues ...*base.Issue) error {
	var err error
	if g.issueFile == nil {
		g.issueFile, err = os.Create(filepath.Join(g.baseDir, "issue.yml"))
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

func (g *RepositoryDumper) createItems(dir string, itemFiles map[int64]*os.File, itemsMap map[int64][]any) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	for number, items := range itemsMap {
		if err := g.encodeItems(number, items, dir, itemFiles); err != nil {
			return err
		}
	}

	return nil
}

func (g *RepositoryDumper) encodeItems(number int64, items []any, dir string, itemFiles map[int64]*os.File) error {
	itemFile := itemFiles[number]
	if itemFile == nil {
		var err error
		itemFile, err = os.Create(filepath.Join(dir, fmt.Sprintf("%d.yml", number)))
		if err != nil {
			return err
		}
		itemFiles[number] = itemFile
	}

	encoder := yaml.NewEncoder(itemFile)
	defer encoder.Close()

	return encoder.Encode(items)
}

// CreateComments creates comments of issues
func (g *RepositoryDumper) CreateComments(_ context.Context, comments ...*base.Comment) error {
	commentsMap := make(map[int64][]any, len(comments))
	for _, comment := range comments {
		commentsMap[comment.IssueIndex] = append(commentsMap[comment.IssueIndex], comment)
	}

	return g.createItems(g.commentDir(), g.commentFiles, commentsMap)
}

func (g *RepositoryDumper) handlePullRequest(ctx context.Context, pr *base.PullRequest) error {
	// SECURITY: this pr must have been ensured safe
	if !pr.EnsuredSafe {
		log.Error("PR #%d in %s/%s has not been checked for safety ... We will ignore this.", pr.Number, g.repoOwner, g.repoName)
		return fmt.Errorf("unsafe PR #%d", pr.Number)
	}

	// First we download the patch file
	err := func() error {
		// if the patchURL is empty there is nothing to download
		if pr.PatchURL == "" {
			return nil
		}

		// SECURITY: We will assume that the pr.PatchURL has been checked
		// pr.PatchURL maybe a local file - but note EnsureSafe should be asserting that this safe
		u, err := g.setURLToken(pr.PatchURL)
		if err != nil {
			return err
		}

		// SECURITY: We will assume that the pr.PatchURL has been checked
		// pr.PatchURL maybe a local file - but note EnsureSafe should be asserting that this safe
		resp, err := http.Get(u) // TODO: This probably needs to use the downloader as there may be rate limiting issues here
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		pullDir := filepath.Join(g.gitPath(), "pulls")
		if err = os.MkdirAll(pullDir, os.ModePerm); err != nil {
			return err
		}
		fPath := filepath.Join(pullDir, fmt.Sprintf("%d.patch", pr.Number))
		f, err := os.Create(fPath)
		if err != nil {
			return err
		}
		defer f.Close()

		// TODO: Should there be limits on the size of this file?
		if _, err = io.Copy(f, resp.Body); err != nil {
			return err
		}
		pr.PatchURL = "git/pulls/" + fmt.Sprintf("%d.patch", pr.Number)

		return nil
	}()
	if err != nil {
		log.Error("PR #%d in %s/%s unable to download patch: %v", pr.Number, g.repoOwner, g.repoName, err)
		return err
	}

	isFork := pr.IsForkPullRequest()

	// Even if it's a forked repo PR, we have to change head info as the same as the base info
	oldHeadOwnerName := pr.Head.OwnerName
	pr.Head.OwnerName, pr.Head.RepoName = pr.Base.OwnerName, pr.Base.RepoName

	if !isFork || pr.State == "closed" {
		return nil
	}

	// OK we want to fetch the current head as a branch from its CloneURL

	// 1. Is there a head clone URL available?
	// 2. Is there a head ref available?
	if pr.Head.CloneURL == "" || pr.Head.Ref == "" {
		// Set head information if pr.Head.SHA is available
		if pr.Head.SHA != "" {
			_, _, err = git.NewCommand("update-ref", "--no-deref").AddDynamicArguments(pr.GetGitRefName(), pr.Head.SHA).RunStdString(ctx, &git.RunOpts{Dir: g.gitPath()})
			if err != nil {
				log.Error("PR #%d in %s/%s unable to update-ref for pr HEAD: %v", pr.Number, g.repoOwner, g.repoName, err)
			}
		}
		return nil
	}

	// 3. We need to create a remote for this clone url
	// ... maybe we already have a name for this remote
	remote, ok := g.prHeadCache[pr.Head.CloneURL+":"]
	if !ok {
		// ... let's try ownername as a reasonable name
		remote = oldHeadOwnerName
		if !git.IsValidRefPattern(remote) {
			// ... let's try something less nice
			remote = "head-pr-" + strconv.FormatInt(pr.Number, 10)
		}
		// ... now add the remote
		err := g.gitRepo.AddRemote(remote, pr.Head.CloneURL, true)
		if err != nil {
			log.Error("PR #%d in %s/%s AddRemote[%s] failed: %v", pr.Number, g.repoOwner, g.repoName, remote, err)
		} else {
			g.prHeadCache[pr.Head.CloneURL+":"] = remote
			ok = true
		}
	}
	if !ok {
		// Set head information if pr.Head.SHA is available
		if pr.Head.SHA != "" {
			_, _, err = git.NewCommand("update-ref", "--no-deref").AddDynamicArguments(pr.GetGitRefName(), pr.Head.SHA).RunStdString(ctx, &git.RunOpts{Dir: g.gitPath()})
			if err != nil {
				log.Error("PR #%d in %s/%s unable to update-ref for pr HEAD: %v", pr.Number, g.repoOwner, g.repoName, err)
			}
		}

		return nil
	}

	// 4. Check if we already have this ref?
	localRef, ok := g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref]
	if !ok {
		// ... We would normally name this migrated branch as <OwnerName>/<HeadRef> but we need to ensure that is safe
		localRef = git.SanitizeRefPattern(oldHeadOwnerName + "/" + pr.Head.Ref)

		// ... Now we must assert that this does not exist
		if g.gitRepo.IsBranchExist(localRef) {
			localRef = "head-pr-" + strconv.FormatInt(pr.Number, 10) + "/" + localRef
			i := 0
			for g.gitRepo.IsBranchExist(localRef) {
				if i > 5 {
					// ... We tried, we really tried but this is just a seriously unfriendly repo
					return fmt.Errorf("unable to create unique local reference from %s", pr.Head.Ref)
				}
				// OK just try some uuids!
				localRef = git.SanitizeRefPattern("head-pr-" + strconv.FormatInt(pr.Number, 10) + uuid.New().String())
				i++
			}
		}

		fetchArg := pr.Head.Ref + ":" + git.BranchPrefix + localRef
		if strings.HasPrefix(fetchArg, "-") {
			fetchArg = git.BranchPrefix + fetchArg
		}

		_, _, err = git.NewCommand("fetch", "--no-tags").AddDashesAndList(remote, fetchArg).RunStdString(ctx, &git.RunOpts{Dir: g.gitPath()})
		if err != nil {
			log.Error("Fetch branch from %s failed: %v", pr.Head.CloneURL, err)
			// We need to continue here so that the Head.Ref is reset and we attempt to set the gitref for the PR
			// (This last step will likely fail but we should try to do as much as we can.)
		} else {
			// Cache the localRef as the Head.Ref - if we've failed we can always try again.
			g.prHeadCache[pr.Head.CloneURL+":"+pr.Head.Ref] = localRef
		}
	}

	// Set the pr.Head.Ref to the localRef
	pr.Head.Ref = localRef

	// 5. Now if pr.Head.SHA == "" we should recover this to the head of this branch
	if pr.Head.SHA == "" {
		headSha, err := g.gitRepo.GetBranchCommitID(localRef)
		if err != nil {
			log.Error("unable to get head SHA of local head for PR #%d from %s in %s/%s. Error: %v", pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
			return nil
		}
		pr.Head.SHA = headSha
	}
	if pr.Head.SHA != "" {
		_, _, err = git.NewCommand("update-ref", "--no-deref").AddDynamicArguments(pr.GetGitRefName(), pr.Head.SHA).RunStdString(ctx, &git.RunOpts{Dir: g.gitPath()})
		if err != nil {
			log.Error("unable to set %s as the local head for PR #%d from %s in %s/%s. Error: %v", pr.Head.SHA, pr.Number, pr.Head.Ref, g.repoOwner, g.repoName, err)
		}
	}

	return nil
}

// CreatePullRequests creates pull requests
func (g *RepositoryDumper) CreatePullRequests(ctx context.Context, prs ...*base.PullRequest) error {
	var err error
	if g.pullrequestFile == nil {
		if err := os.MkdirAll(g.baseDir, os.ModePerm); err != nil {
			return err
		}
		g.pullrequestFile, err = os.Create(filepath.Join(g.baseDir, "pull_request.yml"))
		if err != nil {
			return err
		}
	}

	encoder := yaml.NewEncoder(g.pullrequestFile)
	defer encoder.Close()

	count := 0
	for i := 0; i < len(prs); i++ {
		pr := prs[i]
		if err := g.handlePullRequest(ctx, pr); err != nil {
			log.Error("PR #%d in %s/%s failed - skipping", pr.Number, g.repoOwner, g.repoName, err)
			continue
		}
		prs[count] = pr
		count++
	}
	prs = prs[:count]

	return encoder.Encode(prs)
}

// CreateReviews create pull request reviews
func (g *RepositoryDumper) CreateReviews(_ context.Context, reviews ...*base.Review) error {
	reviewsMap := make(map[int64][]any, len(reviews))
	for _, review := range reviews {
		reviewsMap[review.IssueIndex] = append(reviewsMap[review.IssueIndex], review)
	}

	return g.createItems(g.reviewDir(), g.reviewFiles, reviewsMap)
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *RepositoryDumper) Rollback() error {
	g.Close()
	return os.RemoveAll(g.baseDir)
}

// Finish when migrating succeed, this will update something.
func (g *RepositoryDumper) Finish(_ context.Context) error {
	return nil
}

// DumpRepository dump repository according MigrateOptions to a local directory
func DumpRepository(ctx context.Context, baseDir, ownerName string, opts base.MigrateOptions) error {
	doer, err := user_model.GetAdminUser(ctx)
	if err != nil {
		return err
	}
	downloader, err := newDownloader(ctx, ownerName, opts)
	if err != nil {
		return err
	}
	uploader, err := NewRepositoryDumper(ctx, baseDir, ownerName, opts.RepoName, opts)
	if err != nil {
		return err
	}

	if err := migrateRepository(ctx, doer, downloader, uploader, opts, nil); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
		}
		return err
	}
	return nil
}

func updateOptionsUnits(opts *base.MigrateOptions, units []string) error {
	if len(units) == 0 {
		opts.Wiki = true
		opts.Issues = true
		opts.Milestones = true
		opts.Labels = true
		opts.Releases = true
		opts.Comments = true
		opts.PullRequests = true
		opts.ReleaseAssets = true
	} else {
		for _, unit := range units {
			switch strings.ToLower(strings.TrimSpace(unit)) {
			case "":
				continue
			case "wiki":
				opts.Wiki = true
			case "issues":
				opts.Issues = true
			case "milestones":
				opts.Milestones = true
			case "labels":
				opts.Labels = true
			case "releases":
				opts.Releases = true
			case "release_assets":
				opts.ReleaseAssets = true
			case "comments":
				opts.Comments = true
			case "pull_requests":
				opts.PullRequests = true
			default:
				return errors.New("invalid unit: " + unit)
			}
		}
	}
	return nil
}

// RestoreRepository restore a repository from the disk directory
func RestoreRepository(ctx context.Context, baseDir, ownerName, repoName string, units []string, validation bool) error {
	doer, err := user_model.GetAdminUser(ctx)
	if err != nil {
		return err
	}
	uploader := NewGiteaLocalUploader(ctx, doer, ownerName, repoName)
	downloader, err := NewRepositoryRestorer(ctx, baseDir, ownerName, repoName, validation)
	if err != nil {
		return err
	}
	opts, err := downloader.getRepoOptions()
	if err != nil {
		return err
	}
	tp, _ := strconv.Atoi(opts["service_type"])

	migrateOpts := base.MigrateOptions{
		GitServiceType: structs.GitServiceType(tp),
	}
	if err := updateOptionsUnits(&migrateOpts, units); err != nil {
		return err
	}

	if err = migrateRepository(ctx, doer, downloader, uploader, migrateOpts, nil); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
		}
		return err
	}
	return updateMigrationPosterIDByGitService(ctx, structs.GitServiceType(tp))
}
