// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"bytes"
	"container/list"
	"fmt"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	pull_service "code.gitea.io/gitea/services/pull"

	stdcharset "golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
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
}

func detectEncodingAndBOM(entry *git.TreeEntry, repo *models.Repository) (string, bool) {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		// return default
		return "UTF-8", false
	}
	defer reader.Close()
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil {
		// return default
		return "UTF-8", false
	}
	buf = buf[:n]

	if setting.LFS.StartServer {
		meta := lfs.IsPointerFile(&buf)
		if meta != nil {
			meta, err = repo.GetLFSMetaObjectByOid(meta.Oid)
			if err != nil && err != models.ErrLFSObjectNotExist {
				// return default
				return "UTF-8", false
			}
		}
		if meta != nil {
			dataRc, err := lfs.ReadMetaObject(meta)
			if err != nil {
				// return default
				return "UTF-8", false
			}
			defer dataRc.Close()
			buf = make([]byte, 1024)
			n, err = dataRc.Read(buf)
			if err != nil {
				// return default
				return "UTF-8", false
			}
			buf = buf[:n]
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
func CreateOrUpdateRepoFile(repo *models.Repository, doer *models.User, opts *UpdateRepoFileOptions) (*structs.FileResponse, error) {
	// If no branch name is set, assume master
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	// oldBranch must exist for this operation
	if _, err := repo_module.GetBranch(repo, opts.OldBranch); err != nil {
		return nil, err
	}

	// A NewBranch can be specified for the file to be created/updated in a new branch.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		existingBranch, err := repo_module.GetBranch(repo, opts.NewBranch)
		if existingBranch != nil {
			return nil, models.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
		if err != nil && !git.IsErrBranchNotExist(err) {
			return nil, err
		}
	} else {
		protectedBranch, err := repo.GetBranchProtection(opts.OldBranch)
		if err != nil {
			return nil, err
		}
		if protectedBranch != nil {
			if !protectedBranch.CanUserPush(doer.ID) {
				return nil, models.ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
			if protectedBranch.RequireSignedCommits {
				_, _, err := repo.SignCRUDAction(doer, repo.RepoPath(), opts.OldBranch)
				if err != nil {
					if !models.IsErrWontSign(err) {
						return nil, err
					}
					return nil, models.ErrUserCannotCommit{
						UserName: doer.LowerName,
					}
				}
			}
			patterns := protectedBranch.GetProtectedFilePatterns()
			for _, pat := range patterns {
				if pat.Match(strings.ToLower(opts.TreePath)) {
					return nil, models.ErrFilePathProtected{
						Path: opts.TreePath,
					}
				}
			}
		}
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

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		log.Error("%v", err)
	}
	defer t.Close()
	if err := t.Clone(opts.OldBranch); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}

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
			return nil, fmt.Errorf("DeleteRepoFile: Invalid last commit ID: %v", err)
		}
		opts.LastCommitID = lastCommitID.String()

	}

	encoding := "UTF-8"
	bom := false
	executable := false

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

	// Get the two paths (might be the same if not moving) from the index if they exist
	filesInIndex, err := t.LsFiles(opts.TreePath, opts.FromTreePath)
	if err != nil {
		return nil, fmt.Errorf("UpdateRepoFile: %v", err)
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
	var lfsMetaObject *models.LFSMetaObject

	if setting.LFS.StartServer {
		// Check there is no way this can return multiple infos
		filename2attribute2info, err := t.CheckAttribute("filter", treePath)
		if err != nil {
			return nil, err
		}

		if filename2attribute2info[treePath] != nil && filename2attribute2info[treePath]["filter"] == "lfs" {
			// OK so we are supposed to LFS this data!
			oid, err := models.GenerateLFSOid(strings.NewReader(opts.Content))
			if err != nil {
				return nil, err
			}
			lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: int64(len(opts.Content)), RepositoryID: repo.ID}
			content = lfsMetaObject.Pointer()
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
		commitHash, err = t.CommitTreeWithDate(author, committer, treeHash, message, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree(author, committer, treeHash, message)
	}
	if err != nil {
		return nil, err
	}

	if lfsMetaObject != nil {
		// We have an LFS object - create it
		lfsMetaObject, err = models.NewLFSMetaObject(lfsMetaObject)
		if err != nil {
			return nil, err
		}
		contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
		if !contentStore.Exists(lfsMetaObject) {
			if err := contentStore.Put(lfsMetaObject, strings.NewReader(opts.Content)); err != nil {
				if _, err2 := repo.RemoveLFSMetaObjectByOid(lfsMetaObject.Oid); err2 != nil {
					return nil, fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", lfsMetaObject.Oid, err2, err)
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

	commit, err = t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	file, err := GetFileResponseFromCommit(repo, commit, opts.NewBranch, treePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// PushUpdateOptions defines the push update options
type PushUpdateOptions struct {
	PusherID     int64
	PusherName   string
	RepoUserName string
	RepoName     string
	RefFullName  string
	OldCommitID  string
	NewCommitID  string
}

// IsNewRef return true if it's a first-time push to a branch, tag or etc.
func (opts PushUpdateOptions) IsNewRef() bool {
	return opts.OldCommitID == git.EmptySHA
}

// IsDelRef return true if it's a deletion to a branch or tag
func (opts PushUpdateOptions) IsDelRef() bool {
	return opts.NewCommitID == git.EmptySHA
}

// IsUpdateRef return true if it's an update operation
func (opts PushUpdateOptions) IsUpdateRef() bool {
	return !opts.IsNewRef() && !opts.IsDelRef()
}

// IsTag return true if it's an operation to a tag
func (opts PushUpdateOptions) IsTag() bool {
	return strings.HasPrefix(opts.RefFullName, git.TagPrefix)
}

// IsNewTag return true if it's a creation to a tag
func (opts PushUpdateOptions) IsNewTag() bool {
	return opts.IsTag() && opts.IsNewRef()
}

// IsDelTag return true if it's a deletion to a tag
func (opts PushUpdateOptions) IsDelTag() bool {
	return opts.IsTag() && opts.IsDelRef()
}

// IsBranch return true if it's a push to branch
func (opts PushUpdateOptions) IsBranch() bool {
	return strings.HasPrefix(opts.RefFullName, git.BranchPrefix)
}

// IsNewBranch return true if it's the first-time push to a branch
func (opts PushUpdateOptions) IsNewBranch() bool {
	return opts.IsBranch() && opts.IsNewRef()
}

// IsUpdateBranch return true if it's not the first push to a branch
func (opts PushUpdateOptions) IsUpdateBranch() bool {
	return opts.IsBranch() && opts.IsUpdateRef()
}

// IsDelBranch return true if it's a deletion to a branch
func (opts PushUpdateOptions) IsDelBranch() bool {
	return opts.IsBranch() && opts.IsDelRef()
}

// TagName returns simple tag name if it's an operation to a tag
func (opts PushUpdateOptions) TagName() string {
	return opts.RefFullName[len(git.TagPrefix):]
}

// BranchName returns simple branch name if it's an operation to branch
func (opts PushUpdateOptions) BranchName() string {
	return opts.RefFullName[len(git.BranchPrefix):]
}

// RepoFullName returns repo full name
func (opts PushUpdateOptions) RepoFullName() string {
	return opts.RepoUserName + "/" + opts.RepoName
}

// PushUpdate must be called for any push actions in order to
// generates necessary push action history feeds and other operations
func PushUpdate(repo *models.Repository, branch string, opts PushUpdateOptions) error {
	if opts.IsNewRef() && opts.IsDelRef() {
		return fmt.Errorf("Old and new revisions are both %s", git.EmptySHA)
	}

	repoPath := models.RepoPath(opts.RepoUserName, opts.RepoName)

	_, err := git.NewCommand("update-server-info").RunInDir(repoPath)
	if err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %v", err)
	}

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	if err = repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	var commits = &repo_module.PushCommits{}

	if opts.IsTag() { // If is tag reference
		tagName := opts.TagName()
		if opts.IsDelRef() {
			if err := models.PushUpdateDeleteTag(repo, tagName); err != nil {
				return fmt.Errorf("PushUpdateDeleteTag: %v", err)
			}
		} else {
			// Clear cache for tag commit count
			cache.Remove(repo.GetCommitsCountCacheKey(tagName, true))
			if err := repo_module.PushUpdateAddTag(repo, gitRepo, tagName); err != nil {
				return fmt.Errorf("PushUpdateAddTag: %v", err)
			}
		}
	} else if opts.IsBranch() { // If is branch reference
		pusher, err := models.GetUserByID(opts.PusherID)
		if err != nil {
			return err
		}

		if !opts.IsDelRef() {
			// Clear cache for branch commit count
			cache.Remove(repo.GetCommitsCountCacheKey(opts.BranchName(), true))

			newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
			if err != nil {
				return fmt.Errorf("gitRepo.GetCommit: %v", err)
			}

			// Push new branch.
			var l *list.List
			if opts.IsNewRef() {
				l, err = newCommit.CommitsBeforeLimit(10)
				if err != nil {
					return fmt.Errorf("newCommit.CommitsBeforeLimit: %v", err)
				}
			} else {
				l, err = newCommit.CommitsBeforeUntil(opts.OldCommitID)
				if err != nil {
					return fmt.Errorf("newCommit.CommitsBeforeUntil: %v", err)
				}
			}

			commits = repo_module.ListToPushCommits(l)

			if err = models.RemoveDeletedBranch(repo.ID, opts.BranchName()); err != nil {
				log.Error("models.RemoveDeletedBranch %s/%s failed: %v", repo.ID, opts.BranchName(), err)
			}

			if err = models.WatchIfAuto(opts.PusherID, repo.ID, true); err != nil {
				log.Warn("Fail to perform auto watch on user %v for repo %v: %v", opts.PusherID, repo.ID, err)
			}

			log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)

			go pull_service.AddTestPullRequestTask(pusher, repo.ID, branch, true, opts.OldCommitID, opts.NewCommitID)
		} else if err = pull_service.CloseBranchPulls(pusher, repo.ID, branch); err != nil {
			// close all related pulls
			log.Error("close related pull request failed: %v", err)
		}
	}

	if err := CommitRepoAction(&CommitRepoActionOptions{
		PushUpdateOptions: opts,
		RepoOwnerID:       repo.OwnerID,
		Commits:           commits,
	}); err != nil {
		return fmt.Errorf("CommitRepoAction: %v", err)
	}

	return nil
}

// PushUpdates generates push action history feeds for push updating multiple refs
func PushUpdates(repo *models.Repository, optsList []*PushUpdateOptions) error {
	repoPath := repo.RepoPath()
	_, err := git.NewCommand("update-server-info").RunInDir(repoPath)
	if err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %v", err)
	}
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	if err = repo.UpdateSize(models.DefaultDBContext()); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	actions, err := createCommitRepoActions(repo, gitRepo, optsList)
	if err != nil {
		return err
	}
	if err := CommitRepoAction(actions...); err != nil {
		return fmt.Errorf("CommitRepoAction: %v", err)
	}

	var pusher *models.User

	for _, opts := range optsList {
		if !opts.IsBranch() {
			continue
		}

		branch := opts.BranchName()

		if pusher == nil || pusher.ID != opts.PusherID {
			var err error
			pusher, err = models.GetUserByID(opts.PusherID)
			if err != nil {
				return err
			}
		}

		if !opts.IsDelRef() {
			if err = models.RemoveDeletedBranch(repo.ID, branch); err != nil {
				log.Error("models.RemoveDeletedBranch %s/%s failed: %v", repo.ID, branch, err)
			}

			if err = models.WatchIfAuto(opts.PusherID, repo.ID, true); err != nil {
				log.Warn("Fail to perform auto watch on user %v for repo %v: %v", opts.PusherID, repo.ID, err)
			}

			log.Trace("TriggerTask '%s/%s' by %s", repo.Name, branch, pusher.Name)

			go pull_service.AddTestPullRequestTask(pusher, repo.ID, branch, true, opts.OldCommitID, opts.NewCommitID)
			// close all related pulls
		} else if err = pull_service.CloseBranchPulls(pusher, repo.ID, branch); err != nil {
			log.Error("close related pull request failed: %v", err)
		}
	}

	return nil
}

func createCommitRepoActions(repo *models.Repository, gitRepo *git.Repository, optsList []*PushUpdateOptions) ([]*CommitRepoActionOptions, error) {
	addTags := make([]string, 0, len(optsList))
	delTags := make([]string, 0, len(optsList))
	actions := make([]*CommitRepoActionOptions, 0, len(optsList))

	for _, opts := range optsList {
		if opts.IsNewRef() && opts.IsDelRef() {
			return nil, fmt.Errorf("Old and new revisions are both %s", git.EmptySHA)
		}
		var commits = &repo_module.PushCommits{}
		if opts.IsTag() {
			// If is tag reference
			tagName := opts.TagName()
			if opts.IsDelRef() {
				delTags = append(delTags, tagName)
			} else {
				cache.Remove(repo.GetCommitsCountCacheKey(tagName, true))
				addTags = append(addTags, tagName)
			}
		} else if !opts.IsDelRef() {
			// If is branch reference

			// Clear cache for branch commit count
			cache.Remove(repo.GetCommitsCountCacheKey(opts.BranchName(), true))

			newCommit, err := gitRepo.GetCommit(opts.NewCommitID)
			if err != nil {
				return nil, fmt.Errorf("gitRepo.GetCommit: %v", err)
			}

			// Push new branch.
			var l *list.List
			if opts.IsNewRef() {
				l, err = newCommit.CommitsBeforeLimit(10)
				if err != nil {
					return nil, fmt.Errorf("newCommit.CommitsBeforeLimit: %v", err)
				}
			} else {
				l, err = newCommit.CommitsBeforeUntil(opts.OldCommitID)
				if err != nil {
					return nil, fmt.Errorf("newCommit.CommitsBeforeUntil: %v", err)
				}
			}

			commits = repo_module.ListToPushCommits(l)
		}
		actions = append(actions, &CommitRepoActionOptions{
			PushUpdateOptions: *opts,
			RepoOwnerID:       repo.OwnerID,
			Commits:           commits,
		})
	}
	if err := repo_module.PushUpdateAddDeleteTags(repo, gitRepo, addTags, delTags); err != nil {
		return nil, fmt.Errorf("PushUpdateAddDeleteTags: %v", err)
	}
	return actions, nil
}
