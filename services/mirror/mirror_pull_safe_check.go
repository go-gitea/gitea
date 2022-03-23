// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	admin_model "code.gitea.io/gitea/models/admin"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// get git command running stdout and stderr
func getGitCommandStdoutStderr(ctx context.Context, m *repo_model.Mirror, gitArgs []string, newRepoPath string) (string, string, error) {
	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
		return "", "", remoteErr
	}

	if err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.getMirrorCanUpdate: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()
		sanitizer := util.NewURLSanitizer(remoteAddr, true)
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)
		log.Error("CreateRepositoryNotice: %v", err)
		desc := fmt.Sprintf("Failed to get mirror repository can update '%s': %s", newRepoPath, stderrMessage)
		log.Error("GetMirrorCanUpdate [repo: %-v]: failed to get mirror repository can update:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
		if err = admin_model.CreateRepositoryNotice(desc); err != nil {
			log.Error("GetMirrorCanUpdateNotice: %v", err)
		}
	}
	stdoutRepoCommitCount := stdoutBuilder.String()
	stderrRepoCommitCount := stdoutBuilder.String()
	stderrBuilder.Reset()
	stdoutBuilder.Reset()

	return stdoutRepoCommitCount, stderrRepoCommitCount, nil
}

// detect user can update the mirror
func detectCanUpdateMirror(ctx context.Context, m *repo_model.Mirror, gitArgs []string) (error, bool) {
	repoPath := m.Repo.RepoPath()
	newRepoPath := fmt.Sprintf("%s_update", repoPath)
	timeout := time.Duration(setting.Git.Timeout.Mirror) * time.Second

	//do copy directory recursive
	err := util.CopyDir(repoPath, newRepoPath)
	defer util.RemoveAll(newRepoPath)
	if err != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: CopyDirectory Error %v", m.Repo, err)
		return err, false
	}
	remoteAddr, remoteErr := git.GetRemoteAddress(ctx, newRepoPath, m.GetRemoteName())
	if remoteErr != nil {
		log.Error("GetMirrorCanUpdate [repo: %-v]: GetRemoteAddress Error %v", m.Repo, remoteErr)
	}

	stdoutBuilder := strings.Builder{}
	stderrBuilder := strings.Builder{}
	if err := git.NewCommand(ctx, gitArgs...).
		SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
		RunWithContext(&git.RunContext{
			Timeout: timeout,
			Dir:     newRepoPath,
			Stdout:  &stdoutBuilder,
			Stderr:  &stderrBuilder,
		}); err != nil {
		stdout := stdoutBuilder.String()
		stderr := stderrBuilder.String()

		// sanitize the output, since it may contain the remote address, which may
		// contain a password
		sanitizer := util.NewURLSanitizer(remoteAddr, true)
		stderrMessage := sanitizer.Replace(stderr)
		stdoutMessage := sanitizer.Replace(stdout)

		// Now check if the error is a resolve reference due to broken reference
		if strings.Contains(stderr, "unable to resolve reference") && strings.Contains(stderr, "reference broken") {
			log.Warn("SyncMirrors [repo: %-v]: failed to update mirror repository due to broken references:\nStdout: %s\nStderr: %s\nErr: %v\nAttempting Prune", m.Repo, stdoutMessage, stderrMessage, err)
			err = nil

			// Attempt prune
			pruneErr := pruneBrokenReferences(ctx, m, newRepoPath, timeout, &stdoutBuilder, &stderrBuilder, sanitizer, false)
			if pruneErr == nil {
				// Successful prune - reattempt mirror
				stderrBuilder.Reset()
				stdoutBuilder.Reset()
				if err = git.NewCommand(ctx, gitArgs...).
					SetDescription(fmt.Sprintf("Mirror.runSync: %s", m.Repo.FullName())).
					RunWithContext(&git.RunContext{
						Timeout: timeout,
						Dir:     newRepoPath,
						Stdout:  &stdoutBuilder,
						Stderr:  &stderrBuilder,
					}); err != nil {
					stdout := stdoutBuilder.String()
					stderr := stderrBuilder.String()

					// sanitize the output, since it may contain the remote address, which may
					// contain a password
					stderrMessage = sanitizer.Replace(stderr)
					stdoutMessage = sanitizer.Replace(stdout)
				}
			}
		}

		// If there is still an error (or there always was an error)
		if err != nil {
			log.Error("SyncMirrors [repo: %-v]: failed to update mirror repository:\nStdout: %s\nStderr: %s\nErr: %v", m.Repo, stdoutMessage, stderrMessage, err)
			desc := fmt.Sprintf("Failed to update mirror repository '%s': %s", newRepoPath, stderrMessage)
			if err = admin_model.CreateRepositoryNotice(desc); err != nil {
				log.Error("CreateRepositoryNotice: %v", err)
			}
			return nil, false
		}
	}

	gitCommitCountArgs := []string{"rev-list", "HEAD", "--count"}
	stdoutNewRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, newRepoPath)
	if err != nil {
		return err, false
	}
	stdoutNewRepoCommitCount = strings.TrimSpace(stdoutNewRepoCommitCount)
	stdoutRepoCommitCount, _, err := getGitCommandStdoutStderr(ctx, m, gitCommitCountArgs, repoPath)
	if err != nil {
		return err, false
	}
	stdoutRepoCommitCount = strings.TrimSpace(stdoutRepoCommitCount)
	var repoCommitCount, newRepoCommitCount int64
	if i, err := strconv.ParseInt(stdoutRepoCommitCount, 10, 64); err == nil {
		repoCommitCount = i
	} else {
		return err, false
	}
	if i, err := strconv.ParseInt(stdoutNewRepoCommitCount, 10, 64); err == nil {
		newRepoCommitCount = i
	} else {
		return err, false
	}
	if repoCommitCount > newRepoCommitCount {
		return nil, false
	} else if repoCommitCount == newRepoCommitCount {
		// noting to happen
		return nil, true
	} else {
		//compare commit id
		gitNewRepoLastCommitIdArgs := []string{"log", "-1", fmt.Sprintf("--skip=%d", newRepoCommitCount-newRepoCommitCount), "--format=\"%H\""}
		stdoutNewRepoCommitId, _, err := getGitCommandStdoutStderr(ctx, m, gitNewRepoLastCommitIdArgs, newRepoPath)
		if err != nil {
			return err, false
		}
		gitRepoLastCommitIdArgs := []string{"log", "--format=\"%H\"", "-n", "1"}
		stdoutRepoCommitId, _, err := getGitCommandStdoutStderr(ctx, m, gitRepoLastCommitIdArgs, repoPath)
		if err != nil {
			return err, false
		}
		if stdoutNewRepoCommitId != stdoutRepoCommitId {
			return fmt.Errorf("Old repo commit id: %s not match new repo id: %s", stdoutRepoCommitId, stdoutNewRepoCommitId), false
		} else {
			return nil, true
		}
	}
}
