// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	repo_model "gitea.dev/models/repo"
	system_model "gitea.dev/models/system"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/egress"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// MigrateOptions is equal to base.MigrateOptions
type MigrateOptions = base.MigrateOptions

var factories []base.DownloaderFactory

// RegisterDownloaderFactory registers a downloader factory
func RegisterDownloaderFactory(factory base.DownloaderFactory) {
	factories = append(factories, factory)
}

// IsMigrateURLAllowedForService checks the URL like IsMigrateURLAllowed, and additionally rejects local paths
// unless it is a plain git migration, because other services are accessed by their remote API.
func IsMigrateURLAllowedForService(remoteURL string, serviceType structs.GitServiceType, doer *user_model.User) error {
	if serviceType != structs.PlainGitService {
		if u, err := url.Parse(remoteURL); err == nil && (u.Scheme == "file" || u.Scheme == "") {
			return &git.ErrInvalidCloneAddr{Host: remoteURL, IsProtocolInvalid: true}
		}
	}
	return IsMigrateURLAllowed(remoteURL, doer)
}

// IsMigrateURLAllowed checks if an URL is allowed to be migrated from
func IsMigrateURLAllowed(remoteURL string, doer *user_model.User) error {
	// Remote address can be HTTP/HTTPS/Git URL or local path.
	u, err := url.Parse(remoteURL)
	if err != nil {
		return &git.ErrInvalidCloneAddr{IsURLError: true, Host: remoteURL}
	}

	if u.Scheme == "file" || u.Scheme == "" {
		if !doer.CanImportLocal() {
			return &git.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsPermissionDenied: true, LocalPath: true}
		}
		isAbs := filepath.IsAbs(u.Host + u.Path)
		if !isAbs {
			return &git.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}
		isDir, err := util.IsDir(u.Host + u.Path)
		if err != nil {
			log.Error("Unable to check if %s is a directory: %v", u.Host+u.Path, err)
			return err
		}
		if !isDir {
			return &git.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}

		return nil
	}

	if u.Scheme == "git" && u.Port() != "" && (strings.Contains(remoteURL, "%0d") || strings.Contains(remoteURL, "%0a")) {
		return &git.ErrInvalidCloneAddr{Host: u.Host, IsURLError: true}
	}

	if u.Opaque != "" || u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "git" {
		return &git.ErrInvalidCloneAddr{Host: u.Host, IsProtocolInvalid: true, IsPermissionDenied: true, IsURLError: true}
	}

	if err := egress.NewMigrationPolicy().CheckHostIPs(u); err != nil {
		return &git.ErrInvalidCloneAddr{Host: u.Hostname(), IsPermissionDenied: true}
	}
	return nil
}

// MigrateRepository migrate repository according MigrateOptions
func MigrateRepository(ctx context.Context, doer *user_model.User, ownerName string, opts base.MigrateOptions, messenger base.Messenger) (*repo_model.Repository, error) {
	err := IsMigrateURLAllowedForService(opts.CloneAddr, opts.GitServiceType, doer)
	if err != nil {
		return nil, err
	}
	if opts.LFS && len(opts.LFSEndpoint) > 0 {
		err := IsMigrateURLAllowed(opts.LFSEndpoint, doer)
		if err != nil {
			return nil, err
		}
	}
	downloader, err := newDownloader(ctx, ownerName, opts)
	if err != nil {
		return nil, err
	}

	uploader := NewGiteaLocalUploader(ctx, doer, ownerName, opts.RepoName)
	uploader.gitServiceType = opts.GitServiceType

	if err := migrateRepository(ctx, doer, downloader, wrapGithubIssueRecoveryUploader(uploader, downloader), opts, messenger); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
		}
		noticeMsg := fmt.Sprintf("Migrate repository (%s/%s) from %s failed: %v", ownerName, opts.RepoName, util.SanitizeCredentialURLs(opts.OriginalURL), util.SanitizeErrorCredentialURLs(err))
		if err2 := system_model.CreateRepositoryNotice(noticeMsg); err2 != nil {
			log.Error("create repository notice failed: ", err2)
		}
		return nil, err
	}
	return uploader.repo, nil
}

