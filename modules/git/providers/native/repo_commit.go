// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

//	_
// /   _  ._ _  ._ _  o _|_
// \_ (_) | | | | | | |  |_
//

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	stdout, err := git.NewCommand("show-ref", "--verify", "--hash", name).RunInDir(repo.Path())
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", git.ErrNotExist{
				ID:      name,
				RelPath: "",
			}
		}
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	_, err := git.NewCommand("cat-file", "-e", name).RunInDir(repo.Path())
	return err == nil
}

// GetBranchCommitID returns last commit ID string of given branch.
func (repo *Repository) GetBranchCommitID(name string) (string, error) {
	return repo.GetRefCommitID(git.BranchPrefix + name)
}

// GetTagCommitID returns last commit ID string of given tag.
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
		commitID = actualCommitID[:len(actualCommitID)-1]
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

func (repo *Repository) getCommit(id service.Hash) (*Commit, error) {

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(
		git.NewCommand("cat-file", "--batch"),
		repo.Path(),
		stdoutWriter,
		stdinReader)

	_, err := stdinWriter.Write([]byte(id.String() + "\n"))
	if err != nil {
		return nil, err
	}

	bufReader := bufio.NewReader(stdoutReader)
	_, typ, size, err := ReadBatchLine(bufReader)
	if err != nil {
		return nil, err
	}

	switch typ {
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := ioutil.ReadAll(io.LimitReader(bufReader, size))
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(data)
		if err != nil {
			return nil, err
		}
		tag.Object = &Object{
			hash: id,
			repo: repo,
		}

		if tag.TagType() != string(service.ObjectCommit) {
			return nil, fmt.Errorf("Unexpected tag type: %s", tag.TagType())
		}

		_, err = stdinWriter.Write([]byte(tag.TagObject().String() + "\n"))
		if err != nil {
			return nil, err
		}
		_, typ, size, err := ReadBatchLine(bufReader)
		if err != nil {
			return nil, err
		}
		if typ != string(service.ObjectCommit) {
			return nil, fmt.Errorf("Unexpected tag type: %s", typ)
		}

		commit, err := CommitFromReader(repo, id, io.LimitReader(bufReader, size))
		if err != nil {
			return nil, err
		}

		commit.message = strings.TrimSpace(tag.message)
		commit.author = tag.tagger
		commit.signature = tag.gpgSignature

		return commit, nil
	case "commit":
		return CommitFromReader(repo, id, io.LimitReader(bufReader, size))
	default:
		_ = stdoutReader.CloseWithError(fmt.Errorf("unknown typ: %s", typ))
		log.Error("Unknown git typ: %s for ID: %s", typ, id.String())
		return nil, git.ErrNotExist{
			ID: id.String(),
		}
	}
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
