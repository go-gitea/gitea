// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

// MigrateOptions is equal to base.MigrateOptions
type MigrateOptions = base.MigrateOptions

var (
	factories []base.DownloaderFactory
)

// RegisterDownloaderFactory registers a downloader factory
func RegisterDownloaderFactory(factory base.DownloaderFactory) {
	factories = append(factories, factory)
}

// MigrateRepository migrate repository according MigrateOptions
func MigrateRepository(ctx context.Context, doer *models.User, ownerName string, opts base.MigrateOptions) (*models.Repository, error) {
	var (
		downloader base.Downloader
		uploader   = NewGiteaLocalUploader(ctx, doer, ownerName, opts.RepoName)
		theFactory base.DownloaderFactory
	)

	for _, factory := range factories {
		if match, err := factory.Match(opts); err != nil {
			return nil, err
		} else if match {
			downloader, err = factory.New(opts)
			if err != nil {
				return nil, err
			}
			theFactory = factory
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
		opts.GitServiceType = structs.PlainGitService
		downloader = NewPlainGitDownloader(ownerName, opts.RepoName, opts.CloneAddr)
		log.Trace("Will migrate from git: %s", opts.OriginalURL)
	} else if opts.GitServiceType == structs.NotMigrated {
		opts.GitServiceType = theFactory.GitServiceType()
	}

	uploader.gitServiceType = opts.GitServiceType

	if setting.Migrations.MaxAttempts > 1 {
		downloader = base.NewRetryDownloader(downloader, setting.Migrations.MaxAttempts, setting.Migrations.RetryBackoff)
	}

	downloader.SetContext(ctx)

	if err := migrateRepository(downloader, uploader, opts); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
		}

		if err2 := models.CreateRepositoryNotice(fmt.Sprintf("Migrate repository from %s failed: %v", opts.OriginalURL, err)); err2 != nil {
			log.Error("create respotiry notice failed: ", err2)
		}
		return nil, err
	}

	return uploader.repo, nil
}

// migrateRepository will download informations and upload to Uploader, this is a simple
// process for small repository. For a big repository, save all the data to disk
// before upload is better
func migrateRepository(downloader base.Downloader, uploader base.Uploader, opts base.MigrateOptions) error {
	repo, err := downloader.GetRepoInfo()
	if err != nil {
		return err
	}
	repo.IsPrivate = opts.Private
	repo.IsMirror = opts.Mirror
	if opts.Description != "" {
		repo.Description = opts.Description
	}
	log.Trace("migrating git data")
	if err := uploader.CreateRepo(repo, opts); err != nil {
		return err
	}
	defer uploader.Close()

	log.Trace("migrating topics")
	topics, err := downloader.GetTopics()
	if err != nil {
		return err
	}
	if len(topics) > 0 {
		if err := uploader.CreateTopics(topics...); err != nil {
			return err
		}
	}

	if opts.Milestones {
		log.Trace("migrating milestones")
		milestones, err := downloader.GetMilestones()
		if err != nil {
			return err
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
		labels, err := downloader.GetLabels()
		if err != nil {
			return err
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
		releases, err := downloader.GetReleases()
		if err != nil {
			return err
		}

		relBatchSize := uploader.MaxBatchInsertSize("release")
		for len(releases) > 0 {
			if len(releases) < relBatchSize {
				relBatchSize = len(releases)
			}

			if err := uploader.CreateReleases(releases[:relBatchSize]...); err != nil {
				return err
			}
			releases = releases[relBatchSize:]
		}

		// Once all releases (if any) are inserted, sync any remaining non-release tags
		if err := uploader.SyncTags(); err != nil {
			return err
		}
	}

	var commentBatchSize = uploader.MaxBatchInsertSize("comment")

	if opts.Issues {
		log.Trace("migrating issues and comments")
		var issueBatchSize = uploader.MaxBatchInsertSize("issue")

		for i := 1; ; i++ {
			issues, isEnd, err := downloader.GetIssues(i, issueBatchSize)
			if err != nil {
				return err
			}

			if err := uploader.CreateIssues(issues...); err != nil {
				return err
			}

			if !opts.Comments {
				continue
			}

			var allComments = make([]*base.Comment, 0, commentBatchSize)
			for _, issue := range issues {
				comments, err := downloader.GetComments(issue.Number)
				if err != nil {
					return err
				}

				allComments = append(allComments, comments...)

				if len(allComments) >= commentBatchSize {
					if err := uploader.CreateComments(allComments[:commentBatchSize]...); err != nil {
						return err
					}

					allComments = allComments[commentBatchSize:]
				}
			}

			if len(allComments) > 0 {
				if err := uploader.CreateComments(allComments...); err != nil {
					return err
				}
			}

			if isEnd {
				break
			}
		}
	}

	if opts.PullRequests {
		log.Trace("migrating pull requests and comments")
		var prBatchSize = uploader.MaxBatchInsertSize("pullrequest")
		for i := 1; ; i++ {
			prs, err := downloader.GetPullRequests(i, prBatchSize)
			if err != nil {
				return err
			}

			if err := uploader.CreatePullRequests(prs...); err != nil {
				return err
			}

			if !opts.Comments {
				continue
			}

			var allComments = make([]*base.Comment, 0, commentBatchSize)
			for _, pr := range prs {
				comments, err := downloader.GetComments(pr.Number)
				if err != nil {
					return err
				}

				allComments = append(allComments, comments...)

				if len(allComments) >= commentBatchSize {
					if err := uploader.CreateComments(allComments[:commentBatchSize]...); err != nil {
						return err
					}
					allComments = allComments[commentBatchSize:]
				}
			}
			if len(allComments) > 0 {
				if err := uploader.CreateComments(allComments...); err != nil {
					return err
				}
			}

			if len(prs) < prBatchSize {
				break
			}
		}
	}

	return nil
}