func newDownloader(ctx context.Context, ownerName string, opts base.MigrateOptions) (base.Downloader, error) {
	var (
		downloader base.Downloader
		err        error
	)

	for _, factory := range factories {
		if factory.GitServiceType() == opts.GitServiceType {
			downloader, err = factory.New(ctx, opts)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	if downloader == nil {
		opts.Wiki = true
		opts.Milestones = false
		opts.Labels = false
		opts.Releases = false
		opts.Comments = false
		opts.Issues = false
		opts.PullRequests = false
		downloader = NewPlainGitDownloader(ownerName, opts.RepoName, opts.CloneAddr)
		log.Trace("Will migrate from git: %s", opts.OriginalURL)
	}

	if setting.Migrations.MaxAttempts > 1 {
		downloader = base.NewRetryDownloader(downloader, setting.Migrations.MaxAttempts, setting.Migrations.RetryBackoff)
	}
	return downloader, nil
}

func migrateRepository(ctx context.Context, doer *user_model.User, downloader base.Downloader, uploader base.Uploader, opts base.MigrateOptions, messenger base.Messenger) error {
	if messenger == nil {
		messenger = base.NilMessenger
	}

	repo, err := downloader.GetRepoInfo(ctx)
	if err != nil {
		if !base.IsErrNotSupported(err) {
			return err
		}
		log.Info("migrating repo infos is not supported, ignored")
	}
	repo.IsPrivate = opts.Private
	repo.IsMirror = opts.Mirror
	if opts.Description != "" {
		repo.Description = opts.Description
	}
	if repo.CloneURL, err = downloader.FormatCloneURL(opts, repo.CloneURL); err != nil {
		return err
	}

	// SECURITY: If the downloader is not a RepositoryRestorer then we need to recheck the CloneURL
	if _, ok := downloader.(*RepositoryRestorer); !ok {
		// Now the clone URL can be rewritten by the downloader so we must recheck
		if err := IsMigrateURLAllowed(repo.CloneURL, doer); err != nil {
			return err
		}

		// SECURITY: Ensure that we haven't been redirected from an external to a local filesystem
		// Now we know all of these must parse
		cloneAddrURL, _ := url.Parse(opts.CloneAddr)
		cloneURL, _ := url.Parse(repo.CloneURL)

		if cloneURL.Scheme == "file" || cloneURL.Scheme == "" {
			if cloneAddrURL.Scheme != "file" && cloneAddrURL.Scheme != "" {
				return errors.New("repo info has changed from external to local filesystem")
			}
		}

		// We don't actually need to check the OriginalURL as it isn't used anywhere
	}

	log.Trace("migrating git data from %s", util.SanitizeCredentialURLs(repo.CloneURL))
	messenger("repo.migrate.migrating_git")
	if err = uploader.CreateRepo(ctx, repo, opts); err != nil {
		return err
	}
	defer uploader.Close()

	if err := migrateRepositoryMetadata(ctx, downloader, uploader, opts, messenger); err != nil {
		return err
	}
	if err := migrateRepositoryIssueData(ctx, downloader, uploader, opts, messenger); err != nil {
		return err
	}
	return uploader.Finish(ctx)
}

func migrateRepositoryMetadata(ctx context.Context, downloader base.Downloader, uploader base.Uploader, opts base.MigrateOptions, messenger base.Messenger) error {
	log.Trace("migrating topics")
	messenger("repo.migrate.migrating_topics")
	topics, err := downloader.GetTopics(ctx)
	if err != nil {
		if !base.IsErrNotSupported(err) {
			return err
		}
		log.Warn("migrating topics is not supported, ignored")
	}
	if len(topics) != 0 {
		if err := uploader.CreateTopics(ctx, topics...); err != nil {
			return err
		}
	}

	if opts.Milestones {
		log.Trace("migrating milestones")
		messenger("repo.migrate.migrating_milestones")
		if err := migrateMetadataBatches(ctx, uploader, "milestone", downloader.GetMilestones, uploader.CreateMilestones); err != nil {
			return err
		}
	}
	if opts.Labels {
		log.Trace("migrating labels")
		messenger("repo.migrate.migrating_labels")
		if err := migrateMetadataBatches(ctx, uploader, "label", downloader.GetLabels, uploader.CreateLabels); err != nil {
			return err
		}
	}
	if opts.Releases {
		log.Trace("migrating releases")
		messenger("repo.migrate.migrating_releases")
		if err := migrateMetadataBatches(ctx, uploader, "release", downloader.GetReleases, uploader.CreateReleases); err != nil {
			return err
		}
		// Once all releases (if any) are inserted, sync any remaining non-release tags
		return uploader.SyncTags(ctx)
	}
	return nil
}

func migrateMetadataBatches[T any](ctx context.Context, uploader base.Uploader, entity string,
	download func(context.Context) ([]T, error), upload func(context.Context, ...T) error,
) error {
	items, err := download(ctx)
	if err != nil {
		if !base.IsErrNotSupported(err) {
			return err
		}
		log.Warn("migrating %ss is not supported, ignored", entity)
	}
	for batch := range slices.Chunk(items, uploader.MaxBatchInsertSize(entity)) {
		if err := upload(ctx, batch...); err != nil {
			return err
		}
	}
	return nil
}

func migrateRepositoryIssueData(ctx context.Context, downloader base.Downloader, uploader base.Uploader, opts base.MigrateOptions, messenger base.Messenger) error {
	commentBatchSize := uploader.MaxBatchInsertSize("comment")
	supportAllComments := downloader.SupportGetRepoComments()
	downloadComments := func(ctx context.Context, item base.Reviewable) ([]*base.Comment, error) {
		comments, _, err := downloader.GetComments(ctx, item)
		return comments, err
	}

	issueIndexes, pullIndexes := container.Set[int64]{}, container.Set[int64]{}
	if opts.Issues {
		log.Trace("migrating issues and comments")
		messenger("repo.migrate.migrating_issues")
		if err := migrateIssuePages(ctx, "issues", issueIndexes, uploader.MaxBatchInsertSize("issue"), downloader.GetIssues, uploader.CreateIssues, func(issues []*base.Issue) error {
			if opts.Comments && !supportAllComments {
				return migrateItemChildren(ctx, "comments", issues, commentBatchSize, downloadComments, uploader.CreateComments)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if opts.PullRequests {
		log.Trace("migrating pull requests and comments")
		messenger("repo.migrate.migrating_pulls")
		if err := migrateIssuePages(ctx, "pull requests", pullIndexes, uploader.MaxBatchInsertSize("pullrequest"), downloader.GetPullRequests, uploader.CreatePullRequests, func(pulls []*base.PullRequest) error {
			if !opts.Comments {
				return nil
			}
			if !supportAllComments {
				if err := migrateItemChildren(ctx, "comments", pulls, commentBatchSize, downloadComments, uploader.CreateComments); err != nil {
					return err
				}
			}
			return migrateItemChildren(ctx, "reviews", pulls, uploader.MaxBatchInsertSize("review"), downloader.GetReviews, uploader.CreateReviews)
		}); err != nil {
			return err
		}
		if len(pullIndexes) > 0 { // importing pull requests can create head branches for forks that were not migrated
			log.Trace("syncing branches after migrating pull requests")
			if err := uploader.SyncBranches(ctx); err != nil {
				return err
			}
		}
	}

	if !opts.Comments || !supportAllComments {
		return nil
	}
	log.Trace("migrating comments")
	for page := 1; ; page++ {
		comments, isEnd, err := downloader.GetAllComments(ctx, page, commentBatchSize)
		if err != nil {
			return err
		}
		comments = slices.DeleteFunc(comments, func(comment *base.Comment) bool {
			return !issueIndexes.Contains(comment.IssueIndex) && !pullIndexes.Contains(comment.IssueIndex)
		})
		if err := uploader.CreateComments(ctx, comments...); err != nil {
			return err
		}
		if isEnd {
			return nil
		}
	}
}

func migrateIssuePages[T base.Commentable](ctx context.Context, name string, indexes container.Set[int64], pageSize int,
	download func(context.Context, int, int) ([]T, bool, error),
	upload func(context.Context, ...T) error,
	afterPage func([]T) error,
) error {
	for page := 1; ; page++ {
		items, isEnd, err := download(ctx, page, pageSize)
		if err != nil {
			if !base.IsErrNotSupported(err) {
				return err
			}
			log.Warn("migrating %s is not supported, ignored", name)
			return nil
		}
		items = slices.DeleteFunc(items, func(item T) bool {
			return !indexes.Add(item.GetLocalIndex()) // deletions at the source can shift previously seen items into this page
		})
		if err := upload(ctx, items...); err != nil {
			return err
		}
		if err := afterPage(items); err != nil {
			return err
		}
		if isEnd {
			return nil
		}
	}
}

func migrateItemChildren[T base.Reviewable, C any](ctx context.Context, name string, items []T, batchSize int,
	download func(context.Context, base.Reviewable) ([]C, error), upload func(context.Context, ...C) error,
) error {
	pending := make([]C, 0, batchSize)
	for _, item := range items {
		log.Trace("migrating %s of #%d", name, item.GetLocalIndex())
		children, err := download(ctx, item)
		if err != nil {
			if !base.IsErrNotSupported(err) {
				return err
			}
			log.Warn("migrating %s is not supported, ignored", name)
			break
		}
		pending = append(pending, children...)
		if len(pending) >= batchSize {
			if err := upload(ctx, pending[:batchSize]...); err != nil {
				return err
			}
			pending = pending[batchSize:]
		}
	}
	if len(pending) > 0 {
		return upload(ctx, pending...)
	}
	return nil
}
