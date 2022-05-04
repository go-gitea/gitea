// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	admin_model "code.gitea.io/gitea/models/admin"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// MigrateOptions is equal to base.MigrateOptions
type MigrateOptions = base.MigrateOptions

var (
	factories []base.DownloaderFactory

	allowList *hostmatcher.HostMatchList
	blockList *hostmatcher.HostMatchList
)

// RegisterDownloaderFactory registers a downloader factory
func RegisterDownloaderFactory(factory base.DownloaderFactory) {
	factories = append(factories, factory)
}

// IsMigrateURLAllowed checks if an URL is allowed to be migrated from
func IsMigrateURLAllowed(remoteURL string, doer *user_model.User) error {
	// Remote address can be HTTP/HTTPS/Git URL or local path.
	u, err := url.Parse(remoteURL)
	if err != nil {
		return &models.ErrInvalidCloneAddr{IsURLError: true}
	}

	if u.Scheme == "file" || u.Scheme == "" {
		if !doer.CanImportLocal() {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsPermissionDenied: true, LocalPath: true}
		}
		isAbs := filepath.IsAbs(u.Host + u.Path)
		if !isAbs {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}
		isDir, err := util.IsDir(u.Host + u.Path)
		if err != nil {
			log.Error("Unable to check if %s is a directory: %v", u.Host+u.Path, err)
			return err
		}
		if !isDir {
			return &models.ErrInvalidCloneAddr{Host: "<LOCAL_FILESYSTEM>", IsInvalidPath: true, LocalPath: true}
		}

		return nil
	}

	if u.Scheme == "git" && u.Port() != "" && (strings.Contains(remoteURL, "%0d") || strings.Contains(remoteURL, "%0a")) {
		return &models.ErrInvalidCloneAddr{Host: u.Host, IsURLError: true}
	}

	if u.Opaque != "" || u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "git" {
		return &models.ErrInvalidCloneAddr{Host: u.Host, IsProtocolInvalid: true, IsPermissionDenied: true, IsURLError: true}
	}

	hostName, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		// u.Host can be "host" or "host:port"
		err = nil //nolint
		hostName = u.Host
	}

	// some users only use proxy, there is no DNS resolver. it's safe to ignore the LookupIP error
	addrList, _ := net.LookupIP(hostName)

	var ipAllowed bool
	var ipBlocked bool
	for _, addr := range addrList {
		ipAllowed = ipAllowed || allowList.MatchIPAddr(addr)
		ipBlocked = ipBlocked || blockList.MatchIPAddr(addr)
	}
	var blockedError error
	if blockList.MatchHostName(hostName) || ipBlocked {
		blockedError = &models.ErrInvalidCloneAddr{Host: u.Host, IsPermissionDenied: true}
	}
	// if we have an allow-list, check the allow-list first
	if !allowList.IsEmpty() {
		if !allowList.MatchHostName(hostName) && !ipAllowed {
			return &models.ErrInvalidCloneAddr{Host: u.Host, IsPermissionDenied: true}
		}
	}
	// otherwise, we always follow the blocked list
	return blockedError
}

// MigrateRepository migrate repository according MigrateOptions
func MigrateRepository(ctx context.Context, doer *user_model.User, ownerName string, opts base.MigrateOptions, messenger base.Messenger) (*repo_model.Repository, error) {
	err := IsMigrateURLAllowed(opts.CloneAddr, doer)
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

	if err := migrateRepository(downloader, uploader, opts, messenger); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
		}
		if err2 := admin_model.CreateRepositoryNotice(fmt.Sprintf("Migrate repository from %s failed: %v", opts.OriginalURL, err)); err2 != nil {
			log.Error("create respotiry notice failed: ", err2)
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
		downloader = base.NewRetryDownloader(ctx, downloader, setting.Migrations.MaxAttempts, setting.Migrations.RetryBackoff)
	}
	return downloader, nil
}

