// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"gopkg.in/ini.v1"
)

/*
	GitHub, GitLab, Gogs: *.wiki.git
	BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// wikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func wikiRemoteURL(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	for _, suffix := range commonWikiURLSuffixes {
		wikiURL := remote + suffix
		if git.IsRepoURLAccessible(wikiURL) {
			return wikiURL
		}
	}
	return ""
}

// MigrateRepositoryGitData starts migrating git related data after created migrating repository
func MigrateRepositoryGitData(doer, u *models.User, repo *models.Repository, opts api.MigrateRepoOption) (*models.Repository, error) {
	repoPath := models.RepoPath(u.Name, opts.RepoName)

	if u.IsOrganization() {
		t, err := u.GetOwnerTeam()
		if err != nil {
			return nil, err
		}
		repo.NumWatches = t.NumMembers
	} else {
		repo.NumWatches = 1
	}

	migrateTimeout := time.Duration(setting.Git.Timeout.Migrate) * time.Second

	var err error
	if err = os.RemoveAll(repoPath); err != nil {
		return repo, fmt.Errorf("Failed to remove %s: %v", repoPath, err)
	}

	if err = git.Clone(opts.CloneAddr, repoPath, git.CloneRepoOptions{
		Mirror:  true,
		Quiet:   true,
		Timeout: migrateTimeout,
	}); err != nil {
		return repo, fmt.Errorf("Clone: %v", err)
	}

	if opts.Wiki {
		wikiPath := models.WikiPath(u.Name, opts.RepoName)
		wikiRemotePath := wikiRemoteURL(opts.CloneAddr)
		if len(wikiRemotePath) > 0 {
			if err := os.RemoveAll(wikiPath); err != nil {
				return repo, fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
			}

			if err = git.Clone(wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:  true,
				Quiet:   true,
				Timeout: migrateTimeout,
				Branch:  "master",
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := os.RemoveAll(wikiPath); err != nil {
					return repo, fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
				}
			}
		}
	}

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return repo, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	repo.IsEmpty, err = gitRepo.IsEmpty()
	if err != nil {
		return repo, fmt.Errorf("git.IsEmpty: %v", err)
	}

	if !opts.Releases && !repo.IsEmpty {
		// Try to get HEAD branch and set it as default branch.
		headBranch, err := gitRepo.GetHEADBranch()
		if err != nil {
			return repo, fmt.Errorf("GetHEADBranch: %v", err)
		}
		if headBranch != nil {
			repo.DefaultBranch = headBranch.Name
		}

		if err = SyncReleasesWithTags(repo, gitRepo); err != nil {
			log.Error("Failed to synchronize tags to releases for repository: %v", err)
		}
	}

	if err = repo.UpdateSize(); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if opts.Mirror {
		if err = models.InsertMirror(&models.Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: timeutil.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
		}); err != nil {
			return repo, fmt.Errorf("InsertOne: %v", err)
		}

		repo.IsMirror = true
		err = models.UpdateRepository(repo, false)
	} else {
		repo, err = CleanUpMigrateInfo(repo)
	}

	return repo, err
}

// cleanUpMigrateGitConfig removes mirror info which prevents "push --all".
// This also removes possible user credentials.
func cleanUpMigrateGitConfig(configPath string) error {
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("open config file: %v", err)
	}
	cfg.DeleteSection("remote \"origin\"")
	if err = cfg.SaveToIndent(configPath, "\t"); err != nil {
		return fmt.Errorf("save config file: %v", err)
	}
	return nil
}

// CleanUpMigrateInfo finishes migrating repository and/or wiki with things that don't need to be done for mirrors.
func CleanUpMigrateInfo(repo *models.Repository) (*models.Repository, error) {
	repoPath := repo.RepoPath()
	if err := models.CreateDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %v", err)
	}
	if repo.HasWiki() {
		if err := models.CreateDelegateHooks(repo.WikiPath()); err != nil {
			return repo, fmt.Errorf("createDelegateHooks.(wiki): %v", err)
		}
	}

	_, err := git.NewCommand("remote", "rm", "origin").RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return repo, fmt.Errorf("CleanUpMigrateInfo: %v", err)
	}

	if repo.HasWiki() {
		if err := cleanUpMigrateGitConfig(path.Join(repo.WikiPath(), "config")); err != nil {
			return repo, fmt.Errorf("cleanUpMigrateGitConfig (wiki): %v", err)
		}
	}

	return repo, models.UpdateRepository(repo, false)
}

// SyncReleasesWithTags synchronizes release table with repository tags
func SyncReleasesWithTags(repo *models.Repository, gitRepo *git.Repository) error {
	existingRelTags := make(map[string]struct{})
	opts := models.FindReleasesOptions{IncludeDrafts: true, IncludeTags: true}
	for page := 1; ; page++ {
		rels, err := models.GetReleasesByRepoID(repo.ID, opts, page, 100)
		if err != nil {
			return fmt.Errorf("GetReleasesByRepoID: %v", err)
		}
		if len(rels) == 0 {
			break
		}
		for _, rel := range rels {
			if rel.IsDraft {
				continue
			}
			commitID, err := gitRepo.GetTagCommitID(rel.TagName)
			if err != nil && !git.IsErrNotExist(err) {
				return fmt.Errorf("GetTagCommitID: %s: %v", rel.TagName, err)
			}
			if git.IsErrNotExist(err) || commitID != rel.Sha1 {
				if err := models.PushUpdateDeleteTag(repo, rel.TagName); err != nil {
					return fmt.Errorf("PushUpdateDeleteTag: %s: %v", rel.TagName, err)
				}
			} else {
				existingRelTags[strings.ToLower(rel.TagName)] = struct{}{}
			}
		}
	}
	tags, err := gitRepo.GetTags()
	if err != nil {
		return fmt.Errorf("GetTags: %v", err)
	}
	for _, tagName := range tags {
		if _, ok := existingRelTags[strings.ToLower(tagName)]; !ok {
			if err := models.PushUpdateAddTag(repo, gitRepo, tagName); err != nil {
				return fmt.Errorf("pushUpdateAddTag: %s: %v", tagName, err)
			}
		}
	}
	return nil
}
