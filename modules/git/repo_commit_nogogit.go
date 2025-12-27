// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"errors"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/catfile"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, _, err := gitcmd.NewCommand("show-ref", "--hash").
		AddDynamicArguments(name).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", ErrNotExist{name, ""}
	}

	return stdout, nil
}

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	batch, cancel, err := repo.CatFileBatchCheck(repo.Ctx)
	if err != nil {
		return "", err
	}
	defer cancel()
	_, err = batch.Writer().Write([]byte(name + "\n"))
	if err != nil {
		return "", err
	}
	shaBs, _, _, err := ReadBatchLine(batch.Reader())
	if IsErrNotExist(err) {
		return "", ErrNotExist{name, ""}
	}

	return string(shaBs), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	if err := catfile.EnsureValidGitRepository(repo.Ctx, repo.Path); err != nil {
		log.Error("IsCommitExist: %v", err)
		return false
	}
	_, _, err := gitcmd.NewCommand("cat-file", "-e").
		AddDynamicArguments(name).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	return err == nil
}

func (repo *Repository) getCommit(id ObjectID) (*Commit, error) {
	batch, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	_, _ = batch.Writer().Write([]byte(id.String() + "\n"))

	return repo.getCommitFromBatchReader(batch, id)
}

func (repo *Repository) getCommitFromBatchReader(batch catfile.Batch, id ObjectID) (*Commit, error) {
	rd := batch.Reader()
	_, typ, size, err := ReadBatchLine(rd)
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch typ {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := io.ReadAll(io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(id.Type(), data)
		if err != nil {
			return nil, err
		}

		if _, err := batch.Writer().Write([]byte(tag.Object.String() + "\n")); err != nil {
			return nil, err
		}

		commit, err := repo.getCommitFromBatchReader(batch, tag.Object)
		if err != nil {
			return nil, err
		}

		return commit, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, io.LimitReader(rd, size))
		if err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		log.Debug("Unknown typ: %s", typ)
		if err := DiscardFull(rd, size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToGitID returns a GitHash object from a potential ID string
func (repo *Repository) ConvertToGitID(commitID string) (ObjectID, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	if len(commitID) == objectFormat.FullLength() && objectFormat.IsValid(commitID) {
		ID, err := NewIDFromString(commitID)
		if err == nil {
			return ID, nil
		}
	}

	batch, cancel, err := repo.CatFileBatchCheck(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	_, err = batch.Writer().Write([]byte(commitID + "\n"))
	if err != nil {
		return nil, err
	}
	sha, _, _, err := ReadBatchLine(batch.Reader())
	if err != nil {
		if IsErrNotExist(err) {
			return nil, ErrNotExist{commitID, ""}
		}
		return nil, err
	}

	return MustIDFromString(string(sha)), nil
}
