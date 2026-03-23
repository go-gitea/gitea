// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
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

	compareArg := pr.MergeBase + "..." + pr.GetGitHeadRefName()
	switch {
	case patch:
		err = gitRepo.GetPatch(compareArg, w)
	case binary:
		err = gitRepo.GetDiffBinary(compareArg, w)
	default:
		err = gitRepo.GetDiff(compareArg, w)
	}

	if err != nil {
		log.Error("unable to get patch file from %s to %s in %s Error: %v", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
		return fmt.Errorf("unable to get patch file from %s to %s in %s Error: %w", pr.MergeBase, pr.HeadBranch, pr.BaseRepo.FullName(), err)
	}
	return nil
}

func checkPullRequestBranchMergeable(ctx context.Context, pr *issues_model.PullRequest) error {
	ctx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("checkPullRequestBranchMergeable: %s", pr))
	defer finished()

	if git.DefaultFeatures().SupportGitMergeTree {
		return checkPullRequestMergeableByMergeTree(ctx, pr)
	}

	return checkPullRequestMergeableByTmpRepo(ctx, pr)
}

func checkPullRequestMergeableByTmpRepo(ctx context.Context, pr *issues_model.PullRequest) error {
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		if !git_model.IsErrBranchNotExist(err) {
			log.Error("CreateTemporaryRepoForPR %-v: %v", pr, err)
		}
		return err
	}
	defer cancel()

	gitRepo, err := git.OpenRepository(ctx, prCtx.tmpBasePath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	defer gitRepo.Close()

	// 1. update merge base
	pr.MergeBase, _, err = gitcmd.NewCommand("merge-base", "--", tmpRepoBaseBranch, tmpRepoTrackingBranch).WithDir(prCtx.tmpBasePath).RunStdString(ctx)
	if err != nil {
		var err2 error
		pr.MergeBase, err2 = gitRepo.GetRefCommitID(git.BranchPrefix + tmpRepoBaseBranch)
		if err2 != nil {
			return fmt.Errorf("GetMergeBase: %v and can't find commit ID for base: %w", err, err2)
		}
	}
	pr.MergeBase = strings.TrimSpace(pr.MergeBase)
	if pr.HeadCommitID, err = gitRepo.GetRefCommitID(git.BranchPrefix + tmpRepoTrackingBranch); err != nil {
		return fmt.Errorf("GetBranchCommitID: can't find commit ID for head: %w", err)
	}

	if pr.HeadCommitID == pr.MergeBase {
		pr.Status = issues_model.PullRequestStatusAncestor
		return nil
	}

	// 2. Check for conflicts
	conflicts, err := checkConflictsByTmpRepo(ctx, pr, gitRepo, prCtx.tmpBasePath)
	if err != nil {
		return err
	}

	pr.ChangedProtectedFiles = nil
	if conflicts || pr.Status == issues_model.PullRequestStatusEmpty {
		return nil
	}

	// 3. Check for protected files changes
	if err = checkPullFilesProtection(ctx, pr, gitRepo, tmpRepoTrackingBranch); err != nil {
		return fmt.Errorf("pr.CheckPullFilesProtection(): %w", err)
	}

	pr.Status = issues_model.PullRequestStatusMergeable

	return nil
}

type errMergeConflict struct {
	filename string
}

func (e *errMergeConflict) Error() string {
	return "conflict detected at: " + e.filename
}

