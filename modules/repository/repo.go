// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	migration "code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"gopkg.in/ini.v1"
)

/*
	GitHub, GitLab, Gogs: *.wiki.git
	BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// WikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func WikiRemoteURL(remote string) string {
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
func MigrateRepositoryGitData(ctx context.Context, u *models.User, repo *models.Repository, opts migration.MigrateOptions) (*models.Repository, error) {
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
	if err = util.RemoveAll(repoPath); err != nil {
		return repo, fmt.Errorf("Failed to remove %s: %v", repoPath, err)
	}

	if err = git.CloneWithContext(ctx, opts.CloneAddr, repoPath, git.CloneRepoOptions{
		Mirror:  true,
		Quiet:   true,
		Timeout: migrateTimeout,
	}); err != nil {
		return repo, fmt.Errorf("Clone: %v", err)
	}

	if opts.Wiki {
		wikiPath := models.WikiPath(u.Name, opts.RepoName)
		wikiRemotePath := WikiRemoteURL(opts.CloneAddr)
		if len(wikiRemotePath) > 0 {
			if err := util.RemoveAll(wikiPath); err != nil {
				return repo, fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
			}

			if err = git.CloneWithContext(ctx, wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:  true,
				Quiet:   true,
				Timeout: migrateTimeout,
				Branch:  "master",
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := util.RemoveAll(wikiPath); err != nil {
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

	if opts.LFS {
		lfsFetchDir := filepath.Join(setting.LFS.Path, "tmp")
		_, err = git.NewCommand("config", "lfs.storage", lfsFetchDir).RunInDir(repoPath)
		if err != nil {
			return repo, fmt.Errorf("Failed `git config lfs.storage lfsFetchDir`: %v", err)
		}

		excludedPointersPaths := []string{}

		// scan repo for pointer files, exclude those exceeding MaxFileSize from being downloaded
		if setting.LFS.MaxFileSize > 0 {
			headBranch, _ := gitRepo.GetHEADBranch()
			headCommit, _ := gitRepo.GetCommit(headBranch.Name)
			entries, err := headCommit.Tree.ListEntriesRecursive()
			if err != nil {
				return repo, fmt.Errorf("Failed to access git files: %v", err)
			}

			for _, entry := range entries {
				entryString, _ := entry.Blob().GetBlobContent()

				if !strings.HasPrefix(entryString, models.LFSMetaFileIdentifier) {
					continue
				}

				splitLines := strings.Split(entryString, "\n")
				if len(splitLines) < 3 {
					continue
				}

				size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
				if err != nil {
					continue
				}

				if size > setting.LFS.MaxFileSize {
					excludedPointersPaths = append(excludedPointersPaths, entry.Name())
					log.Info("Denied LFS[%s] download of size %d to %s/%s because of LFS_MAX_FILE_SIZE=%d", entry.Name(), entry.Blob().Size(), u.Name, repo.Name, setting.LFS.MaxFileSize)
				}
			}
		}

		// fetch LFS files
		cmd := git.NewCommand("lfs", "fetch", opts.CloneAddr)
		if len(excludedPointersPaths) > 0 {
			cmd.AddArguments("--exclude", strings.Join(excludedPointersPaths, ","))
		}
		_, err = cmd.RunInDir(repoPath)
		if err != nil {
			return repo, fmt.Errorf("Failed `lfs fetch` %s: %v", opts.CloneAddr, err)
		}

		lfsSrc := path.Join(lfsFetchDir, "objects")
		lfsDst := path.Join(setting.LFS.Path)

		// rename LFS files
		err = filepath.Walk(lfsSrc, func(path string, info os.FileInfo, err error) error {
			var relSrcPath = strings.Replace(path, lfsSrc, "", 1)
			if relSrcPath == "" {
				return nil
			}
			if err != nil {
				return err
			}
			lfsSrcFull := filepath.Join(lfsSrc, relSrcPath)
			lfsDstFull := filepath.Join(lfsDst, relSrcPath)

			if _, err := os.Stat(lfsDstFull); !os.IsNotExist(err) {
				return nil
			}

			if info.IsDir() {
				return os.Mkdir(lfsDstFull, 0755)
			}

			// generate and associate LFS OIDs
			file, err := os.Open(lfsSrcFull)
			if err != nil {
				return err
			}
			defer file.Close()

			oid, err := models.GenerateLFSOid(file)
			if err != nil {
				return err
			}
			fileInfo, err := file.Stat()
			if err != nil {
				return err
			}

			lfsDstFull = filepath.Join(lfsDst, oid[0:2], oid[2:4], oid[4:])
			err = os.Rename(lfsSrcFull, lfsDstFull)
			if err != nil {
				return err
			}

			_, err = models.NewLFSMetaObject(&models.LFSMetaObject{Oid: oid, Size: fileInfo.Size(), RepositoryID: repo.ID})
			if err != nil {
				log.Error("Unable to write LFS OID[%s] size %d meta object in %v/%v to database. Error: %v", oid, fileInfo.Size(), u.Name, repoPath, err)
				return err
			}

			return nil
		})
		if err != nil {
			return repo, fmt.Errorf("Failed to move LFS files %s: %v", lfsSrc, err)
		}

		err = os.RemoveAll(lfsFetchDir)
		if err != nil {
			return repo, fmt.Errorf("Failed to delete temporary LFS directories %s: %v", repoPath, err)
		}
	}

	repo.IsEmpty, err = gitRepo.IsEmpty()
	if err != nil {
		return repo, fmt.Errorf("git.IsEmpty: %v", err)
	}

	if !repo.IsEmpty {
		if len(repo.DefaultBranch) == 0 {
			// Try to get HEAD branch and set it as default branch.
			headBranch, err := gitRepo.GetHEADBranch()
			if err != nil {
				return repo, fmt.Errorf("GetHEADBranch: %v", err)
			}
			if headBranch != nil {
				repo.DefaultBranch = headBranch.Name
			}
		}

		if !opts.Releases {
			if err = SyncReleasesWithTags(repo, gitRepo); err != nil {
				log.Error("Failed to synchronize tags to releases for repository: %v", err)
			}
		}
	}

	if err = repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if opts.Mirror {
		mirrorModel := models.Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: timeutil.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
		}

		if opts.MirrorInterval != "" {
			parsedInterval, err := time.ParseDuration(opts.MirrorInterval)
			if err != nil {
				log.Error("Failed to set Interval: %v", err)
				return repo, err
			}
			if parsedInterval == 0 {
				mirrorModel.Interval = 0
				mirrorModel.NextUpdateUnix = 0
			} else if parsedInterval < setting.Mirror.MinInterval {
				err := fmt.Errorf("Interval %s is set below Minimum Interval of %s", parsedInterval, setting.Mirror.MinInterval)
				log.Error("Interval: %s is too frequent", opts.MirrorInterval)
				return repo, err
			} else {
				mirrorModel.Interval = parsedInterval
				mirrorModel.NextUpdateUnix = timeutil.TimeStampNow().AddDuration(parsedInterval)
			}
		}

		if err = models.InsertMirror(&mirrorModel); err != nil {
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
	if err := createDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %v", err)
	}
	if repo.HasWiki() {
		if err := createDelegateHooks(repo.WikiPath()); err != nil {
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
	opts := models.FindReleasesOptions{IncludeDrafts: true, IncludeTags: true, ListOptions: models.ListOptions{PageSize: 50}}
	for page := 1; ; page++ {
		opts.Page = page
		rels, err := models.GetReleasesByRepoID(repo.ID, opts)
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
			if err := PushUpdateAddTag(repo, gitRepo, tagName); err != nil {
				return fmt.Errorf("pushUpdateAddTag: %v", err)
			}
		}
	}
	return nil
}

// PushUpdateAddTag must be called for any push actions to add tag
func PushUpdateAddTag(repo *models.Repository, gitRepo *git.Repository, tagName string) error {
	tag, err := gitRepo.GetTag(tagName)
	if err != nil {
		return fmt.Errorf("GetTag: %v", err)
	}
	commit, err := tag.Commit()
	if err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	sig := tag.Tagger
	if sig == nil {
		sig = commit.Author
	}
	if sig == nil {
		sig = commit.Committer
	}

	var author *models.User
	var createdAt = time.Unix(1, 0)

	if sig != nil {
		author, err = models.GetUserByEmail(sig.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return fmt.Errorf("GetUserByEmail: %v", err)
		}
		createdAt = sig.When
	}

	commitsCount, err := commit.CommitsCount()
	if err != nil {
		return fmt.Errorf("CommitsCount: %v", err)
	}

	var rel = models.Release{
		RepoID:       repo.ID,
		TagName:      tagName,
		LowerTagName: strings.ToLower(tagName),
		Sha1:         commit.ID.String(),
		NumCommits:   commitsCount,
		CreatedUnix:  timeutil.TimeStamp(createdAt.Unix()),
		IsTag:        true,
	}
	if author != nil {
		rel.PublisherID = author.ID
	}

	return models.SaveOrUpdateTag(repo, &rel)
}
