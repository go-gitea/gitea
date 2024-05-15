// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
)

// DownloadDiffOrPatch will write the patch for the pr to the writer
func DownloadDiffOrPatch(ctx context.Context, pr *issues_model.PullRequest, w io.Writer, patch, binary bool) error {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to load base repository ID %d for pr #%d [%d]", pr.BaseRepoID, pr.Index, pr.ID)
		return err
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.BaseRepo)
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	defer closer.Close()

	if err := gitRepo.GetDiffOrPatch(pr.MergeBase, pr.GetGitRefName(), w, patch, binary); err != nil {
		log.Error("Unable to get patch file from %s to %s in %s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
		return fmt.Errorf("Unable to get patch file from %s to %s in %s Error: %w", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
	}
	return nil
}

var patchErrorSuffices = []string{
	": already exists in index",
	": patch does not apply",
	": already exists in working directory",
	"unrecognized input",
	": No such file or directory",
	": does not exist in index",
}

// TestPatch will test whether a simple patch will apply
func TestPatch(pr *issues_model.PullRequest) error {
	ctx, _, finished := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("TestPatch: %s", pr))
	defer finished()

	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		if !git_model.IsErrBranchNotExist(err) {
			log.Error("CreateTemporaryRepoForPR %-v: %v", pr, err)
		}
		return err
	}
	defer cancel()

	return testPatch(ctx, prCtx, pr)
}

