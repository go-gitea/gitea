// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	repo_module "code.gitea.io/gitea/modules/repository"
)

// GetConflictedFileContent performs a 3-way merge for filePath and returns the
// file content with standard git conflict markers.
//
// It works directly against the real head/base repository directories using
// git cat-file and git merge-file, avoiding the expensive full clone performed
// by createTemporaryRepoForPR.  Only a tiny temp directory holding the three
// extracted blob files is created.
//
// pr.MergeBase must be populated (stored in the DB after the conflict check).
func GetConflictedFileContent(ctx context.Context, pr *issues_model.PullRequest, filePath string) (string, error) {
	if loadErr := pr.LoadBaseRepo(ctx); loadErr != nil {
		return "", fmt.Errorf("load base repo: %w", loadErr)
	}
	if loadErr := pr.LoadHeadRepo(ctx); loadErr != nil {
		return "", fmt.Errorf("load head repo: %w", loadErr)
	}

	baseRepoPath := pr.BaseRepo.RepoPath()

	// For same-repo PRs the head path is identical; for forks use the fork's path.
	headRepoPath := baseRepoPath
	if !pr.IsSameRepo() {
		headRepoPath = pr.HeadRepo.RepoPath()
	}

	// Resolve branch tips directly from the real repositories.
	baseTipSHA, _, gitErr := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(git.BranchPrefix + pr.BaseBranch).
		WithDir(baseRepoPath).RunStdString(ctx)
	if gitErr != nil {
		return "", fmt.Errorf("rev-parse base branch: %w", gitErr)
	}
	baseTipSHA = strings.TrimSpace(baseTipSHA)

	headTipSHA, _, gitErr := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(git.BranchPrefix + pr.HeadBranch).
		WithDir(headRepoPath).RunStdString(ctx)
	if gitErr != nil {
		return "", fmt.Errorf("rev-parse head branch: %w", gitErr)
	}
	headTipSHA = strings.TrimSpace(headTipSHA)

	// Prefer the stored merge base (set by the conflict check); recompute only
	// if it is missing.
	mergeBaseSHA := pr.MergeBase
	if mergeBaseSHA == "" {
		var mbErr error
		mergeBaseSHA, mbErr = gitrepo.MergeBase(ctx, pr.BaseRepo, baseTipSHA, headTipSHA)
		if mbErr != nil {
			return "", fmt.Errorf("compute merge-base: %w", mbErr)
		}
	}

	// Resolve blob SHAs. Ancestor and base blobs are in the base repo.
	// Head blob is in the head repo (fork or same).
	ancestorBlobSHA, _ := getBlobSHAFromRef(ctx, baseRepoPath, mergeBaseSHA, filePath)

	baseBlobSHA, refErr := getBlobSHAFromRef(ctx, baseRepoPath, baseTipSHA, filePath)
	if refErr != nil {
		return "", fmt.Errorf("file %q not found in base branch: %w", filePath, refErr)
	}

	headBlobSHA, refErr := getBlobSHAFromRef(ctx, headRepoPath, headTipSHA, filePath)
	if refErr != nil {
		return "", fmt.Errorf("file %q not found in head branch: %w", filePath, refErr)
	}

	// Create a minimal temp directory for the three plain content files.
	// git merge-file works on regular files and does not require a git repo.
	tmpDir, tmpErr := os.MkdirTemp("", "gitea-conflict-*")
	if tmpErr != nil {
		return "", fmt.Errorf("create temp dir: %w", tmpErr)
	}
	defer os.RemoveAll(tmpDir)

	// Extract blobs via git cat-file blob.
	headFile, exErr := extractBlobToTempFile(ctx, headRepoPath, headBlobSHA, tmpDir, "head")
	if exErr != nil {
		return "", fmt.Errorf("extract head blob: %w", exErr)
	}

	baseFile, exErr := extractBlobToTempFile(ctx, baseRepoPath, baseBlobSHA, tmpDir, "base")
	if exErr != nil {
		return "", fmt.Errorf("extract base blob: %w", exErr)
	}

	var ancestorFile string
	if ancestorBlobSHA != "" {
		ancestorFile, exErr = extractBlobToTempFile(ctx, baseRepoPath, ancestorBlobSHA, tmpDir, "ancestor")
		if exErr != nil {
			return "", fmt.Errorf("extract ancestor blob: %w", exErr)
		}
	} else {
		// File added independently on both branches — use an empty ancestor.
		f, ferr := os.CreateTemp(tmpDir, "ancestor-empty-*")
		if ferr != nil {
			return "", fmt.Errorf("create empty ancestor: %w", ferr)
		}
		f.Close()
		ancestorFile = f.Name()
	}

	// git merge-file <current> <ancestor> <other>
	// current = head file  -> labeled HeadBranch in <<<<<<< line
	// other   = base file  -> labeled BaseBranch in >>>>>>> line
	// Non-zero exit is expected when conflicts remain unresolvable.
	_ = gitcmd.NewCommand("merge-file", "-L").
		AddDynamicArguments(pr.HeadBranch).
		AddArguments("-L", "base", "-L").
		AddDynamicArguments(pr.BaseBranch).
		AddDynamicArguments(headFile, ancestorFile, baseFile).
		WithDir(baseRepoPath).RunWithStderr(ctx)

	content, readErr := os.ReadFile(headFile)
	if readErr != nil {
		return "", fmt.Errorf("read merged file: %w", readErr)
	}
	return string(content), nil
}

