// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	pull_service "code.gitea.io/gitea/services/pull"
)

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	GitUserName  string // to match "git config user.name"
	GitUserEmail string // to match "git config user.email"
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	Author    time.Time
	Committer time.Time
}

type ChangeRepoFile struct {
	Operation     string
	TreePath      string
	FromTreePath  string
	ContentReader io.ReadSeeker
	SHA           string
	Options       *RepoFileOptions
}

// ChangeRepoFilesOptions holds the repository files update options
type ChangeRepoFilesOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	Message      string
	Files        []*ChangeRepoFile
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
	Signoff      bool
}

type RepoFileOptions struct {
	treePath     string
	fromTreePath string
	executable   bool
}

// ErrRepoFileDoesNotExist represents a "RepoFileDoesNotExist" kind of error.
type ErrRepoFileDoesNotExist struct {
	Path string
	Name string
}

// IsErrRepoFileDoesNotExist checks if an error is a ErrRepoDoesNotExist.
func IsErrRepoFileDoesNotExist(err error) bool {
	_, ok := err.(ErrRepoFileDoesNotExist)
	return ok
}

func (err ErrRepoFileDoesNotExist) Error() string {
	return fmt.Sprintf("repository file does not exist [path: %s]", err.Path)
}

func (err ErrRepoFileDoesNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ChangeRepoFiles adds, updates or removes multiple files in the given repository
func ChangeRepoFiles(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *ChangeRepoFilesOptions) (*structs.FilesResponse, error) {
	err := repo.MustNotBeArchived()
	if err != nil {
		return nil, err
	}

	// If no branch name is set, assume default branch
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// oldBranch must exist for this operation
	if _, err := gitRepo.GetBranch(opts.OldBranch); err != nil && !repo.IsEmpty {
		return nil, err
	}

	var treePaths []string
	for _, file := range opts.Files {
		// If FromTreePath is not set, set it to the opts.TreePath
		if file.TreePath != "" && file.FromTreePath == "" {
			file.FromTreePath = file.TreePath
		}

		// Check that the path given in opts.treePath is valid (not a git path)
		treePath := CleanUploadFileName(file.TreePath)
		if treePath == "" {
			return nil, ErrFilenameInvalid{
				Path: file.TreePath,
			}
		}
		// If there is a fromTreePath (we are copying it), also clean it up
		fromTreePath := CleanUploadFileName(file.FromTreePath)
		if fromTreePath == "" && file.FromTreePath != "" {
			return nil, ErrFilenameInvalid{
				Path: file.FromTreePath,
			}
		}

		file.Options = &RepoFileOptions{
			treePath:     treePath,
			fromTreePath: fromTreePath,
			executable:   false,
		}
		treePaths = append(treePaths, treePath)
	}

	// A NewBranch can be specified for the file to be created/updated in a new branch.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		existingBranch, err := gitRepo.GetBranch(opts.NewBranch)
		if existingBranch != nil {
			return nil, git_model.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
		if err != nil && !git.IsErrBranchNotExist(err) {
			return nil, err
		}
	} else if err := VerifyBranchProtection(ctx, repo, doer, opts.OldBranch, treePaths); err != nil {
		return nil, err
	}

	message := strings.TrimSpace(opts.Message)

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		log.Error("NewTemporaryUploadRepository failed: %v", err)
	}
	defer t.Close()
	hasOldBranch := true
	if err := t.Clone(ctx, opts.OldBranch, true); err != nil {
		for _, file := range opts.Files {
			if file.Operation == "delete" {
				return nil, err
			}
		}
		if !git.IsErrBranchNotExist(err) || !repo.IsEmpty {
			return nil, err
		}
		if err := t.Init(ctx, repo.ObjectFormatName); err != nil {
			return nil, err
		}
		hasOldBranch = false
		opts.LastCommitID = ""
	}
	if hasOldBranch {
		if err := t.SetDefaultIndex(ctx); err != nil {
			return nil, err
		}
	}

	for _, file := range opts.Files {
		if file.Operation == "delete" {
			// Get the files in the index
			filesInIndex, err := t.LsFiles(ctx, file.TreePath)
			if err != nil {
				return nil, fmt.Errorf("DeleteRepoFile: %w", err)
			}

			// Find the file we want to delete in the index
			inFilelist := false
			for _, indexFile := range filesInIndex {
				if indexFile == file.TreePath {
					inFilelist = true
					break
				}
			}
			if !inFilelist {
				return nil, ErrRepoFileDoesNotExist{
					Path: file.TreePath,
				}
			}
		}
	}

	if hasOldBranch {
		// Get the commit of the original branch
		commit, err := t.GetBranchCommit(opts.OldBranch)
		if err != nil {
			return nil, err // Couldn't get a commit for the branch
		}

		// Assigned LastCommitID in opts if it hasn't been set
		if opts.LastCommitID == "" {
			opts.LastCommitID = commit.ID.String()
		} else {
			lastCommitID, err := t.gitRepo.ConvertToGitID(opts.LastCommitID)
			if err != nil {
				return nil, fmt.Errorf("ConvertToSHA1: Invalid last commit ID: %w", err)
			}
			opts.LastCommitID = lastCommitID.String()
		}

		for _, file := range opts.Files {
			if err := handleCheckErrors(file, commit, opts); err != nil {
				return nil, err
			}
		}
	}

	contentStore := lfs.NewContentStore()
	for _, file := range opts.Files {
		switch file.Operation {
		case "create", "update":
			if err := CreateOrUpdateFile(ctx, t, file, contentStore, repo.ID, hasOldBranch); err != nil {
				return nil, err
			}
		case "delete":
			// Remove the file from the index
			if err := t.RemoveFilesFromIndex(ctx, file.TreePath); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid file operation: %s %s, supported operations are create, update, delete", file.Operation, file.Options.treePath)
		}
	}

	// Now write the tree
	treeHash, err := t.WriteTree(ctx)
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	commitOpts := &CommitTreeUserOptions{
		ParentCommitID:    opts.LastCommitID,
		TreeHash:          treeHash,
		CommitMessage:     message,
		SignOff:           opts.Signoff,
		DoerUser:          doer,
		AuthorIdentity:    opts.Author,
		AuthorTime:        nil,
		CommitterIdentity: opts.Committer,
		CommitterTime:     nil,
	}
	if opts.Dates != nil {
		commitOpts.AuthorTime, commitOpts.CommitterTime = &opts.Dates.Author, &opts.Dates.Committer
	}
	commitHash, err := t.CommitTree(ctx, commitOpts)
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(ctx, doer, commitHash, opts.NewBranch); err != nil {
		log.Error("%T %v", err, err)
		return nil, err
	}

	commit, err := t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	filesResponse, err := GetFilesResponseFromCommit(ctx, repo, commit, opts.NewBranch, treePaths)
	if err != nil {
		return nil, err
	}

	if repo.IsEmpty {
		if isEmpty, err := gitRepo.IsEmpty(); err == nil && !isEmpty {
			_ = repo_model.UpdateRepositoryCols(ctx, &repo_model.Repository{ID: repo.ID, IsEmpty: false, DefaultBranch: opts.NewBranch}, "is_empty", "default_branch")
		}
	}

	return filesResponse, nil
}

