// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

//	_
// /   _  ._ _  ._ _  o _|_
// \_ (_) | | | | | | |  |_
//

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	ref, err := repo.gogitRepo.Reference(plumbing.ReferenceName(name), true)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return "", git.ErrNotExist{
				ID: name,
			}
		}
		return "", err
	}

	return ref.Hash().String(), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	hash := plumbing.NewHash(name)
	_, err := repo.gogitRepo.CommitObject(hash)
	return err == nil
}

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(name string) (string, error) {
	return repo.GetRefCommitID(git.BranchPrefix + name)
}

func convertPGPSignatureForTag(t *object.Tag) *service.GPGSignature {
	if t.PGPSignature == "" {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w,
		"object %s\ntype %s\ntag %s\ntagger ",
		t.Target.String(), t.TargetType.Bytes(), t.Name); err != nil {
		return nil
	}

	if err = t.Tagger.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n"); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, t.Message); err != nil {
		return nil
	}

	return &service.GPGSignature{
		Signature: t.PGPSignature,
		Payload:   strings.TrimSpace(w.String()) + "\n",
	}
}

func (repo *Repository) getCommit(id service.Hash) (service.Commit, error) {
	var tagObject *object.Tag
	plumbingHash := ToPlumbingHash(id)

	gogitCommit, err := repo.gogitRepo.CommitObject(plumbingHash)
	if err == plumbing.ErrObjectNotFound {
		tagObject, err = repo.gogitRepo.TagObject(plumbingHash)
		if err == plumbing.ErrObjectNotFound {
			return nil, git.ErrNotExist{
				ID: id.String(),
			}
		}
		if err == nil {
			gogitCommit, err = repo.gogitRepo.CommitObject(tagObject.Target)
		}
		// if we get a plumbing.ErrObjectNotFound here then the repository is broken and it should be 500
	}
	if err != nil {
		return nil, err
	}

	commit := convertCommit(repo, gogitCommit)
	nativeCommit := commit.(*native.Commit)

	if tagObject != nil {
		nativeCommit.SetMessage(strings.TrimSpace(tagObject.Message))
		nativeCommit.SetAuthor(convertSignature(&tagObject.Tagger))
		nativeCommit.SetSignature(convertPGPSignatureForTag(tagObject))
	}

	gogitTree, err := gogitCommit.Tree()
	if err != nil {
		return nil, err
	}

	nativeCommit.SetTree(&Tree{
		Object: Object{
			hash: fromPlumbingHash(gogitTree.Hash),
			repo: repo,
		},
		gogitTree:  gogitTree,
		resolvedID: id,
	})

	return nativeCommit, nil
}

// GetTagCommitID returns last commit ID string of given tag.
// FIXME: This is the same as the native variant
func (repo *Repository) GetTagCommitID(name string) (string, error) {
	stdout, err := git.NewCommand("rev-list", "-n", "1", git.TagPrefix+name).RunInDir(repo.Path())
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision or path") {
			return "", git.ErrNotExist{
				ID:      name,
				RelPath: "",
			}
		}
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

// ConvertToSHA1 returns a Hash object from a potential ID string
// FIXME: This is the same as the native variant
func (repo *Repository) ConvertToSHA1(commitID string) (service.Hash, error) {
	if len(commitID) != 40 {
		var err error
		actualCommitID, err := git.NewCommand("rev-parse", "--verify", commitID).RunInDir(repo.Path())
		if err != nil {
			if strings.Contains(err.Error(), "unknown revision or path") ||
				strings.Contains(err.Error(), "fatal: Needed a single revision") {
				return StringHash(""), git.ErrNotExist{
					ID:      commitID,
					RelPath: "",
				}
			}
			return StringHash(""), err
		}
		commitID = actualCommitID
	}
	return StringHash(commitID), nil
}

// GetCommit returns commit object of by ID string.
func (repo *Repository) GetCommit(commitID string) (service.Commit, error) {
	id, err := repo.ConvertToSHA1(commitID)
	if err != nil {
		return nil, err
	}

	return repo.getCommit(id)
}

// GetBranchCommit returns the last commit of given branch.
func (repo *Repository) GetBranchCommit(name string) (service.Commit, error) {
	commitID, err := repo.GetBranchCommitID(name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(commitID)
}

// GetTagCommit get the commit of the specific tag via name
func (repo *Repository) GetTagCommit(name string) (service.Commit, error) {
	commitID, err := repo.GetTagCommitID(name)
	if err != nil {
		return nil, err
	}
	return repo.GetCommit(commitID)
}

// GetCommitByPath returns the last commit of relative path.
// FIXME: This is the same as the native variant
func (repo *Repository) GetCommitByPath(relpath string) (service.Commit, error) {
	// File name starts with ':' must be escaped.
	if relpath[0] == ':' {
		relpath = `\` + relpath
	}

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(
		git.NewCommand("log", "-1", "--pretty=raw", "--", relpath),
		repo.Path(),
		stdoutWriter,
		nil)

	bufReader := bufio.NewReader(stdoutReader)
	_, _ = bufReader.Discard(7)
	idStr, err := bufReader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	idStr = idStr[:len(idStr)-1]

	return native.CommitFromReader(repo, StringHash(idStr), bufReader)
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	var errbuf strings.Builder
	if err := git.NewCommand("log", "-1").RunInDirPipeline(repo.Path(), nil, &errbuf); err != nil {
		if strings.Contains(errbuf.String(), "fatal: bad default revision 'HEAD'") ||
			strings.Contains(errbuf.String(), "fatal: your current branch 'master' does not have any commits yet") {
			return true, nil
		}
		return true, fmt.Errorf("check empty: %v - %s", err, errbuf.String())
	}

	return false, nil
}
