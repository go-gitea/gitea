// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/Unknwon/com"
	ini "gopkg.in/ini.v1"
)

// MigrateRepoOptions contains the repository migrate options
type MigrateRepoOptions struct {
	Name        string
	Description string
	IsPrivate   bool
	IsMirror    bool
	RemoteAddr  string
}

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

func sanitizeRepoPath(path string) string {
	return strings.TrimPrefix(path, setting.RepoRootPath)
}

func doMigration(doer, u *User, repo *Repository, opts MigrateRepoOptions, callback func(error) string) {
	repoPath := RepoPath(u.Name, opts.Name)
	wikiPath := WikiPath(u.Name, opts.Name)

	repoPathTmp := repoPath + ".migration"
	wikiPathTmp := wikiPath + ".migration"

	failedMigration := func(err error) {
		if err := os.RemoveAll(wikiPathTmp); err != nil {
			log.Error(3, "Failed to remove %s: %v", wikiPathTmp, err)
		}
		if err := os.RemoveAll(repoPathTmp); err != nil {
			log.Error(3, "Failed to remove %s: %v", repoPathTmp, err)
		}

		repo.IsEmpty = true
		if _, err := x.ID(repo.ID).AllCols().Update(repo); err != nil {
			log.Error(3, "Couldn't set repo to bare:", err)
		}

		NotifyWatchers(&Action{
			ActUserID: doer.ID,
			ActUser:   doer,
			OpType:    ActionMigrationFailure,
			RepoID:    repo.ID,
			Repo:      repo,
			IsPrivate: repo.IsPrivate,
			Content:   callback(err),
		})
	}

	defer func() {
		if err := recover(); err != nil {
			log.Error(3, "PANIC: Migration failed with panic: %v", err)

			// fail the migration
			failedMigration(fmt.Errorf("Migration failed: %v", err))
		}
	}()

	NotifyWatchers(&Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    ActionMigrationStarted,
		RepoID:    repo.ID,
		Repo:      repo,
		Content:   util.SanitizeURLCredentials(opts.RemoteAddr, true),
		IsPrivate: repo.IsPrivate,
	})
	repo.IsEmpty = true
	repo.IsArchived = true
	if _, err := x.ID(repo.ID).AllCols().Update(repo); err != nil {
		failedMigration(err)
		return
	}

	if u.IsOrganization() {
		t, err := u.GetOwnerTeam()
		if err != nil {
			failedMigration(err)
			return
		}
		repo.NumWatches = t.NumMembers
	} else {
		repo.NumWatches = 1
	}

	migrateTimeout := time.Duration(setting.Git.Timeout.Migrate) * time.Second

	if err := git.Clone(opts.RemoteAddr, repoPathTmp, git.CloneRepoOptions{
		Mirror:  true,
		Quiet:   true,
		Timeout: migrateTimeout,
	}); err != nil {
		failedMigration(fmt.Errorf("Clone: %v", err))
		return

	}

	if err := os.RemoveAll(repoPath); err != nil {
		log.Error(2, "Migration Failed: unable remove temporary repo %s: %v", repoPath, err)
		failedMigration(fmt.Errorf("Failed to remove %s", sanitizeRepoPath(repoPath)))
		return
	}

	if err := os.Rename(repoPathTmp, repoPath); err != nil {
		log.Error(2, "Migration Failed: unable rename temporary migrated repo %s to %s: %v", repoPathTmp, repoPath, err)
		failedMigration(fmt.Errorf("Failed to rename %s to %s", sanitizeRepoPath(repoPathTmp), sanitizeRepoPath(repoPath)))
		return
	}

	wikiRemotePath := wikiRemoteURL(opts.RemoteAddr)
	if len(wikiRemotePath) > 0 {
		if err := git.Clone(wikiRemotePath, wikiPathTmp, git.CloneRepoOptions{
			Mirror:  true,
			Quiet:   true,
			Timeout: migrateTimeout,
			Branch:  "master",
		}); err != nil {
			log.Warn("Clone wiki: %v", err)
			if err := os.RemoveAll(wikiPathTmp); err != nil {
				log.Error(2, "Migration Failed: unable remove migrated empty wiki repo %s: %v", wikiPathTmp, err)
				failedMigration(fmt.Errorf("Failed to remove migrated empty wiki repo %s", sanitizeRepoPath(wikiPathTmp)))
				return
			}
		} else {
			if err := os.RemoveAll(wikiPath); err != nil {
				log.Error(2, "Migration Failed: unable remove placeholder repo %s: %v", wikiPath, err)
				failedMigration(fmt.Errorf("Failed to remove placeholder repo %s", sanitizeRepoPath(wikiPath)))
				return
			}

			if err := os.Rename(wikiPathTmp, wikiPath); err != nil {
				log.Error(2, "Migration Failed: unable rename temporary migrated repo %s to %s: %v", wikiPathTmp, wikiPath, err)
				failedMigration(fmt.Errorf("Failed to rename temporary migrated repo %s to %s", sanitizeRepoPath(wikiPathTmp), sanitizeRepoPath(wikiPath)))
				return
			}
		}
	}

	// Check if repository is empty.
	repo.IsEmpty = false
	_, stderr, err := com.ExecCmdDir(repoPath, "git", "log", "-1")
	if err != nil {
		if strings.Contains(stderr, "fatal: bad default revision 'HEAD'") {
			repo.IsEmpty = true
		} else {
			failedMigration(fmt.Errorf("check empty: %v - %s", err, stderr))
			return
		}
	}

	if !repo.IsEmpty {
		// Try to get HEAD branch and set it as default branch.
		gitRepo, err := git.OpenRepository(repoPath)
		if err != nil {
			failedMigration(fmt.Errorf("OpenRepository: %v", err))
			return
		}
		headBranch, err := gitRepo.GetHEADBranch()
		if err != nil {
			failedMigration(fmt.Errorf("GetHEADBranch: %v", err))
			return
		}
		if headBranch != nil {
			repo.DefaultBranch = headBranch.Name
		}

		if err = SyncReleasesWithTags(repo, gitRepo); err != nil {
			log.Error(4, "Failed to synchronize tags to releases for repository: %v", err)
		}
	}

	if err = repo.UpdateSize(); err != nil {
		log.Error(4, "Failed to update size for repository: %v", err)
	}

	if opts.IsMirror {
		if _, err = x.InsertOne(&Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: util.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
		}); err != nil {
			failedMigration(fmt.Errorf("InsertOne: %v", err))
			return
		}

		repo.IsMirror = true
		err = UpdateRepository(repo, false)
	} else {
		repo, err = CleanUpMigrateInfo(repo)
	}

	if err != nil {
		if !repo.IsEmpty {
			UpdateRepoIndexer(repo)
		}
		failedMigration(err)
		return
	}

	repo.IsArchived = false
	if _, err := x.ID(repo.ID).AllCols().Update(repo); err != nil {
		failedMigration(err)
		return
	}

	NotifyWatchers(&Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    ActionMigrationSuccessful,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   callback(nil),
	})
}

