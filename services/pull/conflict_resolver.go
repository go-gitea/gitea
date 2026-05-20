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
	"code.gitea.io/gitea/modules/util"
)

// GetConflictedFileContent creates a temporary repo for the PR, performs a
// three-way merge for the given file path, and returns the file content with
// standard git conflict markers so the user can resolve it in the web editor.
// The HEAD branch changes are labeled with pr.HeadBranch and the base branch
// changes are labeled with pr.BaseBranch.
func GetConflictedFileContent(ctx context.Context, pr *issues_model.PullRequest, filePath string) (string, error) {
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		return "", fmt.Errorf("createTemporaryRepoForPR: %w", err)
	}
	defer cancel()

	tmpDir := prCtx.tmpBasePath

	// Find the common ancestor commit of both branches.
	mergeBase, _, err := gitcmd.NewCommand("merge-base", "--").
		AddDynamicArguments(tmpRepoBaseBranch, tmpRepoTrackingBranch).
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return "", fmt.Errorf("merge-base: %w", err)
	}
	mergeBase = strings.TrimSpace(mergeBase)

	// Retrieve blob SHAs from each version: ancestor, base branch (ours), head branch (theirs).
	ancestorSHA, _ := getBlobSHAFromRef(ctx, tmpDir, mergeBase, filePath)
	ourSHA, err := getBlobSHAFromRef(ctx, tmpDir, tmpRepoBaseBranch, filePath)
	if err != nil {
		return "", fmt.Errorf("cannot locate %q in base branch: %w", filePath, err)
	}
	theirSHA, err := getBlobSHAFromRef(ctx, tmpDir, tmpRepoTrackingBranch, filePath)
	if err != nil {
		return "", fmt.Errorf("cannot locate %q in head branch: %w", filePath, err)
	}

	// Unpack blobs into temp files so git merge-file can operate on them.
	// "theirs" = head branch (what the user pushed), shown as <<<<<<< in output.
	theirFile, err := unpackBlob(ctx, tmpDir, theirSHA)
	if err != nil {
		return "", fmt.Errorf("unpack head blob: %w", err)
	}
	defer util.Remove(theirFile)

	var ancestorFile string
	if ancestorSHA != "" {
		ancestorFile, err = unpackBlob(ctx, tmpDir, ancestorSHA)
		if err != nil {
			return "", fmt.Errorf("unpack ancestor blob: %w", err)
		}
		defer util.Remove(ancestorFile)
	} else {
		// File was added independently on both branches; use an empty ancestor.
		f, ferr := os.CreateTemp(tmpDir, "empty-ancestor-*")
		if ferr != nil {
			return "", fmt.Errorf("create empty ancestor file: %w", ferr)
		}
		f.Close()
		ancestorFile = f.Name()
		defer util.Remove(ancestorFile)
	}

	ourFile, err := unpackBlob(ctx, tmpDir, ourSHA)
	if err != nil {
		return "", fmt.Errorf("unpack base blob: %w", err)
	}
	defer util.Remove(ourFile)

	// git merge-file <current> <base/ancestor> <other>
	// current = head branch (theirFile) -> labeled HeadBranch in <<<<<<< line
	// other   = base branch (ourFile)   -> labeled BaseBranch in >>>>>>> line
	// A non-zero exit code is expected when there are unresolvable conflicts.
	_ = gitcmd.NewCommand("merge-file", "-L").
		AddDynamicArguments(pr.HeadBranch).
		AddArguments("-L", "base", "-L").
		AddDynamicArguments(pr.BaseBranch).
		AddDynamicArguments(theirFile, ancestorFile, ourFile).
		WithDir(tmpDir).RunWithStderr(ctx)

	content, err := os.ReadFile(theirFile)
	if err != nil {
		return "", fmt.Errorf("read merged file: %w", err)
	}
	return string(content), nil
}

// ResolvedFile holds the path and resolved content for a conflicted file.
type ResolvedFile struct {
	Path    string
	Content string
}

