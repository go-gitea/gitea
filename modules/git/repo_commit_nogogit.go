// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"errors"
	"io"

	"gitea.dev/modules/setting"
)

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(ctx context.Context, name string) (string, error) {
	batch, cancel, err := repo.CatFileBatch()
	if err != nil {
		return "", err
	}
	defer cancel()
	info, err := batch.QueryInfo(name)
	if IsErrNotExist(err) {
		return "", ErrNotExist{name, ""}
	} else if err != nil {
		return "", err
	}
	return info.ID, nil
}

func (repo *Repository) getCommit(ctx context.Context, id ObjectID) (*Commit, error) {
	batch, cancel, err := repo.CatFileBatch()
	if err != nil {
		return nil, err
	}
	defer cancel()
	return repo.getCommitWithBatch(batch, id)
}

func limitDiscardReader(rd BufferedReader, full, limit int64) (io.Reader, func() error) {
	return io.LimitReader(rd, min(full, limit)), func() error {
		if full > limit {
			return DiscardFull(rd, full-limit)
		}
		return nil
	}
}

func (repo *Repository) getCommitWithBatch(batch CatFileBatch, id ObjectID) (*Commit, error) {
	info, rd, err := batch.QueryContent(id.String())
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch info.Type {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		limitReader, limitDiscard := limitDiscardReader(rd, info.Size, MaxGitObjectSize)
		data, err := io.ReadAll(limitReader)
		if err != nil {
			return nil, err
		}
		if err = limitDiscard(); err != nil {
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
		return repo.getCommitWithBatch(batch, tag.Object)
	case "commit":
		limitReader, limitDiscard := limitDiscardReader(rd, info.Size, MaxGitObjectSize)
		commit, err := CommitFromReader(id, limitReader)
		if err != nil {
			return nil, err
		}
		if err = limitDiscard(); err != nil {
			return nil, err
		}
		_, err = rd.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		if info.Type != "blob" && info.Type != "tree" {
			setting.PanicInDevOrTesting("Unknown cat-file object type %s for object %s in repo %s", info.Type, id.String(), repo.LogString())
		}
		if err := DiscardFull(rd, info.Size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToGitID returns a git object ID from the git ref, it doesn't guarantee the returned ID really exists
func (repo *Repository) ConvertToGitID(ctx context.Context, ref string) (ObjectID, error) {
	objectFormat, err := repo.GetObjectFormat(ctx)
	if err != nil {
		return nil, err
	}
	if len(ref) == objectFormat.FullLength() && objectFormat.IsValid(ref) {
		id, err := NewIDFromString(ref)
		if err == nil {
			return id, nil
		}
	}

	batch, cancel, err := repo.CatFileBatch()
	if err != nil {
		return nil, err
	}
	defer cancel()
	info, err := batch.QueryInfo(ref)
	if err != nil {
		if IsErrNotExist(err) {
			return nil, ErrNotExist{ref, ""}
		}
		return nil, err
	}

	return MustIDFromString(info.ID), nil
}
