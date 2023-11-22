// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/gitdiff"
)

// TemporaryUploadRepository is a type to wrap our upload repositories as a shallow clone
type TemporaryUploadRepository struct {
	ctx      context.Context
	repo     *repo_model.Repository
	gitRepo  *git.Repository
	basePath string
}

// NewTemporaryUploadRepository creates a new temporary upload repository
func NewTemporaryUploadRepository(ctx context.Context, repo *repo_model.Repository) (*TemporaryUploadRepository, error) {
	basePath, err := repo_module.CreateTemporaryPath("upload")
	if err != nil {
		return nil, err
	}
	t := &TemporaryUploadRepository{ctx: ctx, repo: repo, basePath: basePath}
	return t, nil
}

// Close the repository cleaning up all files
func (t *TemporaryUploadRepository) Close() {
	defer t.gitRepo.Close()
	if err := repo_module.RemoveTemporaryPath(t.basePath); err != nil {
		log.Error("Failed to remove temporary path %s: %v", t.basePath, err)
	}
}

// Clone the base repository to our path and set branch as the HEAD
func (t *TemporaryUploadRepository) Clone(branch string) error {
	if _, _, err := git.NewCommand(t.ctx, "clone", "-s", "--bare", "-b").AddDynamicArguments(branch, t.repo.RepoPath(), t.basePath).RunStdString(nil); err != nil {
		stderr := err.Error()
		if matched, _ := regexp.MatchString(".*Remote branch .* not found in upstream origin.*", stderr); matched {
			return git.ErrBranchNotExist{
				Name: branch,
			}
		} else if matched, _ := regexp.MatchString(".* repository .* does not exist.*", stderr); matched {
			return repo_model.ErrRepoNotExist{
				ID:        t.repo.ID,
				UID:       t.repo.OwnerID,
				OwnerName: t.repo.OwnerName,
				Name:      t.repo.Name,
			}
		}
		return fmt.Errorf("Clone: %w %s", err, stderr)
	}
	gitRepo, err := git.OpenRepository(t.ctx, t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// Init the repository
func (t *TemporaryUploadRepository) Init() error {
	if err := git.InitRepository(t.ctx, t.basePath, false); err != nil {
		return err
	}
	gitRepo, err := git.OpenRepository(t.ctx, t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// SetDefaultIndex sets the git index to our HEAD
func (t *TemporaryUploadRepository) SetDefaultIndex() error {
	if _, _, err := git.NewCommand(t.ctx, "read-tree", "HEAD").RunStdString(&git.RunOpts{Dir: t.basePath}); err != nil {
		return fmt.Errorf("SetDefaultIndex: %w", err)
	}
	return nil
}

// LsFiles checks if the given filename arguments are in the index
func (t *TemporaryUploadRepository) LsFiles(filenames ...string) ([]string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand(t.ctx, "ls-files", "-z").AddDashesAndList(filenames...).
		Run(&git.RunOpts{
			Dir:    t.basePath,
			Stdout: stdOut,
			Stderr: stdErr,
		}); err != nil {
		log.Error("Unable to run git ls-files for temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		err = fmt.Errorf("Unable to run git ls-files for temporary repo of: %s Error: %w\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
		return nil, err
	}

	fileList := make([]string, 0, len(filenames))
	for _, line := range bytes.Split(stdOut.Bytes(), []byte{'\000'}) {
		fileList = append(fileList, string(line))
	}

	return fileList, nil
}

// RemoveFilesFromIndex removes the given files from the index
func (t *TemporaryUploadRepository) RemoveFilesFromIndex(filenames ...string) error {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	stdIn := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			stdIn.WriteString("0 0000000000000000000000000000000000000000\t")
			stdIn.WriteString(file)
			stdIn.WriteByte('\000')
		}
	}

	if err := git.NewCommand(t.ctx, "update-index", "--remove", "-z", "--index-info").
		Run(&git.RunOpts{
			Dir:    t.basePath,
			Stdin:  stdIn,
			Stdout: stdOut,
			Stderr: stdErr,
		}); err != nil {
		log.Error("Unable to update-index for temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		return fmt.Errorf("Unable to update-index for temporary repo: %s Error: %w\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
	}
	return nil
}

// HashObject writes the provided content to the object db and returns its hash
func (t *TemporaryUploadRepository) HashObject(content io.Reader) (string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand(t.ctx, "hash-object", "-w", "--stdin").
		Run(&git.RunOpts{
			Dir:    t.basePath,
			Stdin:  content,
			Stdout: stdOut,
			Stderr: stdErr,
		}); err != nil {
		log.Error("Unable to hash-object to temporary repo: %s (%s) Error: %v\nstdout: %s\nstderr: %s", t.repo.FullName(), t.basePath, err, stdOut.String(), stdErr.String())
		return "", fmt.Errorf("Unable to hash-object to temporary repo: %s Error: %w\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
	}

	return strings.TrimSpace(stdOut.String()), nil
}

// AddObjectToIndex adds the provided object hash to the index with the provided mode and path
func (t *TemporaryUploadRepository) AddObjectToIndex(mode, objectHash, objectPath string) error {
	if _, _, err := git.NewCommand(t.ctx, "update-index", "--add", "--replace", "--cacheinfo").AddDynamicArguments(mode, objectHash, objectPath).RunStdString(&git.RunOpts{Dir: t.basePath}); err != nil {
		stderr := err.Error()
		if matched, _ := regexp.MatchString(".*Invalid path '.*", stderr); matched {
			return models.ErrFilePathInvalid{
				Message: objectPath,
				Path:    objectPath,
			}
		}
		log.Error("Unable to add object to index: %s %s %s in temporary repo %s(%s) Error: %v", mode, objectHash, objectPath, t.repo.FullName(), t.basePath, err)
		return fmt.Errorf("Unable to add object to index at %s in temporary repo %s Error: %w", objectPath, t.repo.FullName(), err)
	}
	return nil
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (t *TemporaryUploadRepository) WriteTree() (string, error) {
	stdout, _, err := git.NewCommand(t.ctx, "write-tree").RunStdString(&git.RunOpts{Dir: t.basePath})
	if err != nil {
		log.Error("Unable to write tree in temporary repo: %s(%s): Error: %v", t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to write-tree in temporary repo for: %s Error: %w", t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

// GetLastCommit gets the last commit ID SHA of the repo
func (t *TemporaryUploadRepository) GetLastCommit() (string, error) {
	return t.GetLastCommitByRef("HEAD")
}

// GetLastCommitByRef gets the last commit ID SHA of the repo by ref
func (t *TemporaryUploadRepository) GetLastCommitByRef(ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	stdout, _, err := git.NewCommand(t.ctx, "rev-parse").AddDynamicArguments(ref).RunStdString(&git.RunOpts{Dir: t.basePath})
	if err != nil {
		log.Error("Unable to get last ref for %s in temporary repo: %s(%s): Error: %v", ref, t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to rev-parse %s in temporary repo for: %s Error: %w", ref, t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

// CommitTree creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTree(parent string, author, committer *user_model.User, treeHash, message string, signoff bool) (string, error) {
	return t.CommitTreeWithDate(parent, author, committer, treeHash, message, signoff, time.Now(), time.Now())
}

// CommitTreeWithDate creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTreeWithDate(parent string, author, committer *user_model.User, treeHash, message string, signoff bool, authorDate, committerDate time.Time) (string, error) {
	authorSig := author.NewGitSig()
	committerSig := committer.NewGitSig()

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+authorDate.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+committerDate.Format(time.RFC3339),
	)

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(message)
	_, _ = messageBytes.WriteString("\n")

	cmdCommitTree := git.NewCommand(t.ctx, "commit-tree").AddDynamicArguments(treeHash)
	if parent != "" {
		cmdCommitTree.AddOptionValues("-p", parent)
	}

	var sign bool
	var keyID string
	var signer *git.Signature
	if parent != "" {
		sign, keyID, signer, _ = asymkey_service.SignCRUDAction(t.ctx, t.repo.RepoPath(), author, t.basePath, parent)
	} else {
		sign, keyID, signer, _ = asymkey_service.SignInitialCommit(t.ctx, t.repo.RepoPath(), author)
	}
	if sign {
		cmdCommitTree.AddOptionFormat("-S%s", keyID)
		if t.repo.GetTrustModel() == repo_model.CommitterTrustModel || t.repo.GetTrustModel() == repo_model.CollaboratorCommitterTrustModel {
			if committerSig.Name != authorSig.Name || committerSig.Email != authorSig.Email {
				// Add trailers
				_, _ = messageBytes.WriteString("\n")
				_, _ = messageBytes.WriteString("Co-authored-by: ")
				_, _ = messageBytes.WriteString(committerSig.String())
				_, _ = messageBytes.WriteString("\n")
				_, _ = messageBytes.WriteString("Co-committed-by: ")
				_, _ = messageBytes.WriteString(committerSig.String())
				_, _ = messageBytes.WriteString("\n")
			}
			committerSig = signer
		}
	} else {
		cmdCommitTree.AddArguments("--no-gpg-sign")
	}

	if signoff {
		// Signed-off-by
		_, _ = messageBytes.WriteString("\n")
		_, _ = messageBytes.WriteString("Signed-off-by: ")
		_, _ = messageBytes.WriteString(committerSig.String())
	}

	env = append(env,
		"GIT_COMMITTER_NAME="+committerSig.Name,
		"GIT_COMMITTER_EMAIL="+committerSig.Email,
	)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := cmdCommitTree.
		Run(&git.RunOpts{
			Env:    env,
			Dir:    t.basePath,
			Stdin:  messageBytes,
			Stdout: stdout,
			Stderr: stderr,
		}); err != nil {
		log.Error("Unable to commit-tree in temporary repo: %s (%s) Error: %v\nStdout: %s\nStderr: %s",
			t.repo.FullName(), t.basePath, err, stdout, stderr)
		return "", fmt.Errorf("Unable to commit-tree in temporary repo: %s Error: %w\nStdout: %s\nStderr: %s",
			t.repo.FullName(), err, stdout, stderr)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Push the provided commitHash to the repository branch by the provided user
func (t *TemporaryUploadRepository) Push(doer *user_model.User, commitHash, branch string) error {
	// Because calls hooks we need to pass in the environment
	env := repo_module.PushingEnvironment(doer, t.repo)
	if err := git.Push(t.ctx, t.basePath, git.PushOptions{
		Remote: t.repo.RepoPath(),
		Branch: strings.TrimSpace(commitHash) + ":" + git.BranchPrefix + strings.TrimSpace(branch),
		Env:    env,
	}); err != nil {
		if git.IsErrPushOutOfDate(err) {
			return err
		} else if git.IsErrPushRejected(err) {
			rejectErr := err.(*git.ErrPushRejected)
			log.Info("Unable to push back to repo from temporary repo due to rejection: %s (%s)\nStdout: %s\nStderr: %s\nError: %v",
				t.repo.FullName(), t.basePath, rejectErr.StdOut, rejectErr.StdErr, rejectErr.Err)
			return err
		}
		log.Error("Unable to push back to repo from temporary repo: %s (%s)\nError: %v",
			t.repo.FullName(), t.basePath, err)
		return fmt.Errorf("Unable to push back to repo from temporary repo: %s (%s) Error: %v",
			t.repo.FullName(), t.basePath, err)
	}
	return nil
}

// DiffIndex returns a Diff of the current index to the head
func (t *TemporaryUploadRepository) DiffIndex() (*gitdiff.Diff, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to open stdout pipe: %v", err)
		return nil, fmt.Errorf("Unable to open stdout pipe: %w", err)
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	stderr := new(bytes.Buffer)
	var diff *gitdiff.Diff
	var finalErr error

	if err := git.NewCommand(t.ctx, "diff-index", "--src-prefix=\\a/", "--dst-prefix=\\b/", "--cached", "-p", "HEAD").
		Run(&git.RunOpts{
			Timeout: 30 * time.Second,
			Dir:     t.basePath,
			Stdout:  stdoutWriter,
			Stderr:  stderr,
			PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				diff, finalErr = gitdiff.ParsePatch(t.ctx, setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdoutReader, "")
				if finalErr != nil {
					log.Error("ParsePatch: %v", finalErr)
					cancel()
				}
				_ = stdoutReader.Close()
				return finalErr
			},
		}); err != nil {
		if finalErr != nil {
			log.Error("Unable to ParsePatch in temporary repo %s (%s). Error: %v", t.repo.FullName(), t.basePath, finalErr)
			return nil, finalErr
		}
		log.Error("Unable to run diff-index pipeline in temporary repo %s (%s). Error: %v\nStderr: %s",
			t.repo.FullName(), t.basePath, err, stderr)
		return nil, fmt.Errorf("Unable to run diff-index pipeline in temporary repo %s. Error: %w\nStderr: %s",
			t.repo.FullName(), err, stderr)
	}

	diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(t.ctx, t.basePath, git.TrustedCmdArgs{"--cached"}, "HEAD")
	if err != nil {
		return nil, err
	}

	return diff, nil
}

// GetBranchCommit Gets the commit object of the given branch
func (t *TemporaryUploadRepository) GetBranchCommit(branch string) (*git.Commit, error) {
	if t.gitRepo == nil {
		return nil, fmt.Errorf("repository has not been cloned")
	}
	return t.gitRepo.GetBranchCommit(branch)
}

// GetCommit Gets the commit object of the given commit ID
func (t *TemporaryUploadRepository) GetCommit(commitID string) (*git.Commit, error) {
	if t.gitRepo == nil {
		return nil, fmt.Errorf("repository has not been cloned")
	}
	return t.gitRepo.GetCommit(commitID)
}