// CommitConflictResolution creates a proper merge commit on the head branch
// that incorporates the resolved file contents. Using git plumbing (read-tree,
// update-index, write-tree, commit-tree) means we never need a working-tree
// checkout in the temp repo. The commit has TWO parents – the old head tip and
// the current base branch tip – exactly like a "merge base into head" commit.
// With this ancestry, the PR conflict check will find the base tip as a
// reachable ancestor of head and report no conflicts.
func CommitConflictResolution(
	ctx context.Context,
	pr *issues_model.PullRequest,
	doer *user_model.User,
	resolvedFiles []ResolvedFile,
	commitMsg string,
	commitEnv []string, // GIT_AUTHOR_*/GIT_COMMITTER_* already set by caller
) error {
	prCtx, cancel, err := createTemporaryRepoForPR(ctx, pr)
	if err != nil {
		return fmt.Errorf("createTemporaryRepoForPR: %w", err)
	}
	defer cancel()

	tmpDir := prCtx.tmpBasePath

	// 1. Load the tracking-branch tree into the index (no checkout needed).
	if err := gitcmd.NewCommand("read-tree").
		AddDynamicArguments(tmpRepoTrackingBranch).
		WithDir(tmpDir).RunWithStderr(ctx); err != nil {
		return fmt.Errorf("read-tree: %w", err)
	}

	// 2. Write each resolved file as a blob and stage it.
	for _, f := range resolvedFiles {
		content := strings.ReplaceAll(f.Content, "\r", "")

		// Write blob object to the object store.
		blobHash, _, err := gitcmd.NewCommand("hash-object", "-w", "--stdin").
			WithStdinBytes([]byte(content)).
			WithDir(tmpDir).RunStdString(ctx)
		if err != nil {
			return fmt.Errorf("hash-object %s: %w", f.Path, err)
		}
		blobHash = strings.TrimSpace(blobHash)

		// Preserve the original file mode (default to 100644).
		mode := "100644"
		if out, _, _ := gitcmd.NewCommand("ls-tree").
			AddDynamicArguments(tmpRepoTrackingBranch, f.Path).
			WithDir(tmpDir).RunStdString(ctx); out != "" {
			if fields := strings.Fields(out); len(fields) > 0 {
				mode = fields[0]
			}
		}

		// Stage the new blob (cacheinfo format: mode,hash,path).
		if err := gitcmd.NewCommand("update-index", "--cacheinfo").
			AddDynamicArguments(mode + "," + blobHash + "," + f.Path).
			WithDir(tmpDir).RunWithStderr(ctx); err != nil {
			return fmt.Errorf("update-index %s: %w", f.Path, err)
		}
	}

	// 3. Build the new tree object from the updated index.
	newTreeHash, _, err := gitcmd.NewCommand("write-tree").
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("write-tree: %w", err)
	}
	newTreeHash = strings.TrimSpace(newTreeHash)

	// 4. Resolve parent commit SHAs.
	trackingHash, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(tmpRepoTrackingBranch).
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("rev-parse tracking: %w", err)
	}
	trackingHash = strings.TrimSpace(trackingHash)

	baseHash, _, err := gitcmd.NewCommand("rev-parse").
		AddDynamicArguments(tmpRepoBaseBranch).
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("rev-parse base: %w", err)
	}
	baseHash = strings.TrimSpace(baseHash)

	// 5. Create the merge commit: parent1 = old head tip, parent2 = base tip.
	newCommitHash, _, err := gitcmd.NewCommand("commit-tree").
		AddDynamicArguments(newTreeHash).
		AddArguments("-p").AddDynamicArguments(trackingHash).
		AddArguments("-p").AddDynamicArguments(baseHash).
		AddArguments("-m").AddDynamicArguments(commitMsg).
		WithEnv(commitEnv).
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("commit-tree: %w", err)
	}
	newCommitHash = strings.TrimSpace(newCommitHash)

	// 6. Push the new commit to the actual head repository.
	// Use InternalPushingEnvironment to bypass server-side git hooks; the
	// caller (ResolveConflictsBatchPost) explicitly triggers StartPullRequestCheckImmediately
	// which queues the re-check, making the hook's AddTestPullRequestTask redundant.
	pushEnv := repo_module.InternalPushingEnvironment(doer, pr.HeadRepo)
	if err := gitrepo.PushFromLocal(ctx, tmpDir, pr.HeadRepo, git.PushOptions{
		Branch: newCommitHash + ":" + git.BranchPrefix + pr.HeadBranch,
		Env:    pushEnv,
	}); err != nil {
		return fmt.Errorf("push to head repo: %w", err)
	}

	return nil
}

// getBlobSHAFromRef returns the blob object SHA for filePath at the given git ref.
func getBlobSHAFromRef(ctx context.Context, tmpDir, ref, filePath string) (string, error) {
	out, _, err := gitcmd.NewCommand("ls-tree", "--").
		AddDynamicArguments(ref, filePath).
		WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return "", err
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

// unpackBlob extracts a git blob object to a temporary file and returns its path.
func unpackBlob(ctx context.Context, tmpDir, sha string) (string, error) {
	out, _, err := gitcmd.NewCommand("unpack-file").AddDynamicArguments(sha).WithDir(tmpDir).RunStdString(ctx)
	if err != nil {
		return "", fmt.Errorf("unpack-file %s: %w", sha, err)
	}
	return filepath.Join(tmpDir, strings.TrimSpace(out)), nil
}
