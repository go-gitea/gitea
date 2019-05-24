// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
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
func MigrateRepository(doer *models.User, ownerName string, opts base.MigrateOptions) (*models.Repository, error) {
	var (
		downloader base.Downloader
		uploader   = NewGiteaLocalUploader(doer, ownerName, opts.Name)
	)

	for _, factory := range factories {
		if match, err := factory.Match(opts); err != nil {
			return nil, err
		} else if match {
			downloader, err = factory.New(opts)
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
		downloader = NewPlainGitDownloader(ownerName, opts.Name, opts.RemoteURL)
		log.Trace("Will migrate from git: %s", opts.RemoteURL)
	}

	if err := migrateRepository(downloader, uploader, opts); err != nil {
		if err1 := uploader.Rollback(); err1 != nil {
			log.Error("rollback failed: %v", err1)
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
	if err := uploader.CreateRepo(repo, opts.Wiki); err != nil {
		return err
	}

	if opts.Milestones {
		log.Trace("migrating milestones")
		milestones, err := downloader.GetMilestones()
		if err != nil {
			return err
		}

		for _, milestone := range milestones {
			if err := uploader.CreateMilestone(milestone); err != nil {
				return err
			}
		}
	}

	if opts.Labels {
		log.Trace("migrating labels")
		labels, err := downloader.GetLabels()
		if err != nil {
			return err
		}

		for _, label := range labels {
			if err := uploader.CreateLabel(label); err != nil {
				return err
			}
		}
	}

	if opts.Releases {
		log.Trace("migrating releases")
		releases, err := downloader.GetReleases()
		if err != nil {
			return err
		}

		for _, release := range releases {
			if err := uploader.CreateRelease(release); err != nil {
				return err
			}
		}
	}

	if opts.Issues {
		log.Trace("migrating issues and comments")
		for {
			issues, err := downloader.GetIssues(0, 100)
			if err != nil {
				return err
			}
			for _, issue := range issues {
				if !opts.IgnoreIssueAuthor {
					issue.Content = fmt.Sprintf("Author: @%s \n\n%s", issue.PosterName, issue.Content)
				}

				if err := uploader.CreateIssue(issue); err != nil {
					return err
				}

				if !opts.Comments {
					continue
				}

				comments, err := downloader.GetComments(issue.Number)
				if err != nil {
					return err
				}
				for _, comment := range comments {
					if !opts.IgnoreIssueAuthor {
						comment.Content = fmt.Sprintf("Author: @%s \n\n%s", comment.PosterName, comment.Content)
					}
					if err := uploader.CreateComment(issue.Number, comment); err != nil {
						return err
					}
				}
			}

			if len(issues) < 100 {
				break
			}
		}
	}

	if opts.PullRequests {
		log.Trace("migrating pull requests and comments")
		for {
			prs, err := downloader.GetPullRequests(0, 100)
			if err != nil {
				return err
			}

			for _, pr := range prs {
				if !opts.IgnoreIssueAuthor {
					pr.Content = fmt.Sprintf("Author: @%s \n\n%s", pr.PosterName, pr.Content)
				}
				if err := uploader.CreatePullRequest(pr); err != nil {
					return err
				}
				if !opts.Comments {
					continue
				}

				comments, err := downloader.GetComments(pr.Number)
				if err != nil {
					return err
				}
				for _, comment := range comments {
					if !opts.IgnoreIssueAuthor {
						comment.Content = fmt.Sprintf("Author: @%s \n\n%s", comment.PosterName, comment.Content)
					}
					if err := uploader.CreateComment(pr.Number, comment); err != nil {
						return err
					}
				}
			}
			if len(prs) < 100 {
				break
			}
		}
	}

	return nil
}
