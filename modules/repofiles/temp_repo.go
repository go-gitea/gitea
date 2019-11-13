// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"github.com/mcuadros/go-version"
)

// TemporaryUploadRepository is a type to wrap our upload repositories as a shallow clone
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
	defer t.gitRepo.Close()
	if err := models.RemoveTemporaryPath(t.basePath); err != nil {
		log.Error("Failed to remove temporary path %s: %v", t.basePath, err)
	}
}

// Clone the base repository to our path and set branch as the HEAD
func (t *TemporaryUploadRepository) Clone(branch string) error {
	if _, stderr, err := process.GetManager().ExecTimeout(5*time.Minute,
		fmt.Sprintf("Clone (git clone -s --bare): %s", t.basePath),
		git.GitExecutable, "clone", "-s", "--bare", "-b", branch, t.repo.RepoPath(), t.basePath); err != nil {
		if matched, _ := regexp.MatchString(".*Remote branch .* not found in upstream origin.*", stderr); matched {
			return git.ErrBranchNotExist{
				Name: branch,
			}
		} else if matched, _ := regexp.MatchString(".* repository .* does not exist.*", stderr); matched {
			return models.ErrRepoNotExist{
				ID:        t.repo.ID,
				UID:       t.repo.OwnerID,
				OwnerName: t.repo.OwnerName,
				Name:      t.repo.Name,
			}
		} else {
			return fmt.Errorf("Clone: %v %s", err, stderr)
		}
	}
	gitRepo, err := git.OpenRepository(t.basePath)
	if err != nil {
		return err
	}
	t.gitRepo = gitRepo
	return nil
}

// SetDefaultIndex sets the git index to our HEAD
func (t *TemporaryUploadRepository) SetDefaultIndex() error {
	if _, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		t.basePath,
		fmt.Sprintf("SetDefaultIndex (git read-tree HEAD): %s", t.basePath),
		git.GitExecutable, "read-tree", "HEAD"); err != nil {
		return fmt.Errorf("SetDefaultIndex: %v %s", err, stderr)
	}
	return nil
}

// LsFiles checks if the given filename arguments are in the index
func (t *TemporaryUploadRepository) LsFiles(filenames ...string) ([]string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := []string{"ls-files", "-z", "--"}
	for _, arg := range filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := exec.CommandContext(ctx, git.GitExecutable, cmdArgs...)
	desc := fmt.Sprintf("lsFiles: (git ls-files) %v", cmdArgs)
	cmd.Dir = t.basePath
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec(%s) failed: %v(%v)", desc, err, ctx.Err())
	}

	pid := process.GetManager().Add(desc, cmd)
	err := cmd.Wait()
	process.GetManager().Remove(pid)

	if err != nil {
		err = fmt.Errorf("exec(%d:%s) failed: %v(%v) stdout: %v stderr: %v", pid, desc, err, ctx.Err(), stdOut, stdErr)
		return nil, err
	}

	filelist := make([]string, len(filenames))
	for _, line := range bytes.Split(stdOut.Bytes(), []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
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

	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := []string{"update-index", "--remove", "-z", "--index-info"}
	cmd := exec.CommandContext(ctx, git.GitExecutable, cmdArgs...)
	desc := fmt.Sprintf("removeFilesFromIndex: (git update-index) %v", filenames)
	cmd.Dir = t.basePath
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = bytes.NewReader(stdIn.Bytes())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("exec(%s) failed: %v(%v)", desc, err, ctx.Err())
	}

	pid := process.GetManager().Add(desc, cmd)
	err := cmd.Wait()
	process.GetManager().Remove(pid)

	if err != nil {
		err = fmt.Errorf("exec(%d:%s) failed: %v(%v) stdout: %v stderr: %v", pid, desc, err, ctx.Err(), stdOut, stdErr)
	}

	return err
}

// HashObject writes the provided content to the object db and returns its hash
func (t *TemporaryUploadRepository) HashObject(content io.Reader) (string, error) {
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	hashCmd := exec.CommandContext(ctx, git.GitExecutable, "hash-object", "-w", "--stdin")
	hashCmd.Dir = t.basePath
	hashCmd.Stdin = content
	stdOutBuffer := new(bytes.Buffer)
	stdErrBuffer := new(bytes.Buffer)
	hashCmd.Stdout = stdOutBuffer
	hashCmd.Stderr = stdErrBuffer
	desc := fmt.Sprintf("hashObject: (git hash-object)")
	if err := hashCmd.Start(); err != nil {
		return "", fmt.Errorf("git hash-object: %s", err)
	}

	pid := process.GetManager().Add(desc, hashCmd)
	err := hashCmd.Wait()
	process.GetManager().Remove(pid)

	if err != nil {
		err = fmt.Errorf("exec(%d:%s) failed: %v(%v) stdout: %v stderr: %v", pid, desc, err, ctx.Err(), stdOutBuffer, stdErrBuffer)
		return "", err
	}

	return strings.TrimSpace(stdOutBuffer.String()), nil
}