func attemptMerge(ctx context.Context, file *unmergedFile, tmpBasePath string, filesToRemove *[]string, filesToAdd *[]git.IndexObjectInfo) error {
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
		*filesToRemove = append(*filesToRemove, file.stage1.path)
		return nil
	case file.stage1 == nil && file.stage2 != nil && (file.stage3 == nil || file.stage2.SameAs(file.stage3)):
		// 2. Added in ours but not in theirs or identical in both
		//
		// Not a genuine conflict just add to the index
		*filesToAdd = append(*filesToAdd, git.IndexObjectInfo{Mode: file.stage2.mode, Object: git.MustIDFromString(file.stage2.sha), Filename: file.stage2.path})
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
		*filesToAdd = append(*filesToAdd, git.IndexObjectInfo{Mode: file.stage3.mode, Object: git.MustIDFromString(file.stage3.sha), Filename: file.stage3.path})
		return nil
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
		root, _, err := gitcmd.NewCommand("unpack-file").AddDynamicArguments(file.stage1.sha).WithDir(tmpBasePath).RunStdString(ctx)
		if err != nil {
			return fmt.Errorf("unable to get root object: %s at path: %s for merging. Error: %w", file.stage1.sha, file.stage1.path, err)
		}
		root = strings.TrimSpace(root)
		defer func() {
			_ = util.Remove(filepath.Join(tmpBasePath, root))
		}()

		base, _, err := gitcmd.NewCommand("unpack-file").AddDynamicArguments(file.stage2.sha).WithDir(tmpBasePath).RunStdString(ctx)
		if err != nil {
			return fmt.Errorf("unable to get base object: %s at path: %s for merging. Error: %w", file.stage2.sha, file.stage2.path, err)
		}
		base = strings.TrimSpace(filepath.Join(tmpBasePath, base))
		defer func() {
			_ = util.Remove(base)
		}()
		head, _, err := gitcmd.NewCommand("unpack-file").AddDynamicArguments(file.stage3.sha).WithDir(tmpBasePath).RunStdString(ctx)
		if err != nil {
			return fmt.Errorf("unable to get head object:%s at path: %s for merging. Error: %w", file.stage3.sha, file.stage3.path, err)
		}
		head = strings.TrimSpace(head)
		defer func() {
			_ = util.Remove(filepath.Join(tmpBasePath, head))
		}()

		// now git merge-file annoyingly takes a different order to the merge-tree ...
		_, _, conflictErr := gitcmd.NewCommand("merge-file").AddDynamicArguments(base, root, head).WithDir(tmpBasePath).RunStdString(ctx)
		if conflictErr != nil {
			return &errMergeConflict{file.stage2.path}
		}

		// base now contains the merged data
		hash, _, err := gitcmd.NewCommand("hash-object", "-w", "--path").AddDynamicArguments(file.stage2.path, base).WithDir(tmpBasePath).RunStdString(ctx)
		if err != nil {
			return err
		}
		hash = strings.TrimSpace(hash)
		*filesToAdd = append(*filesToAdd, git.IndexObjectInfo{Mode: file.stage2.mode, Object: git.MustIDFromString(hash), Filename: file.stage2.path})
		return nil
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
	if err := gitcmd.NewCommand("read-tree", "-m").AddDynamicArguments(base, ours, theirs).WithDir(gitPath).RunWithStderr(ctx); err != nil {
		log.Error("Unable to run read-tree -m! Error: %v", err)
		return false, nil, fmt.Errorf("unable to run read-tree -m! Error: %w", err)
	}

	var filesToRemove []string
	var filesToAdd []git.IndexObjectInfo

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
		if err := attemptMerge(ctx, file, gitPath, &filesToRemove, &filesToAdd); err != nil {
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

	// Add and remove files in one command, as this is slow with many files otherwise
	if err := gitRepo.RemoveFilesFromIndex(filesToRemove...); err != nil {
		return false, nil, err
	}
	if err := gitRepo.AddObjectsToIndex(filesToAdd...); err != nil {
		return false, nil, err
	}

	return conflict, conflictedFiles, nil
}

func checkConflictsByTmpRepo(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository, tmpBasePath string) (bool, error) {
	// 1. checkConflictsByTmpRepo resets the conflict status - therefore - reset the conflict status
	pr.ConflictedFiles = nil

	// 2. AttemptThreeWayMerge first - this is much quicker than plain patch to base
	description := fmt.Sprintf("PR[%d] %s/%s#%d", pr.ID, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, pr.Index)
	conflict, conflictFiles, err := AttemptThreeWayMerge(ctx,
		tmpBasePath, gitRepo, pr.MergeBase, tmpRepoBaseBranch, tmpRepoTrackingBranch, description)
	if err != nil {
		return false, err
	}

	if !conflict {
		// No conflicts detected so we need to check if the patch is empty...
		// a. Write the newly merged tree and check the new tree-hash
		var treeHash string
		treeHash, _, err = gitcmd.NewCommand("write-tree").WithDir(tmpBasePath).RunStdString(ctx)
		if err != nil {
			lsfiles, _, _ := gitcmd.NewCommand("ls-files", "-u").WithDir(tmpBasePath).RunStdString(ctx)
			return false, fmt.Errorf("unable to write unconflicted tree: %w\n`git ls-files -u`:\n%s", err, lsfiles)
		}
		treeHash = strings.TrimSpace(treeHash)
		baseTree, err := gitRepo.GetTree(tmpRepoBaseBranch)
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
	pr.Status = issues_model.PullRequestStatusConflict
	pr.ConflictedFiles = conflictFiles
	log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
	return true, nil
}

// ErrFilePathProtected represents a "FilePathProtected" kind of error.
type ErrFilePathProtected struct {
	Message string
	Path    string
}

// IsErrFilePathProtected checks if an error is an ErrFilePathProtected.
func IsErrFilePathProtected(err error) bool {
	_, ok := err.(ErrFilePathProtected)
	return ok
}

func (err ErrFilePathProtected) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("path is protected and can not be changed [path: %s]", err.Path)
}

func (err ErrFilePathProtected) Unwrap() error {
	return util.ErrPermissionDenied
}

// CheckFileProtection check file Protection
func CheckFileProtection(repo *git.Repository, branchName, oldCommitID, newCommitID string, patterns []glob.Glob, limit int, env []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	affectedFiles, err := git.GetAffectedFiles(repo, branchName, oldCommitID, newCommitID, env)
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
		err = ErrFilePathProtected{
			Path: changedProtectedFiles[0],
		}
	}
	return changedProtectedFiles, err
}

// CheckUnprotectedFiles check if the commit only touches unprotected files
func CheckUnprotectedFiles(repo *git.Repository, branchName, oldCommitID, newCommitID string, patterns []glob.Glob, env []string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}
	affectedFiles, err := git.GetAffectedFiles(repo, branchName, oldCommitID, newCommitID, env)
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
func checkPullFilesProtection(ctx context.Context, pr *issues_model.PullRequest, gitRepo *git.Repository, headRef string) error {
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

	pr.ChangedProtectedFiles, err = CheckFileProtection(gitRepo, pr.HeadBranch, pr.MergeBase, headRef, pb.GetProtectedFilePatterns(), 10, os.Environ())
	if err != nil && !IsErrFilePathProtected(err) {
		return err
	}
	if len(pr.ChangedProtectedFiles) > 0 {
		log.Trace("Found %d protected files changed in PR %s#%d", len(pr.ChangedProtectedFiles), pr.BaseRepo.FullName(), pr.Index)
	}
	return nil
}
