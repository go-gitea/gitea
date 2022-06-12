// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"os"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"

	"lab.forgefriends.org/friendlyforgeformat/gofff"
	gofff_domain "lab.forgefriends.org/friendlyforgeformat/gofff/domain"
	gofff_forges "lab.forgefriends.org/friendlyforgeformat/gofff/forges"
	gofff_file "lab.forgefriends.org/friendlyforgeformat/gofff/forges/file"
)

// DumpRepository dump repository according MigrateOptions to a local directory
func DumpRepository(ctx context.Context, baseDir, ownerName string, opts base.MigrateOptions) error {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "migrate")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	downloader, err := newDownloader(ctx, ownerName, tmpDir, opts, nil)
	if err != nil {
		return err
	}

	uploader, err := gofff_forges.NewForge(&gofff_file.Options{
		Options: gofff.Options{
			Configuration: gofff.Configuration{
				Directory: baseDir,
			},
			Logger:   ToGofffLogger(nil),
			Features: opts.ToGofffFeatures(),
		},
	})
	if err != nil {
		return err
	}
	uploader.SetContext(ctx)

	return gofff_domain.Migrate(ctx, downloader, uploader, ToGofffLogger(nil), opts.ToGofffFeatures())
}

// RestoreRepository restore a repository from the disk directory
func RestoreRepository(ctx context.Context, baseDir, ownerName, repoName string, units []string, validation bool) error {
	//
	// Uploader
	//
	doer, err := user_model.GetAdminUser()
	if err != nil {
		return err
	}
	serviceType := structs.GiteaService
	opts := base.MigrateOptions{
		RepoName:       repoName,
		GitServiceType: serviceType,
	}
	updateOptionsUnits(&opts, units)
	uploader := NewGiteaLocalUploader(ctx, doer, ownerName, opts)

	//
	// Downloader
	//
	downloader, err := gofff_forges.NewForge(&gofff_file.Options{
		Options: gofff.Options{
			Configuration: gofff.Configuration{
				Directory: baseDir,
			},
			Logger:   ToGofffLogger(nil),
			Features: opts.ToGofffFeatures(),
		},
		Validation: validation,
	})
	if err != nil {
		return err
	}
	uploader.SetContext(ctx)

	//
	// Restore what is read from file to the local Gitea instance
	//
	if err := gofff_domain.Migrate(ctx, downloader, uploader, ToGofffLogger(nil), opts.ToGofffFeatures()); err != nil {
		return err
	}
	return updateMigrationPosterIDByGitService(ctx, serviceType)
}

func updateOptionsUnits(opts *base.MigrateOptions, units []string) {
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
			}
		}
	}
}
