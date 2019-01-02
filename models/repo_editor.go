// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/Unknwon/com"
	gouuid "github.com/satori/go.uuid"

	"code.gitea.io/git"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
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

func (repo *Repository) bareClone(repoPath string, branch string) (err error) {
	if _, stderr, err := process.GetManager().ExecTimeout(5*time.Minute,
		fmt.Sprintf("bareClone (git clone -s --bare): %s", repoPath),
		"git", "clone", "-s", "--bare", "-b", branch, repo.RepoPath(), repoPath); err != nil {
		return fmt.Errorf("bareClone: %v %s", err, stderr)
	}
	return nil
}

func (repo *Repository) setDefaultIndex(repoPath string) (err error) {
	if _, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		repoPath,
		fmt.Sprintf("setDefaultIndex (git read-tree HEAD): %s", repoPath),
		"git", "read-tree", "HEAD"); err != nil {
		return fmt.Errorf("setDefaultIndex: %v %s", err, stderr)
	}
	return nil
}

// FIXME: We should probably return the mode too
func (repo *Repository) lsFiles(repoPath string, args ...string) ([]string, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := []string{"ls-files", "-z", "--"}
	for _, arg := range args {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	desc := fmt.Sprintf("lsFiles: (git ls-files) %v", cmdArgs)
	cmd.Dir = repoPath
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

	filelist := make([]string, len(args))
	for _, line := range bytes.Split(stdOut.Bytes(), []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}

func (repo *Repository) removeFilesFromIndex(repoPath string, args ...string) error {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	stdIn := new(bytes.Buffer)
	for _, file := range args {
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
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	desc := fmt.Sprintf("removeFilesFromIndex: (git update-index) %v", args)
	cmd.Dir = repoPath
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

func (repo *Repository) hashObject(repoPath string, content io.Reader) (string, error) {
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	hashCmd := exec.CommandContext(ctx, "git", "hash-object", "-w", "--stdin")
	hashCmd.Dir = repoPath
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

func (repo *Repository) addObjectToIndex(repoPath, mode, objectHash, objectPath string) error {
	if _, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		repoPath,
		fmt.Sprintf("addObjectToIndex (git update-index): %s", repoPath),
		"git", "update-index", "--add", "--replace", "--cacheinfo", mode, objectHash, objectPath); err != nil {
		return fmt.Errorf("git update-index: %s", stderr)
	}
	return nil
}

func (repo *Repository) writeTree(repoPath string) (string, error) {

	treeHash, stderr, err := process.GetManager().ExecDir(5*time.Minute,
		repoPath,
		fmt.Sprintf("writeTree (git write-tree): %s", repoPath),
		"git", "write-tree")
	if err != nil {
		return "", fmt.Errorf("git write-tree: %s", stderr)
	}
	return strings.TrimSpace(treeHash), nil
}

func (repo *Repository) commitTree(repoPath string, doer *User, treeHash string, message string) (string, error) {
	commitTimeStr := time.Now().Format(time.UnixDate)

	// FIXME: Should we add SSH_ORIGINAL_COMMAND to this
	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+doer.DisplayName(),
		"GIT_AUTHOR_EMAIL="+doer.getEmail(),
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+doer.DisplayName(),
		"GIT_COMMITTER_EMAIL="+doer.getEmail(),
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	commitHash, stderr, err := process.GetManager().ExecDirEnv(5*time.Minute,
		repoPath,
		fmt.Sprintf("commitTree (git commit-tree): %s", repoPath),
		env,
		"git", "commit-tree", treeHash, "-p", "HEAD", "-m", message)
	if err != nil {
		return "", fmt.Errorf("git commit-tree: %s", stderr)
	}
	return strings.TrimSpace(commitHash), nil
}

func (repo *Repository) actuallyPush(repoPath string, doer *User, commitHash string, branch string) error {
	isWiki := "false"
	if strings.HasSuffix(repo.Name, ".wiki") {
		isWiki = "true"
	}

	// FIXME: Should we add SSH_ORIGINAL_COMMAND to this
	// Because calls hooks we need to pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+doer.DisplayName(),
		"GIT_AUTHOR_EMAIL="+doer.getEmail(),
		"GIT_COMMITTER_NAME="+doer.DisplayName(),
		"GIT_COMMITTER_EMAIL="+doer.getEmail(),
		EnvRepoName+"="+repo.Name,
		EnvRepoUsername+"="+repo.OwnerName,
		EnvRepoIsWiki+"="+isWiki,
		EnvPusherName+"="+doer.Name,
		EnvPusherID+"="+fmt.Sprintf("%d", doer.ID),
		ProtectedBranchRepoID+"="+fmt.Sprintf("%d", repo.ID),
	)

	if _, stderr, err := process.GetManager().ExecDirEnv(5*time.Minute,
		repoPath,
		fmt.Sprintf("actuallyPush (git push): %s", repoPath),
		env,
		"git", "push", repo.RepoPath(), strings.TrimSpace(commitHash)+":refs/heads/"+strings.TrimSpace(branch)); err != nil {
		return fmt.Errorf("git push: %s", stderr)
	}
	return nil
}

// UpdateRepoFile adds or updates a file in the repository.
func (repo *Repository) UpdateRepoFile(doer *User, opts UpdateRepoFileOptions) (err error) {
	timeStr := com.ToStr(time.Now().Nanosecond()) // SHOULD USE SOMETHING UNIQUE
	tmpBasePath := path.Join(LocalCopyPath(), "upload-"+timeStr+".git")
	if err := os.MkdirAll(path.Dir(tmpBasePath), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", tmpBasePath, err)
	}

	defer os.RemoveAll(path.Dir(tmpBasePath))

	// Do a bare shared clone into tmpBasePath and
	// make HEAD to point to the OldBranch tree
	if err := repo.bareClone(tmpBasePath, opts.OldBranch); err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	// Set the default index
	if err := repo.setDefaultIndex(tmpBasePath); err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	filesInIndex, err := repo.lsFiles(tmpBasePath, opts.NewTreeName, opts.OldTreeName)

	if err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	if opts.IsNewFile {
		for _, file := range filesInIndex {
			if file == opts.NewTreeName {
				return ErrRepoFileAlreadyExist{opts.NewTreeName}
			}
		}
	}

	//var stdout string
	if opts.OldTreeName != opts.NewTreeName && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == opts.OldTreeName {
				if err := repo.removeFilesFromIndex(tmpBasePath, opts.OldTreeName); err != nil {
					return err
				}
			}
		}

	}

	// Add the object to the database
	objectHash, err := repo.hashObject(tmpBasePath, strings.NewReader(opts.Content))
	if err != nil {
		return err
	}

	// Add the object to the index
	if err := repo.addObjectToIndex(tmpBasePath, "100666", objectHash, opts.NewTreeName); err != nil {
		return err
	}

	// Now write the tree
	treeHash, err := repo.writeTree(tmpBasePath)
	if err != nil {
		return err
	}

	// Now commit the tree
	commitHash, err := repo.commitTree(tmpBasePath, doer, treeHash, opts.Message)
	if err != nil {
		return err
	}

	// Then push this tree to NewBranch
	if err := repo.actuallyPush(tmpBasePath, doer, commitHash, opts.NewBranch); err != nil {
		return err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}
	err = PushUpdate(
		opts.NewBranch,
		PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commitHash,
		},
	)
	if err != nil {
		return fmt.Errorf("PushUpdate: %v", err)
	}
	UpdateRepoIndexer(repo)

	return nil
}

