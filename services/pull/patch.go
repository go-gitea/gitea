// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// DownloadDiff will write the patch for the pr to the writer
func DownloadDiff(pr *models.PullRequest, w io.Writer, patch bool) error {
	return DownloadDiffOrPatch(pr, w, false)
}

// DownloadPatch will write the patch for the pr to the writer
func DownloadPatch(pr *models.PullRequest, w io.Writer, patch bool) error {
	return DownloadDiffOrPatch(pr, w, true)
}

// DownloadDiffOrPatch will write the patch for the pr to the writer
func DownloadDiffOrPatch(pr *models.PullRequest, w io.Writer, patch bool) error {
	// Clone base repo.
	tmpBasePath, err := createTemporaryRepo(pr)
	if err != nil {
		log.Error("CreateTemporaryPath: %v", err)
		return err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("DownloadDiff: RemoveTemporaryPath: %s", err)
		}
	}()

	gitRepo, err := git.OpenRepository(tmpBasePath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	pr.MergeBase, err = git.NewCommand("merge-base", "--", "base", "tracking").RunInDir(tmpBasePath)
	if err != nil {
		pr.MergeBase = "base"
	}
	pr.MergeBase = strings.TrimSpace(pr.MergeBase)
	if err := gitRepo.GetDiffOrPatch(pr.MergeBase, "tracking", w, patch); err != nil {
		log.Error("Unable to get patch file from %s to %s in %s/%s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.MustOwner().Name, pr.BaseRepo.Name, err)
		return fmt.Errorf("Unable to get patch file from %s to %s in %s/%s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.MustOwner().Name, pr.BaseRepo.Name, err)
	}
	return nil
}

var patchErrorSuffices = []string{
	": already exists in index",
	": patch does not apply",
	": already exists in working directory",
	"unrecognized input",
}

// TestPatch will test whether a simple patch will apply
func TestPatch(pr *models.PullRequest) error {
	// Clone base repo.
	tmpBasePath, err := createTemporaryRepo(pr)
	if err != nil {
		log.Error("CreateTemporaryPath: %v", err)
		return err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	gitRepo, err := git.OpenRepository(tmpBasePath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	pr.MergeBase, err = git.NewCommand("merge-base", "--", "base", "tracking").RunInDir(tmpBasePath)
	if err != nil {
		var err2 error
		pr.MergeBase, err2 = gitRepo.GetRefCommitID(git.BranchPrefix + "base")
		if err2 != nil {
			return fmt.Errorf("GetMergeBase: %v and can't find commit ID for base: %v", err, err2)
		}
	}
	pr.MergeBase = strings.TrimSpace(pr.MergeBase)
	tmpPatchFile, err := ioutil.TempFile("", "patch")
	if err != nil {
		log.Error("Unable to create temporary patch file! Error: %v", err)
		return fmt.Errorf("Unable to create temporary patch file! Error: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpPatchFile.Name())
	}()

	if err := gitRepo.GetDiff(pr.MergeBase, "tracking", tmpPatchFile); err != nil {
		tmpPatchFile.Close()
		log.Error("Unable to get patch file from %s to %s in %s/%s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.MustOwner().Name, pr.BaseRepo.Name, err)
		return fmt.Errorf("Unable to get patch file from %s to %s in %s/%s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.MustOwner().Name, pr.BaseRepo.Name, err)
	}
	stat, err := tmpPatchFile.Stat()
	if err != nil {
		tmpPatchFile.Close()
		return fmt.Errorf("Unable to stat patch file: %v", err)
	}
	patchPath := tmpPatchFile.Name()
	tmpPatchFile.Close()

	if stat.Size() == 0 {
		log.Debug("PullRequest[%d]: Patch is empty - ignoring", pr.ID)
		pr.Status = models.PullRequestStatusMergeable
		pr.ConflictedFiles = []string{}
		return nil
	}

	log.Trace("PullRequest[%d].testPatch (patchPath): %s", pr.ID, patchPath)

	pr.Status = models.PullRequestStatusChecking

	_, err = git.NewCommand("read-tree", "base").RunInDir(tmpBasePath)
	if err != nil {
		return fmt.Errorf("git read-tree %s: %v", pr.BaseBranch, err)
	}

	prUnit, err := pr.BaseRepo.GetUnit(models.UnitTypePullRequests)
	if err != nil {
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	args := []string{"apply", "--check", "--cached"}
	if prConfig.IgnoreWhitespaceConflicts {
		args = append(args, "--ignore-whitespace")
	}
	args = append(args, patchPath)
	pr.ConflictedFiles = make([]string, 0, 5)

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to open stderr pipe: %v", err)
		return fmt.Errorf("Unable to open stderr pipe: %v", err)
	}
	defer func() {
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()
	conflict := false
	err = git.NewCommand(args...).
		RunInDirTimeoutEnvFullPipelineFunc(
			nil, -1, tmpBasePath,
			nil, stderrWriter, nil,
			func(ctx context.Context, cancel context.CancelFunc) {
				_ = stderrWriter.Close()
				const prefix = "error: patch failed:"
				const errorPrefix = "error: "
				conflictMap := map[string]bool{}

				scanner := bufio.NewScanner(stderrReader)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, prefix) {
						conflict = true
						filepath := strings.TrimSpace(strings.Split(line[len(prefix):], ":")[0])
						conflictMap[filepath] = true
					} else if strings.HasPrefix(line, errorPrefix) {
						conflict = true
						for _, suffix := range patchErrorSuffices {
							if strings.HasSuffix(line, suffix) {
								filepath := strings.TrimSpace(strings.TrimSuffix(line[len(errorPrefix):], suffix))
								if filepath != "" {
									conflictMap[filepath] = true
								}
								break
							}
						}
					}
					// only list 10 conflicted files
					if len(conflictMap) >= 10 {
						break
					}
				}
				if len(conflictMap) > 0 {
					pr.ConflictedFiles = make([]string, 0, len(conflictMap))
					for key := range conflictMap {
						pr.ConflictedFiles = append(pr.ConflictedFiles, key)
					}
				}
				_ = stderrReader.Close()
			})

	if err != nil {
		if conflict {
			pr.Status = models.PullRequestStatusConflict
			log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
			return nil
		}
		return fmt.Errorf("git apply --check: %v", err)
	}
	pr.Status = models.PullRequestStatusMergeable

	return nil
}