// MigrateRepository migrates a existing repository from other project hosting.
func MigrateRepository(doer, u *User, opts MigrateRepoOptions, messageConverter func(error) string) (*Repository, error) {
	repo, err := CreateRepository(doer, u, CreateRepoOptions{
		Name:        opts.Name,
		Description: opts.Description,
		IsPrivate:   opts.IsPrivate,
		IsMirror:    opts.IsMirror,
		NoWatchers:  true,
	})
	if err != nil {
		return nil, err
	}

	env, ok := os.LookupEnv("GIT_TERMINAL_PROMPT=0")
	os.Setenv("GIT_TERMINAL_PROMPT", "0")
	if _, err = git.NewCommand("ls-remote", "-h", opts.RemoteAddr).RunTimeout(1 * time.Minute); err != nil {
		if ok {
			os.Setenv("GIT_TERMINAL_PROMPT", env)
		} else {
			os.Unsetenv("GIT_TERMINAL_PROMPT")
		}
		return repo, fmt.Errorf("Clone: %v", err)
	}
	if ok {
		os.Setenv("GIT_TERMINAL_PROMPT", env)
	} else {
		os.Unsetenv("GIT_TERMINAL_PROMPT")
	}

	// OK if we succeeded above then we know that the clone should start...
	go doMigration(doer, u, repo, opts, messageConverter)

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
func CleanUpMigrateInfo(repo *Repository) (*Repository, error) {
	repoPath := repo.RepoPath()
	if err := createDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %v", err)
	}
	if repo.HasWiki() {
		if err := createDelegateHooks(repo.WikiPath()); err != nil {
			return repo, fmt.Errorf("createDelegateHooks.(wiki): %v", err)
		}
	}

	if err := cleanUpMigrateGitConfig(repo.GitConfigPath()); err != nil {
		return repo, fmt.Errorf("cleanUpMigrateGitConfig: %v", err)
	}
	if repo.HasWiki() {
		if err := cleanUpMigrateGitConfig(path.Join(repo.WikiPath(), "config")); err != nil {
			return repo, fmt.Errorf("cleanUpMigrateGitConfig (wiki): %v", err)
		}
	}

	return repo, UpdateRepository(repo, false)
}