// ErrRepoFileAlreadyExists represents a "RepoFileAlreadyExist" kind of error.
type ErrRepoFileAlreadyExists struct {
	Path string
}

// IsErrRepoFileAlreadyExists checks if an error is a ErrRepoFileAlreadyExists.
func IsErrRepoFileAlreadyExists(err error) bool {
	_, ok := err.(ErrRepoFileAlreadyExists)
	return ok
}

func (err ErrRepoFileAlreadyExists) Error() string {
	return fmt.Sprintf("repository file already exists [path: %s]", err.Path)
}

func (err ErrRepoFileAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrFilePathInvalid represents a "FilePathInvalid" kind of error.
type ErrFilePathInvalid struct {
	Message string
	Path    string
	Name    string
	Type    git.EntryMode
}

// IsErrFilePathInvalid checks if an error is an ErrFilePathInvalid.
func IsErrFilePathInvalid(err error) bool {
	_, ok := err.(ErrFilePathInvalid)
	return ok
}

func (err ErrFilePathInvalid) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("path is invalid [path: %s]", err.Path)
}

func (err ErrFilePathInvalid) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrSHAOrCommitIDNotProvided represents a "SHAOrCommitIDNotProvided" kind of error.
type ErrSHAOrCommitIDNotProvided struct{}

