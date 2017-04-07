// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/mcuadros/go-version"
)

// Commit represents a git commit.
type Commit struct {
	Tree
	ID            SHA1 // The ID of this commit object
	Author        *Signature
	Committer     *Signature
	CommitMessage string
	Signature     *CommitGPGSignature

	parents        []SHA1 // SHA1 strings
	submoduleCache *ObjectCache
}

// CommitGPGSignature represents a git commit signature part.
type CommitGPGSignature struct {
	Signature string
	Payload   string //TODO check if can be reconstruct from the rest of commit information to not have duplicate data
}

// similar to https://github.com/git/git/blob/3bc53220cb2dcf709f7a027a3f526befd021d858/commit.c#L1128
func newGPGSignatureFromCommitline(data []byte, signatureStart int) (*CommitGPGSignature, error) {
	sig := new(CommitGPGSignature)
	signatureEnd := bytes.LastIndex(data, []byte("-----END PGP SIGNATURE-----"))
	if signatureEnd == -1 {
		return nil, fmt.Errorf("end of commit signature not found")
	}
	sig.Signature = strings.Replace(string(data[signatureStart:signatureEnd+27]), "\n ", "\n", -1)
	sig.Payload = string(data[:signatureStart-8]) + string(data[signatureEnd+27:])
	return sig, nil
}

// Message returns the commit message. Same as retrieving CommitMessage directly.
func (c *Commit) Message() string {
	return c.CommitMessage
}

// Summary returns first line of commit message.
func (c *Commit) Summary() string {
	return strings.Split(c.CommitMessage, "\n")[0]
}

// ParentID returns oid of n-th parent (0-based index).
// It returns nil if no such parent exists.
func (c *Commit) ParentID(n int) (SHA1, error) {
	if n >= len(c.parents) {
		return SHA1{}, ErrNotExist{"", ""}
	}
	return c.parents[n], nil
}

// Parent returns n-th parent (0-based index) of the commit.
func (c *Commit) Parent(n int) (*Commit, error) {
	id, err := c.ParentID(n)
	if err != nil {
		return nil, err
	}
	parent, err := c.repo.getCommit(id)
	if err != nil {
		return nil, err
	}
	return parent, nil
}

// ParentCount returns number of parents of the commit.
// 0 if this is the root commit,  otherwise 1,2, etc.
func (c *Commit) ParentCount() int {
	return len(c.parents)
}

func isImageFile(data []byte) (string, bool) {
	contentType := http.DetectContentType(data)
	if strings.Index(contentType, "image/") != -1 {
		return contentType, true
	}
	return contentType, false
}

// IsImageFile is a file image type
func (c *Commit) IsImageFile(name string) bool {
	blob, err := c.GetBlobByPath(name)
	if err != nil {
		return false
	}

	dataRc, err := blob.Data()
	if err != nil {
		return false
	}
	buf := make([]byte, 1024)
	n, _ := dataRc.Read(buf)
	buf = buf[:n]
	_, isImage := isImageFile(buf)
	return isImage
}

// GetCommitByPath return the commit of relative path object.
func (c *Commit) GetCommitByPath(relpath string) (*Commit, error) {
	return c.repo.getCommitByPathWithID(c.ID, relpath)
}

// AddChanges marks local changes to be ready for commit.
func AddChanges(repoPath string, all bool, files ...string) error {
	cmd := NewCommand("add")
	if all {
		cmd.AddArguments("--all")
	}
	_, err := cmd.AddArguments(files...).RunInDir(repoPath)
	return err
}

// CommitChangesOptions the options when a commit created
type CommitChangesOptions struct {
	Committer *Signature
	Author    *Signature
	Message   string
}

// CommitChanges commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChanges(repoPath string, opts CommitChangesOptions) error {
	cmd := NewCommand()
	if opts.Committer != nil {
		cmd.AddArguments("-c", "user.name="+opts.Committer.Name, "-c", "user.email="+opts.Committer.Email)
	}
	cmd.AddArguments("commit")

	if opts.Author == nil {
		opts.Author = opts.Committer
	}
	if opts.Author != nil {
		cmd.AddArguments(fmt.Sprintf("--author='%s <%s>'", opts.Author.Name, opts.Author.Email))
	}
	cmd.AddArguments("-m", opts.Message)

	_, err := cmd.RunInDir(repoPath)
	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

func commitsCount(repoPath, revision, relpath string) (int64, error) {
	var cmd *Command
	isFallback := false
	if version.Compare(gitVersion, "1.8.0", "<") {
		isFallback = true
		cmd = NewCommand("log", "--pretty=format:''")
	} else {
		cmd = NewCommand("rev-list", "--count")
	}
	cmd.AddArguments(revision)
	if len(relpath) > 0 {
		cmd.AddArguments("--", relpath)
	}

	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return 0, err
	}

	if isFallback {
		return int64(strings.Count(stdout, "\n")) + 1, nil
	}
	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(repoPath, revision string) (int64, error) {
	return commitsCount(repoPath, revision, "")
}

// CommitsCount returns number of total commits of until current revision.
func (c *Commit) CommitsCount() (int64, error) {
	return CommitsCount(c.repo.Path, c.ID.String())
}

// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
func (c *Commit) CommitsByRange(page int) (*list.List, error) {
	return c.repo.commitsByRange(c.ID, page)
}

// CommitsBefore returns all the commits before current revision
func (c *Commit) CommitsBefore() (*list.List, error) {
	return c.repo.getCommitsBefore(c.ID)
}

// CommitsBeforeLimit returns num commits before current revision
func (c *Commit) CommitsBeforeLimit(num int) (*list.List, error) {
	return c.repo.getCommitsBeforeLimit(c.ID, num)
}

// CommitsBeforeUntil returns the commits between commitID to current revision
func (c *Commit) CommitsBeforeUntil(commitID string) (*list.List, error) {
	endCommit, err := c.repo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	return c.repo.CommitsBetween(c, endCommit)
}

// SearchCommits returns the commits match the keyword before current revision
func (c *Commit) SearchCommits(keyword string, all bool) (*list.List, error) {
	return c.repo.searchCommits(c.ID, keyword, all)
}

// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
func (c *Commit) GetFilesChangedSinceCommit(pastCommit string) ([]string, error) {
	return c.repo.getFilesChanged(pastCommit, c.ID.String())
}

// GetSubModules get all the sub modules of current revision git tree
func (c *Commit) GetSubModules() (*ObjectCache, error) {
	if c.submoduleCache != nil {
		return c.submoduleCache, nil
	}

	entry, err := c.GetTreeEntryByPath(".gitmodules")
	if err != nil {
		return nil, err
	}
	rd, err := entry.Blob().Data()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(rd)
	c.submoduleCache = newObjectCache()
	var ismodule bool
	var path string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "[submodule") {
			ismodule = true
			continue
		}
		if ismodule {
			fields := strings.Split(scanner.Text(), "=")
			k := strings.TrimSpace(fields[0])
			if k == "path" {
				path = strings.TrimSpace(fields[1])
			} else if k == "url" {
				c.submoduleCache.Set(path, &SubModule{path, strings.TrimSpace(fields[1])})
				ismodule = false
			}
		}
	}

	return c.submoduleCache, nil
}

// GetSubModule get the sub module according entryname
func (c *Commit) GetSubModule(entryname string) (*SubModule, error) {
	modules, err := c.GetSubModules()
	if err != nil {
		return nil, err
	}

	module, has := modules.Get(entryname)
	if has {
		return module.(*SubModule), nil
	}
	return nil, nil
}