// migrateRepository will download information and then upload it to Uploader, this is a simple
// process for small repository. For a big repository, save all the data to disk
// before upload is better
func migrateRepository(downloader base.Downloader, uploader base.Uploader, opts base.MigrateOptions, messenger base.Messenger) error {
	if messenger == nil {
		messenger = base.NilMessenger
	}

	repo, err := downloader.GetRepoInfo()
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

	log.Trace("migrating git data from %s", repo.CloneURL)
	messenger("repo.migrate.migrating_git")
	if err = uploader.CreateRepo(repo, opts); err != nil {
		return err
	}
	defer uploader.Close()

	log.Trace("migrating topics")
	messenger("repo.migrate.migrating_topics")
	topics, err := downloader.GetTopics()
	if err != nil {
		if !base.IsErrNotSupported(err) {
			return err
		}
		log.Warn("migrating topics is not supported, ignored")
	}
	if len(topics) != 0 {
		if err = uploader.CreateTopics(topics...); err != nil {
			return err
		}
	}

	if opts.Milestones {
		log.Trace("migrating milestones")
		messenger("repo.migrate.migrating_milestones")
		milestones, err := downloader.GetMilestones()
		if err != nil {
			if !base.IsErrNotSupported(err) {
				return err
			}
			log.Warn("migrating milestones is not supported, ignored")
		}

		msBatchSize := uploader.MaxBatchInsertSize("milestone")
		for len(milestones) > 0 {
			if len(milestones) < msBatchSize {
				msBatchSize = len(milestones)
			}

			if err := uploader.CreateMilestones(milestones...); err != nil {
				return err
			}
			milestones = milestones[msBatchSize:]
		}
	}

	if opts.Labels {
		log.Trace("migrating labels")
		messenger("repo.migrate.migrating_labels")
		labels, err := downloader.GetLabels()
		if err != nil {
			if !base.IsErrNotSupported(err) {
				return err
			}
			log.Warn("migrating labels is not supported, ignored")
		}

		lbBatchSize := uploader.MaxBatchInsertSize("label")
		for len(labels) > 0 {
			if len(labels) < lbBatchSize {
				lbBatchSize = len(labels)
			}

			if err := uploader.CreateLabels(labels...); err != nil {
				return err
			}
			labels = labels[lbBatchSize:]
		}
	}

	if opts.Releases {
		log.Trace("migrating releases")
		messenger("repo.migrate.migrating_releases")
		releases, err := downloader.GetReleases()
		if err != nil {
			if !base.IsErrNotSupported(err) {
				return err
			}
			log.Warn("migrating releases is not supported, ignored")
		}

		relBatchSize := uploader.MaxBatchInsertSize("release")
		for len(releases) > 0 {
			if len(releases) < relBatchSize {
				relBatchSize = len(releases)
			}

			if err = uploader.CreateReleases(releases[:relBatchSize]...); err != nil {
				return err
			}
			releases = releases[relBatchSize:]
		}

		// Once all releases (if any) are inserted, sync any remaining non-release tags
		if err = uploader.SyncTags(); err != nil {
			return err
		}
	}

	var (
		commentBatchSize = uploader.MaxBatchInsertSize("comment")
		reviewBatchSize  = uploader.MaxBatchInsertSize("review")
	)

	supportAllComments := downloader.SupportGetRepoComments()

	if opts.Issues {
		log.Trace("migrating issues and comments")
		messenger("repo.migrate.migrating_issues")
		issueBatchSize := uploader.MaxBatchInsertSize("issue")

		for i := 1; ; i++ {
			issues, isEnd, err := downloader.GetIssues(i, issueBatchSize)
			if err != nil {
				if !base.IsErrNotSupported(err) {
					return err
				}
				log.Warn("migrating issues is not supported, ignored")
				break
			}

			if err := uploader.CreateIssues(issues...); err != nil {
				return err
			}

			if opts.Comments && !supportAllComments {
				allComments := make([]*base.Comment, 0, commentBatchSize)
				for _, issue := range issues {
					log.Trace("migrating issue %d's comments", issue.Number)
					comments, _, err := downloader.GetComments(issue)
					if err != nil {
						if !base.IsErrNotSupported(err) {
							return err
						}
						log.Warn("migrating comments is not supported, ignored")
					}

					allComments = append(allComments, comments...)

					if len(allComments) >= commentBatchSize {
						if err = uploader.CreateComments(allComments[:commentBatchSize]...); err != nil {
							return err
						}

						allComments = allComments[commentBatchSize:]
					}
				}

				if len(allComments) > 0 {
					if err = uploader.CreateComments(allComments...); err != nil {
						return err
					}
				}
			}

			if isEnd {
				break
			}
		}
	}

	if opts.PullRequests {
		log.Trace("migrating pull requests and comments")
		messenger("repo.migrate.migrating_pulls")
		prBatchSize := uploader.MaxBatchInsertSize("pullrequest")
		for i := 1; ; i++ {
			prs, isEnd, err := downloader.GetPullRequests(i, prBatchSize)
			if err != nil {
				if !base.IsErrNotSupported(err) {
					return err
				}
				log.Warn("migrating pull requests is not supported, ignored")
				break
			}

			if err := uploader.CreatePullRequests(prs...); err != nil {
				return err
			}

			if opts.Comments {
				if !supportAllComments {
					// plain comments
					allComments := make([]*base.Comment, 0, commentBatchSize)
					for _, pr := range prs {
						log.Trace("migrating pull request %d's comments", pr.Number)
						comments, _, err := downloader.GetComments(pr)
						if err != nil {
							if !base.IsErrNotSupported(err) {
								return err
							}
							log.Warn("migrating comments is not supported, ignored")
						}

						allComments = append(allComments, comments...)

						if len(allComments) >= commentBatchSize {
							if err = uploader.CreateComments(allComments[:commentBatchSize]...); err != nil {
								return err
							}
							allComments = allComments[commentBatchSize:]
						}
					}
					if len(allComments) > 0 {
						if err = uploader.CreateComments(allComments...); err != nil {
							return err
						}
					}
				}

				// migrate reviews
				allReviews := make([]*base.Review, 0, reviewBatchSize)
				for _, pr := range prs {
					reviews, err := downloader.GetReviews(pr)
					if err != nil {
						if !base.IsErrNotSupported(err) {
							return err
						}
						log.Warn("migrating reviews is not supported, ignored")
						break
					}

					allReviews = append(allReviews, reviews...)

					if len(allReviews) >= reviewBatchSize {
						if err = uploader.CreateReviews(allReviews[:reviewBatchSize]...); err != nil {
							return err
						}
						allReviews = allReviews[reviewBatchSize:]
					}
				}
				if len(allReviews) > 0 {
					if err = uploader.CreateReviews(allReviews...); err != nil {
						return err
					}
				}
			}

			if isEnd {
				break
			}
		}
	}

	if opts.Comments && supportAllComments {
		log.Trace("migrating comments")
		for i := 1; ; i++ {
			comments, isEnd, err := downloader.GetAllComments(i, commentBatchSize)
			if err != nil {
				return err
			}

			if err := uploader.CreateComments(comments...); err != nil {
				return err
			}

			if isEnd {
				break
			}
		}
	}

	return uploader.Finish()
}

// Init migrations service
func Init() error {
	// TODO: maybe we can deprecate these legacy ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS/BLOCKED_DOMAINS, use ALLOWED_HOST_LIST/BLOCKED_HOST_LIST instead

	blockList = hostmatcher.ParseSimpleMatchList("migrations.BLOCKED_DOMAINS", setting.Migrations.BlockedDomains)

	allowList = hostmatcher.ParseSimpleMatchList("migrations.ALLOWED_DOMAINS/ALLOW_LOCALNETWORKS", setting.Migrations.AllowedDomains)
	if allowList.IsEmpty() {
		// the default policy is that migration module can access external hosts
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinExternal)
	}
	if setting.Migrations.AllowLocalNetworks {
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinPrivate)
		allowList.AppendBuiltin(hostmatcher.MatchBuiltinLoopback)
	}
	return nil
}
