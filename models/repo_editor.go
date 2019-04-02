// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
)

// ___________    .___.__  __    ___________.__.__
// \_   _____/  __| _/|__|/  |_  \_   _____/|__|  |   ____
//  |    __)_  / __ | |  \   __\  |    __)  |  |  | _/ __ \
//  |        \/ /_/ | |  ||  |    |     \   |  |  |_\  ___/
// /_______  /\____ | |__||__|    \___  /   |__|____/\___  >
//         \/      \/                 \/                 \/

// discardLocalRepoBranchChanges discards local commits/changes of
// given branch to make sure it is even to remote branch.
func discardLocalRepoBranchChanges(localPath, branch string) error {
	if !com.IsExist(localPath) {
		return nil
	}
	// No need to check if nothing in the repository.
	if !git.IsBranchExist(localPath, branch) {
		return nil
	}

	refName := "origin/" + branch
	if err := git.ResetHEAD(localPath, true, refName); err != nil {
		return fmt.Errorf("git reset --hard %s: %v", refName, err)
	}
	return nil
}

// DiscardLocalRepoBranchChanges discards the local repository branch changes
func (repo *Repository) DiscardLocalRepoBranchChanges(branch string) error {
	return discardLocalRepoBranchChanges(repo.LocalCopyPath(), branch)
}

// checkoutNewBranch checks out to a new branch from the a branch name.
func checkoutNewBranch(repoPath, localPath, oldBranch, newBranch string) error {
	if err := git.Checkout(localPath, git.CheckoutOptions{
		Timeout:   time.Duration(setting.Git.Timeout.Pull) * time.Second,
		Branch:    newBranch,
		OldBranch: oldBranch,
	}); err != nil {
		return fmt.Errorf("git checkout -b %s %s: %v", newBranch, oldBranch, err)
	}
	return nil
}

// CheckoutNewBranch checks out a new branch
func (repo *Repository) CheckoutNewBranch(oldBranch, newBranch string) error {
	return checkoutNewBranch(repo.RepoPath(), repo.LocalCopyPath(), oldBranch, newBranch)
}

// UpdateRepoFileOptions holds the repository file update options
type UpdateRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	OldTreeName  string
	NewTreeName  string
	Message      string
	Content      string
	IsNewFile    bool
}

// UpdateRepoFile adds or updates a file in repository.
func (repo *Repository) UpdateRepoFile(doer *User, opts UpdateRepoFileOptions) (event *CommitRepoEvent, err error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.DiscardLocalRepoBranchChanges(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("DiscardLocalRepoBranchChanges [branch: %s]: %v", opts.OldBranch, err)
	} else if err = repo.UpdateLocalCopyBranch(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("UpdateLocalCopyBranch [branch: %s]: %v", opts.OldBranch, err)
	}

	if opts.OldBranch != opts.NewBranch {
		if err := repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
			return nil, fmt.Errorf("CheckoutNewBranch [old_branch: %s, new_branch: %s]: %v", opts.OldBranch, opts.NewBranch, err)
		}
	}

	localPath := repo.LocalCopyPath()
	oldFilePath := path.Join(localPath, opts.OldTreeName)
	filePath := path.Join(localPath, opts.NewTreeName)
	dir := path.Dir(filePath)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	// If it's meant to be a new file, make sure it doesn't exist.
	if opts.IsNewFile {
		if com.IsExist(filePath) {
			return nil, ErrRepoFileAlreadyExist{filePath}
		}
	}

	// Ignore move step if it's a new file under a directory.
	// Otherwise, move the file when name changed.
	if com.IsFile(oldFilePath) && opts.OldTreeName != opts.NewTreeName {
		if err = git.MoveFile(localPath, opts.OldTreeName, opts.NewTreeName); err != nil {
			return nil, fmt.Errorf("git mv %s %s: %v", opts.OldTreeName, opts.NewTreeName, err)
		}
	}

	if err = ioutil.WriteFile(filePath, []byte(opts.Content), 0666); err != nil {
		return nil, fmt.Errorf("WriteFile: %v", err)
	}

	if err = git.AddChanges(localPath, true); err != nil {
		return nil, fmt.Errorf("git add --all: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   opts.Message,
	}); err != nil {
		return nil, fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: opts.NewBranch,
	}); err != nil {
		return nil, fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil, nil
	}
	commit, err := gitRepo.GetBranchCommit(opts.NewBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit [branch: %s]: %v", opts.NewBranch, err)
		return nil, nil
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return nil, fmt.Errorf("GetOwner: %v", err)
	}
	event, err = PushUpdate(
		opts.NewBranch,
		PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commit.ID.String(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("PushUpdate: %v", err)
	}
	UpdateRepoIndexer(repo)

	return event, nil
}

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func (repo *Repository) GetDiffPreview(branch, treePath, content string) (diff *Diff, err error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.DiscardLocalRepoBranchChanges(branch); err != nil {
		return nil, fmt.Errorf("DiscardLocalRepoBranchChanges [branch: %s]: %v", branch, err)
	} else if err = repo.UpdateLocalCopyBranch(branch); err != nil {
		return nil, fmt.Errorf("UpdateLocalCopyBranch [branch: %s]: %v", branch, err)
	}

	localPath := repo.LocalCopyPath()
	filePath := path.Join(localPath, treePath)
	dir := filepath.Dir(filePath)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = ioutil.WriteFile(filePath, []byte(content), 0666); err != nil {
		return nil, fmt.Errorf("WriteFile: %v", err)
	}

	cmd := exec.Command("git", "diff", treePath)
	cmd.Dir = localPath
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetDiffPreview [repo_path: %s]", repo.RepoPath()), cmd)
	defer process.GetManager().Remove(pid)

	diff, err = ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return diff, nil
}