func testPatch(ctx context.Context, prCtx *prContext, pr *issues_model.PullRequest) error {
	gitRepo, err := git.OpenRepository(ctx, prCtx.tmpBasePath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	// 1. update merge base
	pr.MergeBase, _, err = git.NewCommand(ctx, "merge-base", "--", "base", "tracking").RunStdString(&git.RunOpts{Dir: prCtx.tmpBasePath})
	if err != nil {
		var err2 error
		pr.MergeBase, err2 = gitRepo.GetRefCommitID(git.BranchPrefix + "base")
		if err2 != nil {
			return fmt.Errorf("GetMergeBase: %v and can't find commit ID for base: %w", err, err2)
		}
	}
	pr.MergeBase = strings.TrimSpace(pr.MergeBase)
	if pr.HeadCommitID, err = gitRepo.GetRefCommitID(git.BranchPrefix + "tracking"); err != nil {
		return fmt.Errorf("GetBranchCommitID: can't find commit ID for head: %w", err)
	}

	if pr.HeadCommitID == pr.MergeBase {
		pr.Status = issues_model.PullRequestStatusAncestor
		return nil
	}

	// 2. Check for conflicts
	if conflicts, err := checkConflicts(ctx, pr, gitRepo, prCtx.tmpBasePath); err != nil || conflicts || pr.Status == issues_model.PullRequestStatusEmpty {
		return err
	}

	// 3. Check for protected files changes
	if err = checkPullFilesProtection(ctx, pr, gitRepo); err != nil {
		return fmt.Errorf("pr.CheckPullFilesProtection(): %v", err)
	}

	if len(pr.ChangedProtectedFiles) > 0 {
		log.Trace("Found %d protected files changed", len(pr.ChangedProtectedFiles))
	}

	pr.Status = issues_model.PullRequestStatusMergeable

	return nil
}

type errMergeConflict struct {
	filename string
}

func (e *errMergeConflict) Error() string {
	return fmt.Sprintf("conflict detected at: %s", e.filename)
}

func attemptMerge(ctx context.Context, file *unmergedFile, tmpBasePath string, gitRepo *git.Repository) error {
	log.Trace("Attempt to merge:\n%v", file)

	switch {
	case file.stage1 != nil && (file.stage2 == nil || file.stage3 == nil):
		// 1. Deleted in one or both:
		//
		// Conflict <==> the stage1 !SameAs to the undeleted one
		if (file.stage2 != nil && !file.stage1.SameAs(file.stage2)) || (file.stage3 != nil && !file.stage1.SameAs(file.stage3)) {
			// Conflict!
			return &errMergeConflict{file.stage1.path}
		}

		// Not a genuine conflict and we can simply remove the file from the index
		return gitRepo.RemoveFilesFromIndex(file.stage1.path)
	case file.stage1 == nil && file.stage2 != nil && (file.stage3 == nil || file.stage2.SameAs(file.stage3)):
		// 2. Added in ours but not in theirs or identical in both
		//
		// Not a genuine conflict just add to the index
		if err := gitRepo.AddObjectToIndex(file.stage2.mode, git.MustIDFromString(file.stage2.sha), file.stage2.path); err != nil {
			return err
		}
		return nil
	case file.stage1 == nil && file.stage2 != nil && file.stage3 != nil && file.stage2.sha == file.stage3.sha && file.stage2.mode != file.stage3.mode:
		// 3. Added in both with the same sha but the modes are different
		//
		// Conflict! (Not sure that this can actually happen but we should handle)
		return &errMergeConflict{file.stage2.path}
	case file.stage1 == nil && file.stage2 == nil && file.stage3 != nil:
		// 4. Added in theirs but not ours:
		//
		// Not a genuine conflict just add to the index
		return gitRepo.AddObjectToIndex(file.stage3.mode, git.MustIDFromString(file.stage3.sha), file.stage3.path)
	case file.stage1 == nil:
		// 5. Created by new in both
		//
		// Conflict!
		return &errMergeConflict{file.stage2.path}
	case file.stage2 != nil && file.stage3 != nil:
		// 5. Modified in both - we should try to merge in the changes but first:
		//
		if file.stage2.mode == "120000" || file.stage3.mode == "120000" {
			// 5a. Conflicting symbolic link change
			return &errMergeConflict{file.stage2.path}
		}
		if file.stage2.mode == "160000" || file.stage3.mode == "160000" {
			// 5b. Conflicting submodule change
			return &errMergeConflict{file.stage2.path}
		}
		if file.stage2.mode != file.stage3.mode {
			// 5c. Conflicting mode change
			return &errMergeConflict{file.stage2.path}
		}

		// Need to get the objects from the object db to attempt to merge
		root, _, err := git.NewCommand(ctx, "unpack-file").AddDynamicArguments(file.stage1.sha).RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if err != nil {
			return fmt.Errorf("unable to get root object: %s at path: %s for merging. Error: %w", file.stage1.sha, file.stage1.path, err)
		}
		root = strings.TrimSpace(root)
		defer func() {
			_ = util.Remove(filepath.Join(tmpBasePath, root))
		}()

		base, _, err := git.NewCommand(ctx, "unpack-file").AddDynamicArguments(file.stage2.sha).RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if err != nil {
			return fmt.Errorf("unable to get base object: %s at path: %s for merging. Error: %w", file.stage2.sha, file.stage2.path, err)
		}
		base = strings.TrimSpace(filepath.Join(tmpBasePath, base))
		defer func() {
			_ = util.Remove(base)
		}()
		head, _, err := git.NewCommand(ctx, "unpack-file").AddDynamicArguments(file.stage3.sha).RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if err != nil {
			return fmt.Errorf("unable to get head object:%s at path: %s for merging. Error: %w", file.stage3.sha, file.stage3.path, err)
		}
		head = strings.TrimSpace(head)
		defer func() {
			_ = util.Remove(filepath.Join(tmpBasePath, head))
		}()

		// now git merge-file annoyingly takes a different order to the merge-tree ...
		_, _, conflictErr := git.NewCommand(ctx, "merge-file").AddDynamicArguments(base, root, head).RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if conflictErr != nil {
			return &errMergeConflict{file.stage2.path}
		}

		// base now contains the merged data
		hash, _, err := git.NewCommand(ctx, "hash-object", "-w", "--path").AddDynamicArguments(file.stage2.path, base).RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if err != nil {
			return err
		}
		hash = strings.TrimSpace(hash)
		return gitRepo.AddObjectToIndex(file.stage2.mode, git.MustIDFromString(hash), file.stage2.path)
	default:
		if file.stage1 != nil {
			return &errMergeConflict{file.stage1.path}
		} else if file.stage2 != nil {
			return &errMergeConflict{file.stage2.path}
		} else if file.stage3 != nil {
			return &errMergeConflict{file.stage3.path}
		}
	}
	return nil
}

// AttemptThreeWayMerge will attempt to three way merge using git read-tree and then follow the git merge-one-file algorithm to attempt to resolve basic conflicts
func AttemptThreeWayMerge(ctx context.Context, gitPath string, gitRepo *git.Repository, base, ours, theirs, description string) (bool, []string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// First we use read-tree to do a simple three-way merge
	if _, _, err := git.NewCommand(ctx, "read-tree", "-m").AddDynamicArguments(base, ours, theirs).RunStdString(&git.RunOpts{Dir: gitPath}); err != nil {
		log.Error("Unable to run read-tree -m! Error: %v", err)
		return false, nil, fmt.Errorf("unable to run read-tree -m! Error: %w", err)
	}

	// Then we use git ls-files -u to list the unmerged files and collate the triples in unmergedfiles
	unmerged := make(chan *unmergedFile)
	go unmergedFiles(ctx, gitPath, unmerged)

	defer func() {
		cancel()
		for range unmerged {
			// empty the unmerged channel
		}
	}()

	numberOfConflicts := 0
	conflict := false
	conflictedFiles := make([]string, 0, 5)

	for file := range unmerged {
		if file == nil {
			break
		}
		if file.err != nil {
			cancel()
			return false, nil, file.err
		}

		// OK now we have the unmerged file triplet attempt to merge it
		if err := attemptMerge(ctx, file, gitPath, gitRepo); err != nil {
			if conflictErr, ok := err.(*errMergeConflict); ok {
				log.Trace("Conflict: %s in %s", conflictErr.filename, description)
				conflict = true
				if numberOfConflicts < 10 {
					conflictedFiles = append(conflictedFiles, conflictErr.filename)
				}
				numberOfConflicts++
				continue
			}
			return false, nil, err
		}
	}
	return conflict, conflictedFiles, nil
}

func checkConflicts(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository, tmpBasePath string) (bool, error) {
	// 1. checkConflicts resets the conflict status - therefore - reset the conflict status
	pr.ConflictedFiles = nil

	// 2. AttemptThreeWayMerge first - this is much quicker than plain patch to base
	description := fmt.Sprintf("PR[%d] %s/%s#%d", pr.ID, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, pr.Index)
	conflict, conflictFiles, err := AttemptThreeWayMerge(ctx,
		tmpBasePath, gitRepo, pr.MergeBase, "base", "tracking", description)
	if err != nil {
		return false, err
	}

	if !conflict {
		// No conflicts detected so we need to check if the patch is empty...
		// a. Write the newly merged tree and check the new tree-hash
		var treeHash string
		treeHash, _, err = git.NewCommand(ctx, "write-tree").RunStdString(&git.RunOpts{Dir: tmpBasePath})
		if err != nil {
			lsfiles, _, _ := git.NewCommand(ctx, "ls-files", "-u").RunStdString(&git.RunOpts{Dir: tmpBasePath})
			return false, fmt.Errorf("unable to write unconflicted tree: %w\n`git ls-files -u`:\n%s", err, lsfiles)
		}
		treeHash = strings.TrimSpace(treeHash)
		baseTree, err := gitRepo.GetTree("base")
		if err != nil {
			return false, err
		}

		// b. compare the new tree-hash with the base tree hash
		if treeHash == baseTree.ID.String() {
			log.Debug("PullRequest[%d]: Patch is empty - ignoring", pr.ID)
			pr.Status = issues_model.PullRequestStatusEmpty
		}

		return false, nil
	}

	// 3. OK the three-way merge method has detected conflicts
	// 3a. Are still testing with GitApply? If not set the conflict status and move on
	if !setting.Repository.PullRequest.TestConflictingPatchesWithGitApply {
		pr.Status = issues_model.PullRequestStatusConflict
		pr.ConflictedFiles = conflictFiles

		log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
		return true, nil
	}

	// 3b. Create a plain patch from head to base
	tmpPatchFile, err := os.CreateTemp("", "patch")
	if err != nil {
		log.Error("Unable to create temporary patch file! Error: %v", err)
		return false, fmt.Errorf("unable to create temporary patch file! Error: %w", err)
	}
	defer func() {
		_ = util.Remove(tmpPatchFile.Name())
	}()

	if err := gitRepo.GetDiffBinary(pr.MergeBase, "tracking", tmpPatchFile); err != nil {
		tmpPatchFile.Close()
		log.Error("Unable to get patch file from %s to %s in %s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
		return false, fmt.Errorf("unable to get patch file from %s to %s in %s Error: %w", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
	}
	stat, err := tmpPatchFile.Stat()
	if err != nil {
		tmpPatchFile.Close()
		return false, fmt.Errorf("unable to stat patch file: %w", err)
	}
	patchPath := tmpPatchFile.Name()
	tmpPatchFile.Close()

	// 3c. if the size of that patch is 0 - there can be no conflicts!
	if stat.Size() == 0 {
		log.Debug("PullRequest[%d]: Patch is empty - ignoring", pr.ID)
		pr.Status = issues_model.PullRequestStatusEmpty
		return false, nil
	}

	log.Trace("PullRequest[%d].testPatch (patchPath): %s", pr.ID, patchPath)

	// 4. Read the base branch in to the index of the temporary repository
	_, _, err = git.NewCommand(gitRepo.Ctx, "read-tree", "base").RunStdString(&git.RunOpts{Dir: tmpBasePath})
	if err != nil {
		return false, fmt.Errorf("git read-tree %s: %w", pr.BaseBranch, err)
	}

	// 5. Now get the pull request configuration to check if we need to ignore whitespace
	prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		return false, err
	}
	prConfig := prUnit.PullRequestsConfig()

	// 6. Prepare the arguments to apply the patch against the index
	cmdApply := git.NewCommand(gitRepo.Ctx, "apply", "--check", "--cached")
	if prConfig.IgnoreWhitespaceConflicts {
		cmdApply.AddArguments("--ignore-whitespace")
	}
	is3way := false
	if git.DefaultFeatures().CheckVersionAtLeast("2.32.0") {
		cmdApply.AddArguments("--3way")
		is3way = true
	}
	cmdApply.AddDynamicArguments(patchPath)

	// 7. Prep the pipe:
	//   - Here we could do the equivalent of:
	//  `git apply --check --cached patch_file > conflicts`
	//     Then iterate through the conflicts. However, that means storing all the conflicts
	//     in memory - which is very wasteful.
	//   - alternatively we can do the equivalent of:
	//  `git apply --check ... | grep ...`
	//     meaning we don't store all of the conflicts unnecessarily.
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to open stderr pipe: %v", err)
		return false, fmt.Errorf("unable to open stderr pipe: %w", err)
	}
	defer func() {
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
	}()

	// 8. Run the check command
	conflict = false
	err = cmdApply.Run(&git.RunOpts{
		Dir:    tmpBasePath,
		Stderr: stderrWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			// Close the writer end of the pipe to begin processing
			_ = stderrWriter.Close()
			defer func() {
				// Close the reader on return to terminate the git command if necessary
				_ = stderrReader.Close()
			}()

			const prefix = "error: patch failed:"
			const errorPrefix = "error: "
			const threewayFailed = "Failed to perform three-way merge..."
			const appliedPatchPrefix = "Applied patch to '"
			const withConflicts = "' with conflicts."

			conflicts := make(container.Set[string])

			// Now scan the output from the command
			scanner := bufio.NewScanner(stderrReader)
			for scanner.Scan() {
				line := scanner.Text()
				log.Trace("PullRequest[%d].testPatch: stderr: %s", pr.ID, line)
				if strings.HasPrefix(line, prefix) {
					conflict = true
					filepath := strings.TrimSpace(strings.Split(line[len(prefix):], ":")[0])
					conflicts.Add(filepath)
				} else if is3way && line == threewayFailed {
					conflict = true
				} else if strings.HasPrefix(line, errorPrefix) {
					conflict = true
					for _, suffix := range patchErrorSuffices {
						if strings.HasSuffix(line, suffix) {
							filepath := strings.TrimSpace(strings.TrimSuffix(line[len(errorPrefix):], suffix))
							if filepath != "" {
								conflicts.Add(filepath)
							}
							break
						}
					}
				} else if is3way && strings.HasPrefix(line, appliedPatchPrefix) && strings.HasSuffix(line, withConflicts) {
					conflict = true
					filepath := strings.TrimPrefix(strings.TrimSuffix(line, withConflicts), appliedPatchPrefix)
					if filepath != "" {
						conflicts.Add(filepath)
					}
				}
				// only list 10 conflicted files
				if len(conflicts) >= 10 {
					break
				}
			}

			if len(conflicts) > 0 {
				pr.ConflictedFiles = make([]string, 0, len(conflicts))
				for key := range conflicts {
					pr.ConflictedFiles = append(pr.ConflictedFiles, key)
				}
			}

			return nil
		},
	})

	// 9. Check if the found conflictedfiles is non-zero, "err" could be non-nil, so we should ignore it if we found conflicts.
	// Note: `"err" could be non-nil` is due that if enable 3-way merge, it doesn't return any error on found conflicts.
	if len(pr.ConflictedFiles) > 0 {
		if conflict {
			pr.Status = issues_model.PullRequestStatusConflict
			log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)

			return true, nil
		}
	} else if err != nil {
		return false, fmt.Errorf("git apply --check: %w", err)
	}
	return false, nil
}