func (repo *Repository) diffIndex(repoPath string) (diff *Diff, err error) {
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stdErr := new(bytes.Buffer)

	cmd := exec.CommandContext(ctx, "git", "diff-index", "--cached", "-p", "HEAD")
	cmd.Dir = repoPath
	cmd.Stderr = stdErr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v stderr %s", err, stdErr.String())
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v stderr %s", err, stdErr.String())
	}

	pid := process.GetManager().Add(fmt.Sprintf("diffIndex [repo_path: %s]", repo.RepoPath()), cmd)
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

// GetDiffPreview produces and returns diff result of a file which is not yet committed.
func (repo *Repository) GetDiffPreview(branch, treePath, content string) (diff *Diff, err error) {
	timeStr := com.ToStr(time.Now().Nanosecond()) // SHOULD USE SOMETHING UNIQUE
	tmpBasePath := path.Join(LocalCopyPath(), "upload-"+timeStr+".git")
	if err := os.MkdirAll(path.Dir(tmpBasePath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("Failed to create dir %s: %v", tmpBasePath, err)
	}

	defer os.RemoveAll(path.Dir(tmpBasePath))

	// Do a bare shared clone into tmpBasePath and
	// make HEAD to point to the branch tree
	if err := repo.bareClone(tmpBasePath, branch); err != nil {
		return nil, fmt.Errorf("GetDiffPreview: %v", err)
	}

	// Set the default index
	if err := repo.setDefaultIndex(tmpBasePath); err != nil {
		return nil, fmt.Errorf("GetDiffPreview: %v", err)
	}

	// Add the object to the database
	objectHash, err := repo.hashObject(tmpBasePath, strings.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("GetDiffPreview: %v", err)
	}

	// Add the object to the index
	if err := repo.addObjectToIndex(tmpBasePath, "100666", objectHash, treePath); err != nil {
		return nil, fmt.Errorf("GetDiffPreview: %v", err)
	}

	return repo.diffIndex(tmpBasePath)
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
func (repo *Repository) DeleteRepoFile(doer *User, opts DeleteRepoFileOptions) (err error) {
	timeStr := com.ToStr(time.Now().Nanosecond()) // SHOULD USE SOMETHING UNIQUE
	tmpBasePath := path.Join(LocalCopyPath(), "upload-"+timeStr+".git")
	if err := os.MkdirAll(path.Dir(tmpBasePath), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", tmpBasePath, err)
	}

	defer os.RemoveAll(path.Dir(tmpBasePath))

	// Do a bare shared clone into tmpBasePath and
	// make HEAD to point to the OldBranch tree
	if err := repo.bareClone(tmpBasePath, opts.OldBranch); err != nil {
		return fmt.Errorf("UpdateRepoFile: %s", err)
	}

	// Set the default index
	if err := repo.setDefaultIndex(tmpBasePath); err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	if err := repo.removeFilesFromIndex(tmpBasePath, opts.TreePath); err != nil {
		return err
	}

	// Now write the tree
	treeHash, err := repo.writeTree(tmpBasePath)
	if err != nil {
		return err
	}

	// Now commit the tree
	commitHash, err := repo.commitTree(tmpBasePath, doer, treeHash, opts.Message)
	if err != nil {
		return err
	}

	// Then push this tree to NewBranch
	if err := repo.actuallyPush(tmpBasePath, doer, commitHash, opts.NewBranch); err != nil {
		return err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}
	err = PushUpdate(
		opts.NewBranch,
		PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commitHash,
		},
	)
	if err != nil {
		return fmt.Errorf("PushUpdate: %v", err)
	}

	// FIXME: Should we UpdateRepoIndexer(repo) here?
	return nil
}

//  ____ ___        .__                    .___ ___________.___.__
// |    |   \______ |  |   _________     __| _/ \_   _____/|   |  |   ____   ______
// |    |   /\____ \|  |  /  _ \__  \   / __ |   |    __)  |   |  | _/ __ \ /  ___/
// |    |  / |  |_> >  |_(  <_> ) __ \_/ /_/ |   |     \   |   |  |_\  ___/ \___ \
// |______/  |   __/|____/\____(____  /\____ |   \___  /   |___|____/\___  >____  >
//           |__|                   \/      \/       \/                  \/     \/
//

// Upload represent a uploaded file to a repo to be deleted when moved
type Upload struct {
	ID   int64  `xorm:"pk autoincr"`
	UUID string `xorm:"uuid UNIQUE"`
	Name string
}

// UploadLocalPath returns where uploads is stored in local file system based on given UUID.
func UploadLocalPath(uuid string) string {
	return path.Join(setting.Repository.Upload.TempPath, uuid[0:1], uuid[1:2], uuid)
}

// LocalPath returns where uploads are temporarily stored in local file system.
func (upload *Upload) LocalPath() string {
	return UploadLocalPath(upload.UUID)
}

// NewUpload creates a new upload object.
func NewUpload(name string, buf []byte, file multipart.File) (_ *Upload, err error) {
	upload := &Upload{
		UUID: gouuid.NewV4().String(),
		Name: name,
	}

	localPath := upload.LocalPath()
	if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("MkdirAll: %v", err)
	}

	fw, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if _, err = fw.Write(buf); err != nil {
		return nil, fmt.Errorf("Write: %v", err)
	} else if _, err = io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("Copy: %v", err)
	}

	if _, err := x.Insert(upload); err != nil {
		return nil, err
	}

	return upload, nil
}

