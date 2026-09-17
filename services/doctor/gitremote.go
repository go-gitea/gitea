// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"slices"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	repo_service "gitea.dev/services/repository"
)

const pullMirrorRemoteName = "origin" // repo_model.Mirror.GetRemoteName()

// isSelfOverwritingFetchRefSpec reports whether a fetch refspec makes a "git fetch"/"git push" in the
// server-side repository overwrite its own refs instead of remote-tracking refs. Such refspecs are
// created by "git clone --mirror" and "git remote add --mirror=fetch", and they let a plain
// "git push <remote-name>" roll branches back to the value they had when the push started.
func isSelfOverwritingFetchRefSpec(refSpec string) bool {
	_, dst, found := strings.Cut(strings.TrimPrefix(refSpec, "+"), ":")
	if !found || dst == "" {
		return false // fetches into FETCH_HEAD only
	}
	return !strings.HasPrefix(dst, "refs/remotes/")
}

func checkMirrorRemoteRefSpecs(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos, numFound, numFixed := 0, 0, 0

	if err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		numRepos++

		pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(ctx, repo.ID, db.ListOptions{ListAll: true})
		if err != nil {
			return err
		}
		pushMirrorRemotes := make(container.Set[string], len(pushMirrors))
		for _, m := range pushMirrors {
			pushMirrorRemotes.Add(m.RemoteName)
		}

		storageRepos := []git.RepositoryFacade{repo.CodeStorageRepo()}
		if repo_service.HasWiki(ctx, repo) {
			storageRepos = append(storageRepos, repo.WikiStorageRepo())
		}

		for _, storageRepo := range storageRepos {
			configs, err := git.ManagedConfigGetRegexp(ctx, storageRepo, `^remote\..*\.fetch$`)
			if err != nil {
				logger.Warn("Unable to read remote config of %s: %v", storageRepo.LogString(), err)
				continue
			}

			for key, refSpecs := range configs {
				remoteName := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".fetch")
				// a pull mirror legitimately mirrors upstream refs into its own refs
				if repo.IsMirror && remoteName == pullMirrorRemoteName {
					continue
				}
				if !slices.ContainsFunc(refSpecs, isSelfOverwritingFetchRefSpec) {
					continue
				}

				numFound++
				if !pushMirrorRemotes.Contains(remoteName) {
					logger.Warn("%s: remote %q fetches into its own refs (%s). It is not managed by Gitea, please remove it manually.", storageRepo.LogString(), remoteName, strings.Join(refSpecs, ", "))
					continue
				}
				logger.Warn("%s: push mirror remote %q has a fetch refspec (%s)", storageRepo.LogString(), remoteName, strings.Join(refSpecs, ", "))
				if autofix {
					if err := git.ManagedConfigUnsetAll(ctx, storageRepo, key); err != nil {
						logger.Warn("Unable to remove %s of %s: %v", key, storageRepo.LogString(), err)
						continue
					}
					numFixed++
				}
			}
		}
		return nil
	}); err != nil {
		logger.Critical("Unable to check mirror remote refspecs: %v", err)
		return err
	}

	if autofix {
		logger.Info("Checked %d repositories, removed %d of %d dangerous fetch refspecs.", numRepos, numFixed, numFound)
	} else {
		logger.Info("Checked %d repositories, found %d dangerous fetch refspecs.", numRepos, numFound)
	}
	return nil
}

func init() {
	Register(&Check{
		Title:     "Check mirror remotes for fetch refspecs that can overwrite local branches",
		Name:      "check-mirror-remote-refspecs",
		IsDefault: false,
		Run:       checkMirrorRemoteRefSpecs,
		Priority:  9,
	})
}