// ResolvedFile holds the path and resolved content for a conflicted file.
type ResolvedFile struct {
	Path    string
	Content string
}

// CommitConflictResolution creates a merge commit on the head branch whose
// second parent is the current base branch tip.  This makes the PR conflict
// check find no conflicts.  After a successful push the function schedules an
// immediate re-check so the merge box updates without waiting for the next
// page load.
func CommitConflictResolution(
	ctx context.Context,
	pr *issues_model.PullRequest,
	doer *user_model.User,
	resolvedFiles []ResolvedFile,
	commitMsg string,
	commitEnv []string,
) error {
	if loadErr := pr.LoadBaseRepo(ctx); loadErr != nil {
		return fmt.Errorf("load base repo: %w", loadErr)
	}
	if loadErr := pr.LoadHeadRepo(ctx); loadErr != nil {
		return fmt.Errorf("load head repo: %w", loadErr)
	}

	baseRepoPath := pr.BaseRepo.RepoPath()
	headRepoPath := pr.HeadRepo.RepoPath()

	// Resolve current branch tips from the real repositories.
	baseTipSHA, _, gitErr := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(git.BranchPrefix + pr.BaseBranch).
		WithDir(baseRepoPath).RunStdString(ctx)
	if gitErr != nil {
		return fmt.Errorf("rev-parse base: %w", gitErr)
	}
	baseTipSHA = strings.TrimSpace(baseTipSHA)

	headTipSHA, _, gitErr := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(git.BranchPrefix + pr.HeadBranch).
		WithDir(headRepoPath).RunStdString(ctx)
	if gitErr != nil {
		return fmt.Errorf("rev-parse head: %w", gitErr)
	}
	headTipSHA = strings.TrimSpace(headTipSHA)

	// Build a minimal bare git repository using only Go file operations—
	// no git init or git fetch needed. Alternates make both repos' objects
	// readable; new objects are written to tmpDir and pushed to head repo.
	tmpDir, tmpErr := os.MkdirTemp("", "gitea-conflict-merge-*")
	if tmpErr != nil {
		return fmt.Errorf("create temp dir: %w", tmpErr)
	}
	defer os.RemoveAll(tmpDir)

	if initErr := initMinimalBareRepo(tmpDir, pr, baseTipSHA, headTipSHA); initErr != nil {
		return fmt.Errorf("init minimal bare repo: %w", initErr)
	}

	// tmpDir is now a bare git repo with:
	//   refs/heads/base     -> baseTipSHA
	//   refs/heads/tracking -> headTipSHA
	//   objects via alternates pointing to the real repos

	// 1. Load the tracking-branch tree into the index (no checkout needed).
	if gitErr := gitcmd.NewCommand("read-tree").
		AddDynamicArguments(tmpRepoTrackingBranch).
		WithDir(tmpDir).RunWithStderr(ctx); gitErr != nil {
		return fmt.Errorf("read-tree: %w", gitErr)
	}

	// 2. Write each resolved file as a blob and stage it.
	for _, f := range resolvedFiles {
		content := strings.ReplaceAll(f.Content, "\r", "")

		blobHash, _, gitErr := gitcmd.NewCommand("hash-object", "-w", "--stdin").
			WithStdinBytes([]byte(content)).
			WithDir(tmpDir).RunStdString(ctx)
		if gitErr != nil {
			return fmt.Errorf("hash-object %s: %w", f.Path, gitErr)
		}
		blobHash = strings.TrimSpace(blobHash)

		mode := "100644"
		if out, _, _ := gitcmd.NewCommand("ls-tree").
			AddDynamicArguments(tmpRepoTrackingBranch, f.Path).
			WithDir(tmpDir).RunStdString(ctx); out != "" {
			if fields := strings.Fields(out); len(fields) > 0 {
				mode = fields[0]
			}
		}

		if gitErr := gitcmd.NewCommand("update-index", "--cacheinfo").
			AddDynamicArguments(mode + "," + blobHash + "," + f.Path).
			WithDir(tmpDir).RunWithStderr(ctx); gitErr != nil {
			return fmt.Errorf("update-index %s: %w", f.Path, gitErr)
		}
	}

	// 3. Build the new tree object.
	newTreeHash, _, gitErr := gitcmd.NewCommand("write-tree").
		WithDir(tmpDir).RunStdString(ctx)
	if gitErr != nil {
		return fmt.Errorf("write-tree: %w", gitErr)
	}
	newTreeHash = strings.TrimSpace(newTreeHash)

	// 4. Parent SHAs were already resolved from the real repos above.

	// 5. Create the merge commit: parent1 = old head tip, parent2 = base tip.
	newCommitHash, _, gitErr := gitcmd.NewCommand("commit-tree").
		AddDynamicArguments(newTreeHash).
		AddArguments("-p").AddDynamicArguments(headTipSHA).
		AddArguments("-p").AddDynamicArguments(baseTipSHA).
		AddArguments("-m").AddDynamicArguments(commitMsg).
		WithEnv(commitEnv).
		WithDir(tmpDir).RunStdString(ctx)
	if gitErr != nil {
		return fmt.Errorf("commit-tree: %w", gitErr)
	}
	newCommitHash = strings.TrimSpace(newCommitHash)

	// 6. Push to the actual head repository.
	// InternalPushingEnvironment bypasses server-side hooks; the re-check is
	// triggered explicitly in step 7.
	pushEnv := repo_module.InternalPushingEnvironment(doer, pr.HeadRepo)
	if pushErr := gitrepo.PushFromLocal(ctx, tmpDir, pr.HeadRepo, git.PushOptions{
		Branch: newCommitHash + ":" + git.BranchPrefix + pr.HeadBranch,
		Env:    pushEnv,
	}); pushErr != nil {
		return fmt.Errorf("push to head repo: %w", pushErr)
	}

	// 7. Schedule an immediate re-check so the PR page shows "checking" /
	// "mergeable" without waiting for a subsequent page load or git-hook.
	StartPullRequestCheckImmediately(ctx, pr)

	return nil
}

