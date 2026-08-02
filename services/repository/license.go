// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/options"
	"gitea.dev/modules/queue"

	licenseclassifier "github.com/google/licenseclassifier/v2"
)

const (
	LicenseLegacyFile = "LICENSE"
	// REUSE license spec - see https://reuse.software/spec-3.3/
	// TODO: Surface this version in repo creation
	LicenseReuseDir = "LICENSES"
)

var (
	classifier *licenseclassifier.Classifier

	// licenseUpdaterQueue represents a queue to handle update repo licenses
	licenseUpdaterQueue *queue.WorkerPoolQueue[*LicenseUpdaterOptions]

	licensePrefixes = []string{"license", "licence", "copying"}
)

func AddRepoToLicenseUpdaterQueue(opts *LicenseUpdaterOptions) error {
	if opts == nil {
		return nil
	}
	return licenseUpdaterQueue.Push(opts)
}

func InitLicenseClassifier() error {
	// threshold should be 0.84~0.86 or the test will be failed
	classifier = licenseclassifier.NewClassifier(.85)
	licenseFiles, err := options.AssetFS().ListFiles("license", true)
	if err != nil {
		return err
	}

	for _, licenseFile := range licenseFiles {
		licenseName := licenseFile
		data, err := options.License(licenseFile)
		if err != nil {
			return err
		}
		classifier.AddContent("License", licenseName, licenseName, data)
	}
	return nil
}

type LicenseUpdaterOptions struct {
	RepoID int64
}

func repoLicenseUpdater(items ...*LicenseUpdaterOptions) []*LicenseUpdaterOptions {
	ctx := graceful.GetManager().ShutdownContext()

	for _, opts := range items {
		repo, err := repo_model.GetRepositoryByID(ctx, opts.RepoID)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: GetRepositoryByID: %v", opts.RepoID, err)
			continue
		}
		if repo.IsEmpty {
			continue
		}

		gitRepo, err := git.OpenRepository(ctx, repo)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: OpenRepository: %v", opts.RepoID, err)
			continue
		}
		defer gitRepo.Close()

		commit, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
		if err != nil {
			log.Error("repoLicenseUpdater [%d] failed: GetBranchCommit: %v", opts.RepoID, err)
			continue
		}
		if err = UpdateRepoLicenses(ctx, repo, gitRepo, commit); err != nil {
			log.Error("repoLicenseUpdater [%d] failed: updateRepoLicenses: %v", opts.RepoID, err)
		}
	}
	return nil
}

func SyncRepoLicenses(ctx context.Context) error {
	log.Trace("Doing: SyncRepoLicenses")

	if err := db.Iterate(
		ctx,
		nil,
		func(ctx context.Context, repo *repo_model.Repository) error {
			select {
			case <-ctx.Done():
				return db.ErrCancelledf("before sync repo licenses for %s", repo.FullName())
			default:
			}
			return AddRepoToLicenseUpdaterQueue(&LicenseUpdaterOptions{RepoID: repo.ID})
		},
	); err != nil {
		log.Trace("Error: SyncRepoLicenses: %v", err)
		return err
	}

	log.Trace("Finished: SyncReposLicenses")
	return nil
}

// resolveReuseLicenses gathers all licenses in a subtree (assumed to be LicenseReuseDir as per REUSE specification)
func resolveReuseLicenses(ctx context.Context, gitrepo *git.Repository, tree *git.Tree) ([]repo_model.DetectedLicense, error) {
	entries, err := tree.ListEntries(ctx, gitrepo)
	if err != nil {
		return nil, fmt.Errorf("ListEntries: %w", err)
	}
	licenses := make([]repo_model.DetectedLicense, 0)
	for _, entry := range entries {
		if entry.IsRegular() {
			spdxID := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
			licenses = append(licenses, repo_model.DetectedLicense{SPDXID: spdxID, LicensePath: path.Join(LicenseReuseDir, entry.Name())})
		}
	}

	return licenses, nil
}

// isLicenseFile checks the prefix of the file and determines if it could plausibly be a license one
// it's checking well-known ones: license, licence and copying
func isLicenseFile(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range licensePrefixes {
		if strings.HasPrefix(lower, prefix) {
			rest := lower[len(prefix):]
			// Exact match (e.g. "LICENSE") or extension present (e.g. "LICENSE.txt").
			// Reject bare dot (e.g. "LICENSE.") — not a real file extension.
			return rest == "" || (rest[0] == '.' && len(rest) > 1)
		}
	}
	return false
}

func resolveLicenses(ctx context.Context, gitRepo *git.Repository, commit *git.Commit) ([]repo_model.DetectedLicense, error) {
	tree, err := commit.SubTree(ctx, gitRepo, LicenseReuseDir)
	if err != nil && !git.IsErrNotExist(err) {
		return nil, fmt.Errorf("SubTree: %w", err)
	}

	// handle REUSE license spec first
	if !git.IsErrNotExist(err) {
		return resolveReuseLicenses(ctx, gitRepo, tree)
	}

	tree, err = commit.SubTree(ctx, gitRepo, "")
	if err != nil && !git.IsErrNotExist(err) {
		return nil, fmt.Errorf("SubTree: %w", err)
	}

	entries, err := tree.ListEntries(ctx, gitRepo)
	if err != nil {
		return nil, fmt.Errorf("ListEntries: %w", err)
	}
	licenses := make([]repo_model.DetectedLicense, 0)
	for _, entry := range entries {
		if !entry.IsRegular() {
			continue
		}
		if isLicenseFile(entry.Name()) {
			r, err := entry.Blob(gitRepo).DataAsync(ctx)
			if err != nil {
				continue
			}
			found, err := detectLicense(r)
			r.Close()
			if err != nil {
				continue
			}
			for _, license := range found {
				licenses = append(licenses, repo_model.DetectedLicense{SPDXID: license, LicensePath: entry.Name()})
			}
		}
	}
	return licenses, nil
}

// UpdateRepoLicenses will update repository licenses col if license file exists
func UpdateRepoLicenses(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, commit *git.Commit) error {
	if commit == nil {
		return nil
	}
	licenses, err := resolveLicenses(ctx, gitRepo, commit)
	if err != nil {
		return err
	}
	if len(licenses) == 0 {
		return repo_model.CleanRepoLicenses(ctx, repo)
	}

	return repo_model.UpdateRepoLicenses(ctx, repo, commit.ID.String(), licenses)
}

// detectLicense returns the licenses detected by the given content buff
func detectLicense(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, nil
	}

	matches, err := classifier.MatchFrom(r)
	if err != nil {
		return nil, err
	}
	if len(matches.Matches) > 0 {
		results := make(container.Set[string], len(matches.Matches))
		for _, r := range matches.Matches {
			if r.MatchType == "License" && !results.Contains(r.Variant) {
				results.Add(r.Variant)
			}
		}
		return results.Values(), nil
	}
	return nil, nil
}