// GetUploadByUUID returns the Upload by UUID
func GetUploadByUUID(uuid string) (*Upload, error) {
	upload := &Upload{UUID: uuid}
	has, err := x.Get(upload)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUploadNotExist{0, uuid}
	}
	return upload, nil
}

// GetUploadsByUUIDs returns multiple uploads by UUIDS
func GetUploadsByUUIDs(uuids []string) ([]*Upload, error) {
	if len(uuids) == 0 {
		return []*Upload{}, nil
	}

	// Silently drop invalid uuids.
	uploads := make([]*Upload, 0, len(uuids))
	return uploads, x.In("uuid", uuids).Find(&uploads)
}

// DeleteUploads deletes multiple uploads
func DeleteUploads(uploads ...*Upload) (err error) {
	if len(uploads) == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	ids := make([]int64, len(uploads))
	for i := 0; i < len(uploads); i++ {
		ids[i] = uploads[i].ID
	}
	if _, err = sess.
		In("id", ids).
		Delete(new(Upload)); err != nil {
		return fmt.Errorf("delete uploads: %v", err)
	}

	for _, upload := range uploads {
		localPath := upload.LocalPath()
		if !com.IsFile(localPath) {
			continue
		}

		if err := os.Remove(localPath); err != nil {
			return fmt.Errorf("remove upload: %v", err)
		}
	}

	return sess.Commit()
}

