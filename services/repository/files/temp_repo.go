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

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/gitdiff"
)

// TemporaryUploadRepository is a type to wrap our upload repositories as a shallow clone
type TemporaryUploadRepository struct {
	repo     *repo_model.Repository
	gitRepo  *git.Repository
	basePath string
}

// NewTemporaryUploadRepository creates a new temporary upload repository
func NewTemporaryUploadRepository(repo *repo_model.Repository) (*TemporaryUploadRepository, error) {
	basePath, err := repo_module.CreateTemporaryPath("upload")
	if err != nil {
		return nil, err
	}
	t := &TemporaryUploadRepository{repo: repo, basePath: basePath}
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
func (t *TemporaryUploadRepository) Clone(ctx context.Context, branch string, bare bool) error {
	cmd := git.NewCommand("clone", "-s", "-b").AddDynamicArguments(branch, t.repo.RepoPath(), t.basePath)
	if bare {
		cmd.AddArguments("--bare")
	}

	if _, _, err := cmd.RunStdString(ctx, nil); err != nil {
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
	gitRepo, err := git.OpenRepository(ctx, t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// Init the repository
func (t *TemporaryUploadRepository) Init(ctx context.Context, objectFormatName string) error {
	if err := git.InitRepository(ctx, t.basePath, false, objectFormatName); err != nil {
		return err
	}
	gitRepo, err := git.OpenRepository(ctx, t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// SetDefaultIndex sets the git index to our HEAD
func (t *TemporaryUploadRepository) SetDefaultIndex(ctx context.Context) error {
	if _, _, err := git.NewCommand("read-tree", "HEAD").RunStdString(ctx, &git.RunOpts{Dir: t.basePath}); err != nil {
		return fmt.Errorf("SetDefaultIndex: %w", err)
	}
	return nil
}

// RefreshIndex looks at the current index and checks to see if merges or updates are needed by checking stat() information.
func (t *TemporaryUploadRepository) RefreshIndex(ctx context.Context) error {
	if _, _, err := git.NewCommand("update-index", "--refresh").RunStdString(ctx, &git.RunOpts{Dir: t.basePath}); err != nil {
		return fmt.Errorf("RefreshIndex: %w", err)
	}
	return nil
}

// LsFiles checks if the given filename arguments are in the index
func (t *TemporaryUploadRepository) LsFiles(ctx context.Context, filenames ...string) ([]string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand("ls-files", "-z").AddDashesAndList(filenames...).
		Run(ctx, &git.RunOpts{
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
func (t *TemporaryUploadRepository) RemoveFilesFromIndex(ctx context.Context, filenames ...string) error {
	objFmt, err := t.gitRepo.GetObjectFormat()
	if err != nil {
		return fmt.Errorf("unable to get object format for temporary repo: %q, error: %w", t.repo.FullName(), err)
	}
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	stdIn := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			// man git-update-index: input syntax (1): mode SP sha1 TAB path
			// mode=0 means "remove from index", then hash part "does not matter as long as it is well formatted."
			_, _ = fmt.Fprintf(stdIn, "0 %s\t%s\x00", objFmt.EmptyObjectID(), file)
		}
	}

	if err := git.NewCommand("update-index", "--remove", "-z", "--index-info").
		Run(ctx, &git.RunOpts{
			Dir:    t.basePath,
			Stdin:  stdIn,
			Stdout: stdOut,
			Stderr: stdErr,
		}); err != nil {
		return fmt.Errorf("unable to update-index for temporary repo: %q, error: %w\nstdout: %s\nstderr: %s", t.repo.FullName(), err, stdOut.String(), stdErr.String())
	}
	return nil
}

// HashObject writes the provided content to the object db and returns its hash
func (t *TemporaryUploadRepository) HashObject(ctx context.Context, content io.Reader) (string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	if err := git.NewCommand("hash-object", "-w", "--stdin").
		Run(ctx, &git.RunOpts{
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
func (t *TemporaryUploadRepository) AddObjectToIndex(ctx context.Context, mode, objectHash, objectPath string) error {
	if _, _, err := git.NewCommand("update-index", "--add", "--replace", "--cacheinfo").AddDynamicArguments(mode, objectHash, objectPath).RunStdString(ctx, &git.RunOpts{Dir: t.basePath}); err != nil {
		stderr := err.Error()
		if matched, _ := regexp.MatchString(".*Invalid path '.*", stderr); matched {
			return ErrFilePathInvalid{
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
func (t *TemporaryUploadRepository) WriteTree(ctx context.Context) (string, error) {
	stdout, _, err := git.NewCommand("write-tree").RunStdString(ctx, &git.RunOpts{Dir: t.basePath})
	if err != nil {
		log.Error("Unable to write tree in temporary repo: %s(%s): Error: %v", t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to write-tree in temporary repo for: %s Error: %w", t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

// GetLastCommit gets the last commit ID SHA of the repo
func (t *TemporaryUploadRepository) GetLastCommit(ctx context.Context) (string, error) {
	return t.GetLastCommitByRef(ctx, "HEAD")
}

// GetLastCommitByRef gets the last commit ID SHA of the repo by ref
func (t *TemporaryUploadRepository) GetLastCommitByRef(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	stdout, _, err := git.NewCommand("rev-parse").AddDynamicArguments(ref).RunStdString(ctx, &git.RunOpts{Dir: t.basePath})
	if err != nil {
		log.Error("Unable to get last ref for %s in temporary repo: %s(%s): Error: %v", ref, t.repo.FullName(), t.basePath, err)
		return "", fmt.Errorf("Unable to rev-parse %s in temporary repo for: %s Error: %w", ref, t.repo.FullName(), err)
	}
	return strings.TrimSpace(stdout), nil
}

type CommitTreeUserOptions struct {
	ParentCommitID string
	TreeHash       string
	CommitMessage  string
	SignOff        bool

	DoerUser *user_model.User

	AuthorIdentity    *IdentityOptions // if nil, use doer
	AuthorTime        *time.Time       // if nil, use now
	CommitterIdentity *IdentityOptions
	CommitterTime     *time.Time
}

func makeGitUserSignature(doer *user_model.User, identity, other *IdentityOptions) *git.Signature {
	gitSig := &git.Signature{}
	if identity != nil {
		gitSig.Name, gitSig.Email = identity.GitUserName, identity.GitUserEmail
	}
	if other != nil {
		gitSig.Name = util.IfZero(gitSig.Name, other.GitUserName)
		gitSig.Email = util.IfZero(gitSig.Email, other.GitUserEmail)
	}
	if gitSig.Name == "" {
		gitSig.Name = doer.GitName()
	}
	if gitSig.Email == "" {
		gitSig.Email = doer.GetEmail()
	}
	return gitSig
}

// CommitTree creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTree(ctx context.Context, opts *CommitTreeUserOptions) (string, error) {
	authorSig := makeGitUserSignature(opts.DoerUser, opts.AuthorIdentity, opts.CommitterIdentity)
	committerSig := makeGitUserSignature(opts.DoerUser, opts.CommitterIdentity, opts.AuthorIdentity)

	authorDate := opts.AuthorTime
	committerDate := opts.CommitterTime
	if authorDate == nil && committerDate == nil {
		authorDate = util.ToPointer(time.Now())
		committerDate = authorDate
	} else if authorDate == nil {
		authorDate = committerDate
	} else if committerDate == nil {
		committerDate = authorDate
	}

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+authorDate.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+committerDate.Format(time.RFC3339),
	)

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(opts.CommitMessage)
	_, _ = messageBytes.WriteString("\n")

	cmdCommitTree := git.NewCommand("commit-tree").AddDynamicArguments(opts.TreeHash)
	if opts.ParentCommitID != "" {
		cmdCommitTree.AddOptionValues("-p", opts.ParentCommitID)
	}

	var sign bool
	var keyID string
	var signer *git.Signature
	if opts.ParentCommitID != "" {
		sign, keyID, signer, _ = asymkey_service.SignCRUDAction(ctx, t.repo.RepoPath(), opts.DoerUser, t.basePath, opts.ParentCommitID)
	} else {
		sign, keyID, signer, _ = asymkey_service.SignInitialCommit(ctx, t.repo.RepoPath(), opts.DoerUser)
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

	if opts.SignOff {
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
		Run(ctx, &git.RunOpts{
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
func (t *TemporaryUploadRepository) Push(ctx context.Context, doer *user_model.User, commitHash, branch string) error {
	// Because calls hooks we need to pass in the environment
	env := repo_module.PushingEnvironment(doer, t.repo)
	if err := git.Push(ctx, t.basePath, git.PushOptions{
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
func (t *TemporaryUploadRepository) DiffIndex(ctx context.Context) (*gitdiff.Diff, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to open stdout pipe: %w", err)
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	stderr := new(bytes.Buffer)
	var diff *gitdiff.Diff
	err = git.NewCommand("diff-index", "--src-prefix=\\a/", "--dst-prefix=\\b/", "--cached", "-p", "HEAD").
		Run(ctx, &git.RunOpts{
			Timeout: 30 * time.Second,
			Dir:     t.basePath,
			Stdout:  stdoutWriter,
			Stderr:  stderr,
			PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				defer cancel()
				var diffErr error
				diff, diffErr = gitdiff.ParsePatch(ctx, setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdoutReader, "")
				_ = stdoutReader.Close()
				if diffErr != nil {
					// if the diffErr is not nil, it will be returned as the error of "Run()"
					return fmt.Errorf("ParsePatch: %w", diffErr)
				}
				return nil
			},
		})
	if err != nil && !git.IsErrCanceledOrKilled(err) {
		log.Error("Unable to diff-index in temporary repo %s (%s). Error: %v\nStderr: %s", t.repo.FullName(), t.basePath, err, stderr)
		return nil, fmt.Errorf("unable to run diff-index pipeline in temporary repo: %w", err)
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