// ________         .__          __           ___________.__.__
// \______ \   ____ |  |   _____/  |_  ____   \_   _____/|__|  |   ____
//  |    |  \_/ __ \|  | _/ __ \   __\/ __ \   |    __)  |  |  | _/ __ \
//  |    `   \  ___/|  |_\  ___/|  | \  ___/   |     \   |  |  |_\  ___/
// /_______  /\___  >____/\___  >__|  \___  >  \___  /   |__|____/\___  >
//         \/     \/          \/          \/       \/                 \/
//

// DeleteRepoFileOptions holds the repository delete file options
type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
}

// DeleteRepoFile deletes a repository file
func (repo *Repository) DeleteRepoFile(doer *User, opts DeleteRepoFileOptions) (event *CommitRepoEvent, err error) {
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.DiscardLocalRepoBranchChanges(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("DiscardLocalRepoBranchChanges [branch: %s]: %v", opts.OldBranch, err)
	} else if err = repo.UpdateLocalCopyBranch(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("UpdateLocalCopyBranch [branch: %s]: %v", opts.OldBranch, err)
	}

	if opts.OldBranch != opts.NewBranch {
		if err := repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
			return nil, fmt.Errorf("CheckoutNewBranch [old_branch: %s, new_branch: %s]: %v", opts.OldBranch, opts.NewBranch, err)
		}
	}

	localPath := repo.LocalCopyPath()
	if err = os.Remove(path.Join(localPath, opts.TreePath)); err != nil {
		return nil, fmt.Errorf("Remove: %v", err)
	}

	if err = git.AddChanges(localPath, true); err != nil {
		return nil, fmt.Errorf("git add --all: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   opts.Message,
	}); err != nil {
		return nil, fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: opts.NewBranch,
	}); err != nil {
		return nil, fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil, nil
	}
	commit, err := gitRepo.GetBranchCommit(opts.NewBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit [branch: %s]: %v", opts.NewBranch, err)
		return nil, nil
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return nil, fmt.Errorf("GetOwner: %v", err)
	}
	event, err = PushUpdate(
		opts.NewBranch,
		PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commit.ID.String(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("PushUpdate: %v", err)
	}
	return event, nil
}

// UploadRepoFileOptions contains the uploaded repository file options
type UploadRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	Files        []string // In UUID format.
}

// UploadRepoFiles uploads files to a repository
func (repo *Repository) UploadRepoFiles(doer *User, opts UploadRepoFileOptions) (event *CommitRepoEvent, err error) {
	if len(opts.Files) == 0 {
		return nil, nil
	}

	uploads, err := GetUploadsByUUIDs(opts.Files)
	if err != nil {
		return nil, fmt.Errorf("GetUploadsByUUIDs [uuids: %v]: %v", opts.Files, err)
	}

	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.DiscardLocalRepoBranchChanges(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("DiscardLocalRepoBranchChanges [branch: %s]: %v", opts.OldBranch, err)
	} else if err = repo.UpdateLocalCopyBranch(opts.OldBranch); err != nil {
		return nil, fmt.Errorf("UpdateLocalCopyBranch [branch: %s]: %v", opts.OldBranch, err)
	}

	if opts.OldBranch != opts.NewBranch {
		if err = repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
			return nil, fmt.Errorf("CheckoutNewBranch [old_branch: %s, new_branch: %s]: %v", opts.OldBranch, opts.NewBranch, err)
		}
	}

	localPath := repo.LocalCopyPath()
	dirPath := path.Join(localPath, opts.TreePath)

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("Failed to create dir %s: %v", dirPath, err)
	}

	// Copy uploaded files into repository.
	for _, upload := range uploads {
		tmpPath := upload.LocalPath()
		targetPath := path.Join(dirPath, upload.Name)
		if !com.IsFile(tmpPath) {
			continue
		}

		if err = com.Copy(tmpPath, targetPath); err != nil {
			return nil, fmt.Errorf("Copy: %v", err)
		}
	}

	if err = git.AddChanges(localPath, true); err != nil {
		return nil, fmt.Errorf("git add --all: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   opts.Message,
	}); err != nil {
		return nil, fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: opts.NewBranch,
	}); err != nil {
		return nil, fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil, nil
	}
	commit, err := gitRepo.GetBranchCommit(opts.NewBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit [branch: %s]: %v", opts.NewBranch, err)
		return nil, nil
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return nil, fmt.Errorf("GetOwner: %v", err)
	}
	event, err = PushUpdate(
		opts.NewBranch,
		PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commit.ID.String(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("PushUpdate: %v", err)
	}

	return event, DeleteUploads(uploads...)
}