// DeleteUpload delete a upload
func DeleteUpload(u *Upload) error {
	return DeleteUploads(u)
}

// DeleteUploadByUUID deletes a upload by UUID
func DeleteUploadByUUID(uuid string) error {
	upload, err := GetUploadByUUID(uuid)
	if err != nil {
		if IsErrUploadNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetUploadByUUID: %v", err)
	}

	if err := DeleteUpload(upload); err != nil {
		return fmt.Errorf("DeleteUpload: %v", err)
	}

	return nil
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
func (repo *Repository) UploadRepoFiles(doer *User, opts UploadRepoFileOptions) (err error) {
	if len(opts.Files) == 0 {
		return nil
	}

	uploads, err := GetUploadsByUUIDs(opts.Files)
	if err != nil {
		return fmt.Errorf("GetUploadsByUUIDs [uuids: %v]: %v", opts.Files, err)
	}

	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	if err = repo.DiscardLocalRepoBranchChanges(opts.OldBranch); err != nil {
		return fmt.Errorf("DiscardLocalRepoBranchChanges [branch: %s]: %v", opts.OldBranch, err)
	} else if err = repo.UpdateLocalCopyBranch(opts.OldBranch); err != nil {
		return fmt.Errorf("UpdateLocalCopyBranch [branch: %s]: %v", opts.OldBranch, err)
	}

	if opts.OldBranch != opts.NewBranch {
		if err = repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
			return fmt.Errorf("CheckoutNewBranch [old_branch: %s, new_branch: %s]: %v", opts.OldBranch, opts.NewBranch, err)
		}
	}

	localPath := repo.LocalCopyPath()
	dirPath := path.Join(localPath, opts.TreePath)

	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dirPath, err)
	}

	// Copy uploaded files into repository.
	for _, upload := range uploads {
		tmpPath := upload.LocalPath()
		targetPath := path.Join(dirPath, upload.Name)
		if !com.IsFile(tmpPath) {
			continue
		}

		if err = com.Copy(tmpPath, targetPath); err != nil {
			return fmt.Errorf("Copy: %v", err)
		}
	}

	if err = git.AddChanges(localPath, true); err != nil {
		return fmt.Errorf("git add --all: %v", err)
	} else if err = git.CommitChanges(localPath, git.CommitChangesOptions{
		Committer: doer.NewGitSig(),
		Message:   opts.Message,
	}); err != nil {
		return fmt.Errorf("CommitChanges: %v", err)
	} else if err = git.Push(localPath, git.PushOptions{
		Remote: "origin",
		Branch: opts.NewBranch,
	}); err != nil {
		return fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error(4, "OpenRepository: %v", err)
		return nil
	}
	commit, err := gitRepo.GetBranchCommit(opts.NewBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit [branch: %s]: %v", opts.NewBranch, err)
		return nil
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}
	err = PushUpdate(
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
		return fmt.Errorf("PushUpdate: %v", err)
	}

	return DeleteUploads(uploads...)
}
