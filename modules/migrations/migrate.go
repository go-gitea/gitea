// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations/base"
)

// MigrateRepository migrate
func MigrateRepository(doer *models.User, ownerName string, opts MigrateOptions) error {
	source, err := opts.Source()
	if err != nil {
		return err
	}
	var (
		downloader base.Downloader
		uploader   = NewGiteaLocalUploader(doer, ownerName, opts.Name)
	)

	switch source {
	case MigrateFromGithub:
		if opts.AuthPassword == "" {
			downloader = NewGithubDownloaderV3(opts.AuthUsername, ownerName, opts.Name)
		}
	}
	if downloader == nil {
		opts.Comments = false
		opts.Issues = false
		opts.Labels = false
		opts.Milestones = false
		opts.PullRequests = false
		downloader = NewPlainGitDownloader(ownerName, opts.Name, opts.RemoteURL)
	}

	return migrateRepository(downloader, uploader, opts)
}

// migrateRepository will download informations and upload to Uploader, this is a simple
// process for small repository. For a big repository, save all the data to disk
// before upload is better
func migrateRepository(downloader base.Downloader, uploader base.Uploader, opts MigrateOptions) error {
	repo, err := downloader.GetRepoInfo()
	if err != nil {
		return err
	}
	repo.IsPrivate = opts.Private
	repo.IsMirror = opts.Mirror
	if err := uploader.CreateRepo(repo); err != nil {
		return err
	}

	if opts.Milestones {
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

	if opts.Issues {
		issues, err := downloader.GetIssues(0, 1000000)
		if err != nil {
			return err
		}
		for _, issue := range issues {
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
				if err := uploader.CreateComment(issue.Number, comment); err != nil {
					return err
				}
			}
		}
	}

	if opts.PullRequests {
		prs, err := downloader.GetPullRequests(0, 1000000)
		if err != nil {
			return err
		}

		for _, pr := range prs {
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
				if err := uploader.CreateComment(pr.Number, comment); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