// CheckFileProtection check file Protection
func CheckFileProtection(repo *git.Repository, oldCommitID, newCommitID string, patterns []glob.Glob, limit int, env []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	affectedFiles, err := git.GetAffectedFiles(repo, oldCommitID, newCommitID, env)
	if err != nil {
		return nil, err
	}
	changedProtectedFiles := make([]string, 0, limit)
	for _, affectedFile := range affectedFiles {
		lpath := strings.ToLower(affectedFile)
		for _, pat := range patterns {
			if pat.Match(lpath) {
				changedProtectedFiles = append(changedProtectedFiles, lpath)
				break
			}
		}
		if len(changedProtectedFiles) >= limit {
			break
		}
	}
	if len(changedProtectedFiles) > 0 {
		err = models.ErrFilePathProtected{
			Path: changedProtectedFiles[0],
		}
	}
	return changedProtectedFiles, err
}

// CheckUnprotectedFiles check if the commit only touches unprotected files
func CheckUnprotectedFiles(repo *git.Repository, oldCommitID, newCommitID string, patterns []glob.Glob, env []string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}
	affectedFiles, err := git.GetAffectedFiles(repo, oldCommitID, newCommitID, env)
	if err != nil {
		return false, err
	}
	for _, affectedFile := range affectedFiles {
		lpath := strings.ToLower(affectedFile)
		unprotected := false
		for _, pat := range patterns {
			if pat.Match(lpath) {
				unprotected = true
				break
			}
		}
		if !unprotected {
			return false, nil
		}
	}
	return true, nil
}

// checkPullFilesProtection check if pr changed protected files and save results
func checkPullFilesProtection(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository) error {
	if pr.Status == issues_model.PullRequestStatusEmpty {
		pr.ChangedProtectedFiles = nil
		return nil
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return err
	}

	if pb == nil {
		pr.ChangedProtectedFiles = nil
		return nil
	}

	pr.ChangedProtectedFiles, err = CheckFileProtection(gitRepo, pr.MergeBase, "tracking", pb.GetProtectedFilePatterns(), 10, os.Environ())
	if err != nil && !models.IsErrFilePathProtected(err) {
		return err
	}
	return nil
}
