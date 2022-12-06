// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"

	stdcharset "golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// Declare all files status
const (
	CreateFileAction = "create"
	EditFileAction   = "edit"
	DeleteFileAction = "delete"
)

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	Name  string
	Email string
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	Author    time.Time
	Committer time.Time
}

// UpdateRepoFileOptions holds the repository file update options
type UpdateRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	FromTreePath string
	Message      string
	Content      string
	SHA          string
	IsNewFile    bool
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
	Signoff      bool
	FileAction   string
}

type UpdateRepoFilesOptions struct {
	Files     []*UpdateRepoFileOptions `json:"files"`
	OldBranch string
	NewBranch string
	Author    *IdentityOptions
	Committer *IdentityOptions
	Dates     *CommitDateOptions
	Signoff   bool
	Message   string
}

type PushedFileRes struct {
	Name string `json:"name"`
	Path string `json:"path"`
	SHA  string `json:"sha"`
	// SHA256 string `json:"sha256"`
}

type PushedFilesRes struct {
	Commit *structs.FileCommitResponse `json:"commit"`
	Files  []*PushedFileRes            `json:"files"`
}

func detectEncodingAndBOM(entry *git.TreeEntry, repo *repo_model.Repository) (string, bool) {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		// return default
		return "UTF-8", false
	}
	defer reader.Close()
	buf := make([]byte, 1024)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		// return default
		return "UTF-8", false
	}
	buf = buf[:n]

	if setting.LFS.StartServer {
		pointer, _ := lfs.ReadPointerFromBuffer(buf)
		if pointer.IsValid() {
			meta, err := git_model.GetLFSMetaObjectByOid(repo.ID, pointer.Oid)
			if err != nil && err != git_model.ErrLFSObjectNotExist {
				// return default
				return "UTF-8", false
			}
			if meta != nil {
				dataRc, err := lfs.ReadMetaObject(pointer)
				if err != nil {
					// return default
					return "UTF-8", false
				}
				defer dataRc.Close()
				buf = make([]byte, 1024)
				n, err = util.ReadAtMost(dataRc, buf)
				if err != nil {
					// return default
					return "UTF-8", false
				}
				buf = buf[:n]
			}
		}
	}

	encoding, err := charset.DetectEncoding(buf)
	if err != nil {
		// just default to utf-8 and no bom
		return "UTF-8", false
	}
	if encoding == "UTF-8" {
		return encoding, bytes.Equal(buf[0:3], charset.UTF8BOM)
	}
	charsetEncoding, _ := stdcharset.Lookup(encoding)
	if charsetEncoding == nil {
		return "UTF-8", false
	}

	result, n, err := transform.String(charsetEncoding.NewDecoder(), string(buf))
	if err != nil {
		// return default
		return "UTF-8", false
	}

	if n > 2 {
		return encoding, bytes.Equal([]byte(result)[0:3], charset.UTF8BOM)
	}

	return encoding, false
}