// IsErrSHAOrCommitIDNotProvided checks if an error is a ErrSHAOrCommitIDNotProvided.
func IsErrSHAOrCommitIDNotProvided(err error) bool {
	_, ok := err.(ErrSHAOrCommitIDNotProvided)
	return ok
}

func (err ErrSHAOrCommitIDNotProvided) Error() string {
	return "a SHA or commit ID must be proved when updating a file"
}

// handles the check for various issues for ChangeRepoFiles
func handleCheckErrors(file *ChangeRepoFile, commit *git.Commit, opts *ChangeRepoFilesOptions) error {
	if file.Operation == "update" || file.Operation == "delete" {
		fromEntry, err := commit.GetTreeEntryByPath(file.Options.fromTreePath)
		if err != nil {
			return err
		}
		if file.SHA != "" {
			// If a SHA was given and the SHA given doesn't match the SHA of the fromTreePath, throw error
			if file.SHA != fromEntry.ID.String() {
				return pull_service.ErrSHADoesNotMatch{
					Path:       file.Options.treePath,
					GivenSHA:   file.SHA,
					CurrentSHA: fromEntry.ID.String(),
				}
			}
		} else if opts.LastCommitID != "" {
			// If a lastCommitID was given and it doesn't match the commitID of the head of the branch throw
			// an error, but only if we aren't creating a new branch.
			if commit.ID.String() != opts.LastCommitID && opts.OldBranch == opts.NewBranch {
				if changed, err := commit.FileChangedSinceCommit(file.Options.treePath, opts.LastCommitID); err != nil {
					return err
				} else if changed {
					return ErrCommitIDDoesNotMatch{
						GivenCommitID:   opts.LastCommitID,
						CurrentCommitID: opts.LastCommitID,
					}
				}
				// The file wasn't modified, so we are good to delete it
			}
		} else {
			// When updating a file, a lastCommitID or SHA needs to be given to make sure other commits
			// haven't been made. We throw an error if one wasn't provided.
			return ErrSHAOrCommitIDNotProvided{}
		}
		file.Options.executable = fromEntry.IsExecutable()
	}
	if file.Operation == "create" || file.Operation == "update" {
		// For the path where this file will be created/updated, we need to make
		// sure no parts of the path are existing files or links except for the last
		// item in the path which is the file name, and that shouldn't exist IF it is
		// a new file OR is being moved to a new path.
		treePathParts := strings.Split(file.Options.treePath, "/")
		subTreePath := ""
		for index, part := range treePathParts {
			subTreePath = path.Join(subTreePath, part)
			entry, err := commit.GetTreeEntryByPath(subTreePath)
			if err != nil {
				if git.IsErrNotExist(err) {
					// Means there is no item with that name, so we're good
					break
				}
				return err
			}
			if index < len(treePathParts)-1 {
				if !entry.IsDir() {
					return ErrFilePathInvalid{
						Message: fmt.Sprintf("a file exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
						Path:    subTreePath,
						Name:    part,
						Type:    git.EntryModeBlob,
					}
				}
			} else if entry.IsLink() {
				return ErrFilePathInvalid{
					Message: fmt.Sprintf("a symbolic link exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
					Path:    subTreePath,
					Name:    part,
					Type:    git.EntryModeSymlink,
				}
			} else if entry.IsDir() {
				return ErrFilePathInvalid{
					Message: fmt.Sprintf("a directory exists where you’re trying to create a file [path: %s]", subTreePath),
					Path:    subTreePath,
					Name:    part,
					Type:    git.EntryModeTree,
				}
			} else if file.Options.fromTreePath != file.Options.treePath || file.Operation == "create" {
				// The entry shouldn't exist if we are creating new file or moving to a new path
				return ErrRepoFileAlreadyExists{
					Path: file.Options.treePath,
				}
			}
		}
	}

	return nil
}

// CreateOrUpdateFile handles creating or updating a file for ChangeRepoFiles
func CreateOrUpdateFile(ctx context.Context, t *TemporaryUploadRepository, file *ChangeRepoFile, contentStore *lfs.ContentStore, repoID int64, hasOldBranch bool) error {
	// Get the two paths (might be the same if not moving) from the index if they exist
	filesInIndex, err := t.LsFiles(ctx, file.TreePath, file.FromTreePath)
	if err != nil {
		return fmt.Errorf("UpdateRepoFile: %w", err)
	}
	// If is a new file (not updating) then the given path shouldn't exist
	if file.Operation == "create" {
		for _, indexFile := range filesInIndex {
			if indexFile == file.TreePath {
				return ErrRepoFileAlreadyExists{
					Path: file.TreePath,
				}
			}
		}
	}

	// Remove the old path from the tree
	if file.Options.fromTreePath != file.Options.treePath && len(filesInIndex) > 0 {
		for _, indexFile := range filesInIndex {
			if indexFile == file.Options.fromTreePath {
				if err := t.RemoveFilesFromIndex(ctx, file.FromTreePath); err != nil {
					return err
				}
			}
		}
	}

	treeObjectContentReader := file.ContentReader
	var lfsMetaObject *git_model.LFSMetaObject
	if setting.LFS.StartServer && hasOldBranch {
		// Check there is no way this can return multiple infos
		filename2attribute2info, err := t.gitRepo.CheckAttribute(git.CheckAttributeOpts{
			Attributes: []string{"filter"},
			Filenames:  []string{file.Options.treePath},
			CachedOnly: true,
		})
		if err != nil {
			return err
		}

		if filename2attribute2info[file.Options.treePath] != nil && filename2attribute2info[file.Options.treePath]["filter"] == "lfs" {
			// OK so we are supposed to LFS this data!
			pointer, err := lfs.GeneratePointer(treeObjectContentReader)
			if err != nil {
				return err
			}
			lfsMetaObject = &git_model.LFSMetaObject{Pointer: pointer, RepositoryID: repoID}
			treeObjectContentReader = strings.NewReader(pointer.StringContent())
		}
	}

	// Add the object to the database
	objectHash, err := t.HashObject(ctx, treeObjectContentReader)
	if err != nil {
		return err
	}

	// Add the object to the index
	if file.Options.executable {
		if err := t.AddObjectToIndex(ctx, "100755", objectHash, file.Options.treePath); err != nil {
			return err
		}
	} else {
		if err := t.AddObjectToIndex(ctx, "100644", objectHash, file.Options.treePath); err != nil {
			return err
		}
	}

	if lfsMetaObject != nil {
		// We have an LFS object - create it
		lfsMetaObject, err = git_model.NewLFSMetaObject(ctx, lfsMetaObject.RepositoryID, lfsMetaObject.Pointer)
		if err != nil {
			return err
		}
		exist, err := contentStore.Exists(lfsMetaObject.Pointer)
		if err != nil {
			return err
		}
		if !exist {
			_, err := file.ContentReader.Seek(0, io.SeekStart)
			if err != nil {
				return err
			}
			if err := contentStore.Put(lfsMetaObject.Pointer, file.ContentReader); err != nil {
				if _, err2 := git_model.RemoveLFSMetaObjectByOid(ctx, repoID, lfsMetaObject.Oid); err2 != nil {
					return fmt.Errorf("unable to remove failed inserted LFS object %s: %v (Prev Error: %w)", lfsMetaObject.Oid, err2, err)
				}
				return err
			}
		}
	}

	return nil
}

// VerifyBranchProtection verify the branch protection for modifying the given treePath on the given branch
func VerifyBranchProtection(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, branchName string, treePaths []string) error {
	protectedBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, branchName)
	if err != nil {
		return err
	}
	if protectedBranch != nil {
		protectedBranch.Repo = repo
		globUnprotected := protectedBranch.GetUnprotectedFilePatterns()
		globProtected := protectedBranch.GetProtectedFilePatterns()
		canUserPush := protectedBranch.CanUserPush(ctx, doer)
		for _, treePath := range treePaths {
			isUnprotectedFile := false
			if len(globUnprotected) != 0 {
				isUnprotectedFile = protectedBranch.IsUnprotectedFile(globUnprotected, treePath)
			}
			if !canUserPush && !isUnprotectedFile {
				return ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
			if protectedBranch.IsProtectedFile(globProtected, treePath) {
				return pull_service.ErrFilePathProtected{
					Path: treePath,
				}
			}
		}
		if protectedBranch.RequireSignedCommits {
			_, _, _, err := asymkey_service.SignCRUDAction(ctx, repo.RepoPath(), doer, repo.RepoPath(), branchName)
			if err != nil {
				if !asymkey_service.IsErrWontSign(err) {
					return err
				}
				return ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
		}
	}
	return nil
}
