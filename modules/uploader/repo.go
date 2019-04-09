// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"fmt"
	"io"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// TemporaryUploadRepository is a type to wrap our upload repositories
type TemporaryUploadRepository struct {
	repo     *models.Repository
	gitRepo  *git.Repository
	basePath string
}

// NewTemporaryUploadRepository creates a new temporary upload repository
func NewTemporaryUploadRepository(repo *models.Repository) (*TemporaryUploadRepository, error) {
	basePath, err := models.CreateTemporaryPath("upload")
	if err != nil {
		return nil, err
	}
	t := &TemporaryUploadRepository{repo: repo, basePath: basePath}
	return t, nil
}

// Close the repository cleaning up all files
func (t *TemporaryUploadRepository) Close() {
	if err := models.RemoveTemporaryPath(t.basePath); err != nil {
		log.Error("Failed to remove temporary path %s: %v", t.basePath, err)
	}
}

// Clone the base repository to our path and set branch as the HEAD
func (t *TemporaryUploadRepository) Clone(branch string) error {
	if err := git.Clone(t.repo.RepoPath(), t.basePath, git.CloneRepoOptions{
		Bare:   true,
		Shared: true,
		Branch: branch,
	}); err != nil {
		log.Error("Failed to clone repository: %s (%v)", t.repo.FullName(), err)
		return fmt.Errorf("Failed to clone repository: %s (%v)", t.repo.FullName(), err)
	}

	var err error
	t.gitRepo, err = git.OpenRepository(t.basePath)
	if err != nil {
		log.Error("Unable to open temporary repository: %s (%v)", t.basePath, err)
		return fmt.Errorf("Failed to open new temporary repository in: %s %v", t.basePath, err)
	}
	return nil
}

// SetDefaultIndex sets the git index to our HEAD
func (t *TemporaryUploadRepository) SetDefaultIndex() error {
	if err := t.gitRepo.ReadTreeToIndex("HEAD"); err != nil {
		log.Error("Unable to read HEAD tree to index in: %s %v", t.basePath, err)
		return fmt.Errorf("Unable to read HEAD tree to index in: %s %v", t.basePath, err)
	}
	return nil
}

// LsFiles checks if the given filename arguments are in the index
func (t *TemporaryUploadRepository) LsFiles(filenames ...string) ([]string, error) {
	return t.gitRepo.LsFiles(filenames...)
}

// RemoveFilesFromIndex removes the given files from the index
func (t *TemporaryUploadRepository) RemoveFilesFromIndex(filenames ...string) error {
	return t.gitRepo.RemoveFilesFromIndex(filenames...)
}

// HashObject writes the provided content to the object db and returns its hash
func (t *TemporaryUploadRepository) HashObject(content io.Reader) (string, error) {
	hash, err := t.gitRepo.HashObject(content)
	if err != nil {
		return "", err
	}
	return hash.String(), err
}

// AddObjectToIndex adds the provided object hash to the index with the provided mode and path
func (t *TemporaryUploadRepository) AddObjectToIndex(mode, objectHash, objectPath string) error {
	return t.gitRepo.AddObjectToIndex(mode, git.MustIDFromString(objectHash), objectPath)
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (t *TemporaryUploadRepository) WriteTree() (string, error) {
	tree, err := t.gitRepo.WriteTree()
	if err != nil {
		return "", err
	}
	return tree.ID.String(), err
}

// CommitTree creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTree(doer *models.User, treeHash string, message string) (string, error) {
	tree, err := t.gitRepo.GetTree(treeHash)
	if err != nil {
		return "", err
	}
	hash, err := t.gitRepo.CommitTree(doer.NewGitSig(), tree, git.CommitTreeOpts{
		Message: message,
		Parents: []string{"HEAD"},
	})
	if err != nil {
		return "", err
	}

	return hash.String(), err
}

// Push the provided commitHash to the repository branch by the provided user
func (t *TemporaryUploadRepository) Push(doer *models.User, commitHash string, branch string) error {
	// Because calls hooks we need to pass in the environment

	if err := git.Push(t.basePath, git.PushOptions{
		Remote: "origin",
		Branch: fmt.Sprintf("%s:%s%s", commitHash, git.BranchPrefix, branch),
		Env:    models.PushingEnvironment(doer, t.repo),
	}); err != nil {
		log.Error("%v", err)
		return fmt.Errorf("Push: %v", err)
	}
	return nil
}

// DiffIndex returns a Diff of the current index to the head
func (t *TemporaryUploadRepository) DiffIndex() (diff *models.Diff, err error) {
	stdout, err := t.gitRepo.DiffIndex("HEAD")
	if err != nil {
		return nil, fmt.Errorf("Failed to generate diff: %v", err)
	}

	diff, err = models.ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	return diff, nil
}

// CheckAttribute checks the given attribute of the provided files
func (t *TemporaryUploadRepository) CheckAttribute(attribute string, args ...string) (map[string]map[string]string, error) {
	return t.gitRepo.CheckAttribute(true, attribute, args...)
}