// CreateOrUpdateRepoFile adds or updates a file in the given repository
func CreateOrUpdateRepoFile(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *UpdateRepoFileOptions) (*structs.FileResponse, error) {
	// If no branch name is set, assume default branch
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// oldBranch must exist for this operation
	if _, err := gitRepo.GetBranch(opts.OldBranch); err != nil && !repo.IsEmpty {
		return nil, err
	}

	// A NewBranch can be specified for the file to be created/updated in a new branch.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		existingBranch, err := gitRepo.GetBranch(opts.NewBranch)
		if existingBranch != nil {
			return nil, models.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
		if err != nil && !git.IsErrBranchNotExist(err) {
			return nil, err
		}
	} else if err := VerifyBranchProtection(ctx, repo, doer, opts.OldBranch, opts.TreePath); err != nil {
		return nil, err
	}

	// If FromTreePath is not set, set it to the opts.TreePath
	if opts.TreePath != "" && opts.FromTreePath == "" {
		opts.FromTreePath = opts.TreePath
	}

	// Check that the path given in opts.treePath is valid (not a git path)
	treePath := CleanUploadFileName(opts.TreePath)
	if treePath == "" {
		return nil, models.ErrFilenameInvalid{
			Path: opts.TreePath,
		}
	}
	// If there is a fromTreePath (we are copying it), also clean it up
	fromTreePath := CleanUploadFileName(opts.FromTreePath)
	if fromTreePath == "" && opts.FromTreePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: opts.FromTreePath,
		}
	}

	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(ctx, repo)
	if err != nil {
		log.Error("%v", err)
	}
	defer t.Close()
	hasOldBranch := true
	if err := t.Clone(opts.OldBranch); err != nil {
		if !git.IsErrBranchNotExist(err) || !repo.IsEmpty {
			return nil, err
		}
		if err := t.Init(); err != nil {
			return nil, err
		}
		hasOldBranch = false
		opts.LastCommitID = ""
	}
	if hasOldBranch {
		if err := t.SetDefaultIndex(); err != nil {
			return nil, err
		}
	}

	encoding := "UTF-8"
	bom := false
	executable := false

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
			lastCommitID, err := t.gitRepo.ConvertToSHA1(opts.LastCommitID)
			if err != nil {
				return nil, fmt.Errorf("ConvertToSHA1: Invalid last commit ID: %w", err)
			}
			opts.LastCommitID = lastCommitID.String()

		}

		if !opts.IsNewFile {
			fromEntry, err := commit.GetTreeEntryByPath(fromTreePath)
			if err != nil {
				return nil, err
			}
			if opts.SHA != "" {
				// If a SHA was given and the SHA given doesn't match the SHA of the fromTreePath, throw error
				if opts.SHA != fromEntry.ID.String() {
					return nil, models.ErrSHADoesNotMatch{
						Path:       treePath,
						GivenSHA:   opts.SHA,
						CurrentSHA: fromEntry.ID.String(),
					}
				}
			} else if opts.LastCommitID != "" {
				// If a lastCommitID was given and it doesn't match the commitID of the head of the branch throw
				// an error, but only if we aren't creating a new branch.
				if commit.ID.String() != opts.LastCommitID && opts.OldBranch == opts.NewBranch {
					if changed, err := commit.FileChangedSinceCommit(treePath, opts.LastCommitID); err != nil {
						return nil, err
					} else if changed {
						return nil, models.ErrCommitIDDoesNotMatch{
							GivenCommitID:   opts.LastCommitID,
							CurrentCommitID: opts.LastCommitID,
						}
					}
					// The file wasn't modified, so we are good to delete it
				}
			} else {
				// When updating a file, a lastCommitID or SHA needs to be given to make sure other commits
				// haven't been made. We throw an error if one wasn't provided.
				return nil, models.ErrSHAOrCommitIDNotProvided{}
			}
			encoding, bom = detectEncodingAndBOM(fromEntry, repo)
			executable = fromEntry.IsExecutable()
		}

		// For the path where this file will be created/updated, we need to make
		// sure no parts of the path are existing files or links except for the last
		// item in the path which is the file name, and that shouldn't exist IF it is
		// a new file OR is being moved to a new path.
		treePathParts := strings.Split(treePath, "/")
		subTreePath := ""
		for index, part := range treePathParts {
			subTreePath = path.Join(subTreePath, part)
			entry, err := commit.GetTreeEntryByPath(subTreePath)
			if err != nil {
				if git.IsErrNotExist(err) {
					// Means there is no item with that name, so we're good
					break
				}
				return nil, err
			}
			if index < len(treePathParts)-1 {
				if !entry.IsDir() {
					return nil, models.ErrFilePathInvalid{
						Message: fmt.Sprintf("a file exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
						Path:    subTreePath,
						Name:    part,
						Type:    git.EntryModeBlob,
					}
				}
			} else if entry.IsLink() {
				return nil, models.ErrFilePathInvalid{
					Message: fmt.Sprintf("a symbolic link exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
					Path:    subTreePath,
					Name:    part,
					Type:    git.EntryModeSymlink,
				}
			} else if entry.IsDir() {
				return nil, models.ErrFilePathInvalid{
					Message: fmt.Sprintf("a directory exists where you’re trying to create a file [path: %s]", subTreePath),
					Path:    subTreePath,
					Name:    part,
					Type:    git.EntryModeTree,
				}
			} else if fromTreePath != treePath || opts.IsNewFile {
				// The entry shouldn't exist if we are creating new file or moving to a new path
				return nil, models.ErrRepoFileAlreadyExists{
					Path: treePath,
				}
			}

		}
	}

	// Get the two paths (might be the same if not moving) from the index if they exist
	filesInIndex, err := t.LsFiles(opts.TreePath, opts.FromTreePath)
	if err != nil {
		return nil, fmt.Errorf("UpdateRepoFile: %w", err)
	}
	// If is a new file (not updating) then the given path shouldn't exist
	if opts.IsNewFile {
		for _, file := range filesInIndex {
			if file == opts.TreePath {
				return nil, models.ErrRepoFileAlreadyExists{
					Path: opts.TreePath,
				}
			}
		}
	}

	// Remove the old path from the tree
	if fromTreePath != treePath && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == fromTreePath {
				if err := t.RemoveFilesFromIndex(opts.FromTreePath); err != nil {
					return nil, err
				}
			}
		}
	}

	content := opts.Content
	if bom {
		content = string(charset.UTF8BOM) + content
	}
	if encoding != "UTF-8" {
		charsetEncoding, _ := stdcharset.Lookup(encoding)
		if charsetEncoding != nil {
			result, _, err := transform.String(charsetEncoding.NewEncoder(), content)
			if err != nil {
				// Look if we can't encode back in to the original we should just stick with utf-8
				log.Error("Error re-encoding %s (%s) as %s - will stay as UTF-8: %v", opts.TreePath, opts.FromTreePath, encoding, err)
				result = content
			}
			content = result
		} else {
			log.Error("Unknown encoding: %s", encoding)
		}
	}
	// Reset the opts.Content to our adjusted content to ensure that LFS gets the correct content
	opts.Content = content
	var lfsMetaObject *git_model.LFSMetaObject

	if setting.LFS.StartServer && hasOldBranch {
		// Check there is no way this can return multiple infos
		filename2attribute2info, err := t.gitRepo.CheckAttribute(git.CheckAttributeOpts{
			Attributes: []git.CmdArg{"filter"},
			Filenames:  []string{treePath},
			CachedOnly: true,
		})
		if err != nil {
			return nil, err
		}

		if filename2attribute2info[treePath] != nil && filename2attribute2info[treePath]["filter"] == "lfs" {
			// OK so we are supposed to LFS this data!
			pointer, err := lfs.GeneratePointer(strings.NewReader(opts.Content))
			if err != nil {
				return nil, err
			}
			lfsMetaObject = &git_model.LFSMetaObject{Pointer: pointer, RepositoryID: repo.ID}
			content = pointer.StringContent()
		}
	}
	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Add the object to the index
	if executable {
		if err := t.AddObjectToIndex("100755", objectHash, treePath); err != nil {
			return nil, err
		}
	} else {
		if err := t.AddObjectToIndex("100644", objectHash, treePath); err != nil {
			return nil, err
		}
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate(opts.LastCommitID, author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree(opts.LastCommitID, author, committer, treeHash, message, opts.Signoff)
	}
	if err != nil {
		return nil, err
	}

	if lfsMetaObject != nil {
		// We have an LFS object - create it
		lfsMetaObject, err = git_model.NewLFSMetaObject(lfsMetaObject)
		if err != nil {
			return nil, err
		}
		contentStore := lfs.NewContentStore()
		exist, err := contentStore.Exists(lfsMetaObject.Pointer)
		if err != nil {
			return nil, err
		}
		if !exist {
			if err := contentStore.Put(lfsMetaObject.Pointer, strings.NewReader(opts.Content)); err != nil {
				if _, err2 := git_model.RemoveLFSMetaObjectByOid(repo.ID, lfsMetaObject.Oid); err2 != nil {
					return nil, fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %w)", lfsMetaObject.Oid, err2, err)
				}
				return nil, err
			}
		}
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		log.Error("%T %v", err, err)
		return nil, err
	}

	commit, err := t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	file, err := GetFileResponseFromCommit(ctx, repo, commit, opts.NewBranch, treePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// VerifyBranchProtection verify the branch protection for modifying the given treePath on the given branch
func VerifyBranchProtection(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, branchName, treePath string) error {
	protectedBranch, err := git_model.GetProtectedBranchBy(ctx, repo.ID, branchName)
	if err != nil {
		return err
	}
	if protectedBranch != nil {
		isUnprotectedFile := false
		glob := protectedBranch.GetUnprotectedFilePatterns()
		if len(glob) != 0 {
			isUnprotectedFile = protectedBranch.IsUnprotectedFile(glob, treePath)
		}
		if !protectedBranch.CanUserPush(doer.ID) && !isUnprotectedFile {
			return models.ErrUserCannotCommit{
				UserName: doer.LowerName,
			}
		}
		if protectedBranch.RequireSignedCommits {
			_, _, _, err := asymkey_service.SignCRUDAction(ctx, repo.RepoPath(), doer, repo.RepoPath(), branchName)
			if err != nil {
				if !asymkey_service.IsErrWontSign(err) {
					return err
				}
				return models.ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
		}
		patterns := protectedBranch.GetProtectedFilePatterns()
		for _, pat := range patterns {
			if pat.Match(strings.ToLower(treePath)) {
				return models.ErrFilePathProtected{
					Path: treePath,
				}
			}
		}
	}
	return nil
}

// CreateOrUpdateOrDeleteRepoFiles adds or updates or delete more than file in the given repository
func CreateOrUpdateOrDeleteRepoFiles(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *UpdateRepoFilesOptions) (*PushedFilesRes, error) {
	// If no branch name is set, assume default branch
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	message := strings.TrimSpace(opts.Message)
	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())

	if err != nil {
		log.Error("%v", err)
	}
	defer closer.Close()

	t, err := NewTemporaryUploadRepository(ctx, repo)
	if err != nil {
		log.Error("%v", err)
	}
	defer t.Close()

	// oldBranch must exist for this operation
	if _, err := gitRepo.GetBranch(opts.OldBranch); err != nil {
		return nil, err
	}

	for i := 0; i < len(opts.Files); i++ {

		// A NewBranch can be specified for the file to be created/updated in a new branch.
		// Check to make sure the branch does not already exist, otherwise we can't proceed.
		// If we aren't branching to a new branch, make sure user can commit to the given branch
		if opts.NewBranch != opts.OldBranch {
			existingBranch, err := gitRepo.GetBranch(opts.NewBranch)
			if existingBranch != nil {
				return nil, models.ErrBranchAlreadyExists{
					BranchName: opts.NewBranch,
				}
			}
			if err != nil && !git.IsErrBranchNotExist(err) {
				return nil, err
			}
		} else if err := VerifyBranchProtection(ctx, repo, doer, opts.OldBranch, opts.Files[i].TreePath); err != nil {
			return nil, err
		}

		// If FromTreePath is not set, set it to the opts.TreePath
		if opts.Files[i].TreePath != "" && opts.Files[i].FromTreePath == "" {
			opts.Files[i].FromTreePath = opts.Files[i].TreePath
		}

		// Check that the path given in opts.treePath is valid (not a git path)
		treePath := CleanUploadFileName(opts.Files[i].TreePath)
		if treePath == "" {
			return nil, models.ErrFilenameInvalid{
				Path: opts.Files[i].TreePath,
			}
		}
		// If there is a fromTreePath (we are copying it), also clean it up
		fromTreePath := CleanUploadFileName(opts.Files[i].FromTreePath)
		if fromTreePath == "" && opts.Files[i].FromTreePath != "" {
			return nil, models.ErrFilenameInvalid{
				Path: opts.Files[i].FromTreePath,
			}
		}
		hasOldBranch := true
		if i == 0 {
			if err := t.Clone(opts.OldBranch); err != nil {
				if !git.IsErrBranchNotExist(err) || !repo.IsEmpty {
					return nil, err
				}
				if err := t.Init(); err != nil {
					return nil, err
				}
				hasOldBranch = false
				opts.Files[i].LastCommitID = ""
			}
		}

		if hasOldBranch {
			if err := t.SetDefaultIndex(); err != nil {
				return nil, err
			}
		}

		if opts.Files[i].FileAction != DeleteFileAction {
			encoding := "UTF-8"
			bom := false
			executable := false
			if hasOldBranch {
				// Get the commit of the original branch
				commit, err := t.GetBranchCommit(opts.OldBranch)
				if err != nil {
					return nil, err // Couldn't get a commit for the branch
				}
				// Assigned LastCommitID in opts if it hasn't been set
				if opts.Files[i].LastCommitID == "" {
					opts.Files[i].LastCommitID = commit.ID.String()
				} else {
					lastCommitID, err := t.gitRepo.ConvertToSHA1(opts.Files[i].LastCommitID)
					if err != nil {
						return nil, fmt.Errorf("DeleteRepoFile: Invalid last commit ID: %v", err)
					}
					opts.Files[i].LastCommitID = lastCommitID.String()

				}
				if !opts.Files[i].IsNewFile {
					fromEntry, err := commit.GetTreeEntryByPath(fromTreePath)
					if err != nil {
						return nil, err
					}
					if opts.Files[i].SHA != "" {
						// If a SHA was given and the SHA given doesn't match the SHA of the fromTreePath, throw error
						if opts.Files[i].SHA != fromEntry.ID.String() {
							return nil, models.ErrSHADoesNotMatch{
								Path:       treePath,
								GivenSHA:   opts.Files[i].SHA,
								CurrentSHA: fromEntry.ID.String(),
							}
						}
					} else if opts.Files[i].LastCommitID != "" {
						// If a lastCommitID was given and it doesn't match the commitID of the head of the branch throw
						// an error, but only if we aren't creating a new branch.
						if commit.ID.String() != opts.Files[i].LastCommitID && opts.OldBranch == opts.NewBranch {
							if changed, err := commit.FileChangedSinceCommit(treePath, opts.Files[i].LastCommitID); err != nil {
								return nil, err
							} else if changed {
								return nil, models.ErrCommitIDDoesNotMatch{
									GivenCommitID:   opts.Files[i].LastCommitID,
									CurrentCommitID: opts.Files[i].LastCommitID,
								}
							}
							// The file wasn't modified, so we are good to delete it
						}
					} else {
						// When updating a file, a lastCommitID or SHA needs to be given to make sure other commits
						// haven't been made. We throw an error if one wasn't provided.
						return nil, models.ErrSHAOrCommitIDNotProvided{}
					}
					encoding, bom = detectEncodingAndBOM(fromEntry, repo)
					executable = fromEntry.IsExecutable()
				}

				// For the path where this file will be created/updated, we need to make
				// sure no parts of the path are existing files or links except for the last
				// item in the path which is the file name, and that shouldn't exist IF it is
				// a new file OR is being moved to a new path.
				treePathParts := strings.Split(treePath, "/")
				subTreePath := ""
				for index, part := range treePathParts {
					subTreePath = path.Join(subTreePath, part)
					entry, err := commit.GetTreeEntryByPath(subTreePath)
					if err != nil {
						if git.IsErrNotExist(err) {
							// Means there is no item with that name, so we're good
							break
						}
						return nil, err
					}
					if index < len(treePathParts)-1 {
						if !entry.IsDir() {
							return nil, models.ErrFilePathInvalid{
								Message: fmt.Sprintf("a file exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
								Path:    subTreePath,
								Name:    part,
								Type:    git.EntryModeBlob,
							}
						}
					} else if entry.IsLink() {
						return nil, models.ErrFilePathInvalid{
							Message: fmt.Sprintf("a symbolic link exists where you’re trying to create a subdirectory [path: %s]", subTreePath),
							Path:    subTreePath,
							Name:    part,
							Type:    git.EntryModeSymlink,
						}
					} else if entry.IsDir() {
						return nil, models.ErrFilePathInvalid{
							Message: fmt.Sprintf("a directory exists where you’re trying to create a file [path: %s]", subTreePath),
							Path:    subTreePath,
							Name:    part,
							Type:    git.EntryModeTree,
						}
					} else if fromTreePath != treePath || opts.Files[i].IsNewFile {
						// The entry shouldn't exist if we are creating new file or moving to a new path
						return nil, models.ErrRepoFileAlreadyExists{
							Path: treePath,
						}
					}

				}
			}

			// Get the two paths (might be the same if not moving) from the index if they exist
			filesInIndex, err := t.LsFiles(opts.Files[i].TreePath, opts.Files[i].FromTreePath)
			if err != nil {
				return nil, fmt.Errorf("UpdateRepoFile: %v", err)
			}
			// If is a new file (not updating) then the given path shouldn't exist
			if opts.Files[i].IsNewFile {
				for _, file := range filesInIndex {
					if file == opts.Files[i].TreePath {
						return nil, models.ErrRepoFileAlreadyExists{
							Path: opts.Files[i].TreePath,
						}
					}
				}
			}

			// Remove the old path from the tree
			if fromTreePath != treePath && len(filesInIndex) > 0 {
				for _, file := range filesInIndex {
					if file == fromTreePath {
						if err := t.RemoveFilesFromIndex(opts.Files[i].FromTreePath); err != nil {
							return nil, err
						}
					}
				}
			}

			content := opts.Files[i].Content
			if bom {
				content = string(charset.UTF8BOM) + content
			}
			if encoding != "UTF-8" {
				charsetEncoding, _ := stdcharset.Lookup(encoding)
				if charsetEncoding != nil {
					result, _, err := transform.String(charsetEncoding.NewEncoder(), content)
					if err != nil {
						// Look if we can't encode back in to the original we should just stick with utf-8
						log.Error("Error re-encoding %s (%s) as %s - will stay as UTF-8: %v", opts.Files[i].TreePath, opts.Files[i].FromTreePath, encoding, err)
						result = content
					}
					content = result
				} else {
					log.Error("Unknown encoding: %s", encoding)
				}
			}
			// Reset the opts.Content to our adjusted content to ensure that LFS gets the correct content
			opts.Files[i].Content = content
			var lfsMetaObject *git_model.LFSMetaObject
			if setting.LFS.StartServer {
				// Check there is no way this can return multiple infos
				filename2attribute2info, err := t.gitRepo.CheckAttribute(git.CheckAttributeOpts{
					Attributes: []git.CmdArg{"filter"},
					Filenames:  []string{treePath},
					CachedOnly: true,
				})
				if err != nil {
					return nil, err
				}

				if filename2attribute2info[treePath] != nil && filename2attribute2info[treePath]["filter"] == "lfs" {
					// OK so we are supposed to LFS this data!
					pointer, err := lfs.GeneratePointer(strings.NewReader(opts.Files[i].Content))
					if err != nil {
						return nil, err
					}
					lfsMetaObject = &git_model.LFSMetaObject{Pointer: pointer, RepositoryID: repo.ID}
					content = pointer.StringContent()
				}
			}
			// Add the object to the database
			objectHash, err := t.HashObject(strings.NewReader(content))
			if err != nil {
				return nil, err
			}
			// Add the object to the index
			if executable {
				if err := t.AddObjectToIndex("100755", objectHash, treePath); err != nil {
					return nil, err
				}
			} else {
				if err := t.AddObjectToIndex("100644", objectHash, treePath); err != nil {
					return nil, err
				}
			}

			if lfsMetaObject != nil {
				// We have an LFS object - create it
				lfsMetaObject, err = git_model.NewLFSMetaObject(lfsMetaObject)
				if err != nil {
					return nil, err
				}
				contentStore := lfs.NewContentStore()
				exist, err := contentStore.Exists(lfsMetaObject.Pointer)
				if err != nil {
					return nil, err
				}
				if !exist {
					if err := contentStore.Put(lfsMetaObject.Pointer, strings.NewReader(opts.Files[i].Content)); err != nil {
						if _, err2 := git_model.RemoveLFSMetaObjectByOid(repo.ID, lfsMetaObject.Oid); err2 != nil {
							return nil, fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %w)", lfsMetaObject.Oid, err2, err)
						}
						return nil, err
					}
				}
			}
		} else {
			// Remove the file from the index
			if err := t.RemoveFilesFromIndex(opts.Files[i].TreePath); err != nil {
				return nil, err
			}
		}
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Files[0].Dates != nil {
		commitHash, err = t.CommitTreeWithDate("HEAD", author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree("HEAD", author, committer, treeHash, message, opts.Signoff)
	}
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		log.Error("%T %v", err, err)
		return nil, err
	}

	commit, err := t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	pushedFilesRes := PushedFilesRes{}

	for j := 0; j < len(opts.Files); j++ {

		if opts.Files[j].FileAction != DeleteFileAction {
			file, err := GetFileResponseFromCommit(ctx, repo, commit, opts.NewBranch, opts.Files[j].TreePath)
			if err != nil {
				return nil, err
			}

			pushedFileRes := PushedFileRes{
				Name: file.Content.Name,
				Path: file.Content.Path,
				SHA:  file.Content.SHA,
				// SHA256: file.Content.SHA256,
			}
			pushedFilesRes.Files = append(pushedFilesRes.Files, &pushedFileRes)
			pushedFilesRes.Commit = file.Commit
		}
	}

	return &pushedFilesRes, nil
}