// AddObjectToIndex adds the provided object hash to the index with the provided mode and path
func (t *TemporaryUploadRepository) AddObjectToIndex(mode, objectHash, objectPath string) error {
	if _, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		t.basePath,
		fmt.Sprintf("addObjectToIndex (git update-index): %s", t.basePath),
		git.GitExecutable, "update-index", "--add", "--replace", "--cacheinfo", mode, objectHash, objectPath); err != nil {
		if matched, _ := regexp.MatchString(".*Invalid path '.*", stderr); matched {
			return models.ErrFilePathInvalid{
				Message: objectPath,
				Path:    objectPath,
			}
		}
		return fmt.Errorf("git update-index: %s", stderr)
	}
	return nil
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (t *TemporaryUploadRepository) WriteTree() (string, error) {
	treeHash, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		t.basePath,
		fmt.Sprintf("WriteTree (git write-tree): %s", t.basePath),
		git.GitExecutable, "write-tree")
	if err != nil {
		return "", fmt.Errorf("git write-tree: %s", stderr)
	}
	return strings.TrimSpace(treeHash), nil
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
	treeHash, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		t.basePath,
		fmt.Sprintf("GetLastCommit (git rev-parse %s): %s", ref, t.basePath),
		git.GitExecutable, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %s", ref, stderr)
	}
	return strings.TrimSpace(treeHash), nil
}

// CommitTree creates a commit from a given tree for the user with provided message
func (t *TemporaryUploadRepository) CommitTree(author, committer *models.User, treeHash string, message string) (string, error) {
	commitTimeStr := time.Now().Format(time.RFC3339)
	authorSig := author.NewGitSig()
	committerSig := committer.NewGitSig()

	binVersion, err := git.BinVersion()
	if err != nil {
		return "", fmt.Errorf("Unable to get git version: %v", err)
	}

	// FIXME: Should we add SSH_ORIGINAL_COMMAND to this
	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+committerSig.Name,
		"GIT_COMMITTER_EMAIL="+committerSig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(message)
	_, _ = messageBytes.WriteString("\n")

	args := []string{"commit-tree", treeHash, "-p", "HEAD"}
	if version.Compare(binVersion, "2.0.0", ">=") {
		args = append(args, "--no-gpg-sign")
	}

	commitHash, stderr, err := process.GetManager().ExecDirEnvStdIn(5*time.Minute,
		t.basePath,
		fmt.Sprintf("commitTree (git commit-tree): %s", t.basePath),
		env,
		messageBytes,
		git.GitExecutable, args...)
	if err != nil {
		return "", fmt.Errorf("git commit-tree: %s", stderr)
	}
	return strings.TrimSpace(commitHash), nil
}

// Push the provided commitHash to the repository branch by the provided user
func (t *TemporaryUploadRepository) Push(doer *models.User, commitHash string, branch string) error {
	// Because calls hooks we need to pass in the environment
	env := models.PushingEnvironment(doer, t.repo)

	if _, stderr, err := process.GetManager().ExecDirEnv(5*time.Minute,
		t.basePath,
		fmt.Sprintf("actuallyPush (git push): %s", t.basePath),
		env,
		git.GitExecutable, "push", t.repo.RepoPath(), strings.TrimSpace(commitHash)+":refs/heads/"+strings.TrimSpace(branch)); err != nil {
		return fmt.Errorf("git push: %s", stderr)
	}
	return nil
}

// DiffIndex returns a Diff of the current index to the head
func (t *TemporaryUploadRepository) DiffIndex() (diff *models.Diff, err error) {
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stdErr := new(bytes.Buffer)

	cmd := exec.CommandContext(ctx, git.GitExecutable, "diff-index", "--cached", "-p", "HEAD")
	cmd.Dir = t.basePath
	cmd.Stderr = stdErr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v stderr %s", err, stdErr.String())
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v stderr %s", err, stdErr.String())
	}

	pid := process.GetManager().Add(fmt.Sprintf("diffIndex [repo_path: %s]", t.repo.RepoPath()), cmd)
	defer process.GetManager().Remove(pid)

	diff, err = models.ParsePatch(setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return diff, nil
}

// CheckAttribute checks the given attribute of the provided files
func (t *TemporaryUploadRepository) CheckAttribute(attribute string, args ...string) (map[string]map[string]string, error) {
	binVersion, err := git.BinVersion()
	if err != nil {
		log.Error("Error retrieving git version: %v", err)
		return nil, err
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := []string{"check-attr", "-z", attribute}

	// git check-attr --cached first appears in git 1.7.8
	if version.Compare(binVersion, "1.7.8", ">=") {
		cmdArgs = append(cmdArgs, "--cached")
	}
	cmdArgs = append(cmdArgs, "--")

	for _, arg := range args {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := exec.CommandContext(ctx, git.GitExecutable, cmdArgs...)
	desc := fmt.Sprintf("checkAttr: (git check-attr) %s %v", attribute, cmdArgs)
	cmd.Dir = t.basePath
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec(%s) failed: %v(%v)", desc, err, ctx.Err())
	}

	pid := process.GetManager().Add(desc, cmd)
	err = cmd.Wait()
	process.GetManager().Remove(pid)

	if err != nil {
		err = fmt.Errorf("exec(%d:%s) failed: %v(%v) stdout: %v stderr: %v", pid, desc, err, ctx.Err(), stdOut, stdErr)
		return nil, err
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	var name2attribute2info = make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, err
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