// getBlobSHAFromRef returns the blob object SHA for filePath at the given git
// ref (branch name or commit SHA) inside repoPath.
func getBlobSHAFromRef(ctx context.Context, repoPath, ref, filePath string) (string, error) {
	out, _, gitErr := gitcmd.NewCommand("ls-tree", "--").
		AddDynamicArguments(ref, filePath).
		WithDir(repoPath).RunStdString(ctx)
	if gitErr != nil {
		return "", gitErr
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", fmt.Errorf("file %q not found at ref %s", filePath, ref)
	}
	// ls-tree output: "{mode} {type} {sha}\t{path}"
	fields := strings.Fields(out)
	if len(fields) < 3 {
		return "", fmt.Errorf("unexpected ls-tree output: %s", out)
	}
	return fields[2], nil
}

// extractBlobToTempFile reads a git blob from repoPath via `git cat-file blob`
// and writes it to a new temp file inside tmpDir, returning the full path.
func extractBlobToTempFile(ctx context.Context, repoPath, sha, tmpDir, label string) (string, error) {
	data, _, gitErr := gitcmd.NewCommand("cat-file", "blob").
		AddDynamicArguments(sha).
		WithDir(repoPath).RunStdBytes(ctx)
	if gitErr != nil {
		return "", fmt.Errorf("cat-file blob %s (%s): %w", sha, label, gitErr)
	}
	f, ferr := os.CreateTemp(tmpDir, label+"-*")
	if ferr != nil {
		return "", ferr
	}
	defer f.Close()
	if _, werr := f.Write(data); werr != nil {
		return "", werr
	}
	return filepath.Join(tmpDir, filepath.Base(f.Name())), nil
}

// initMinimalBareRepo creates a minimal bare git repository in dir using only
// Go file operations (no git init, no git fetch).  The repository's object
// store is wired via alternates to the real base and head repositories, making
// all existing git objects readable without copying them.  Two fake branch refs
// are written so subsequent git plumbing commands can address the branch tips
// by the conventional names (tmpRepoBaseBranch / tmpRepoTrackingBranch).
func initMinimalBareRepo(dir string, pr *issues_model.PullRequest, baseTipSHA, headTipSHA string) error {
	// Create the minimum directory structure required for a bare git repo.
	for _, sub := range []string{
		filepath.Join(dir, "objects", "info"),
		filepath.Join(dir, "objects", "pack"),
		filepath.Join(dir, "refs", "heads"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return err
		}
	}

	// HEAD must exist for git to recognise the directory as a repo.
	if err := os.WriteFile(
		filepath.Join(dir, "HEAD"),
		[]byte("ref: refs/heads/"+tmpRepoTrackingBranch+"\n"),
		0o644,
	); err != nil {
		return err
	}

	// alternates: make both repos' objects readable from this bare repo.
	// For same-repo PRs the base and head paths are identical; write the
	// path once (deduplication is not required but keeps the file tidy).
	baseObjPath := filepath.Join(pr.BaseRepo.RepoPath(), "objects")
	headObjPath := filepath.Join(pr.HeadRepo.RepoPath(), "objects")
	alternates := baseObjPath + "\n"
	if !pr.IsSameRepo() {
		alternates += headObjPath + "\n"
	}
	if err := os.WriteFile(
		filepath.Join(dir, "objects", "info", "alternates"),
		[]byte(alternates),
		0o644,
	); err != nil {
		return err
	}

	// Write fake branch refs so git plumbing commands can address both tips.
	for name, sha := range map[string]string{
		tmpRepoBaseBranch:     baseTipSHA,
		tmpRepoTrackingBranch: headTipSHA,
	} {
		if err := os.WriteFile(
			filepath.Join(dir, "refs", "heads", name),
			[]byte(sha+"\n"),
			0o644,
		); err != nil {
			return err
		}
	}
	return nil
}
