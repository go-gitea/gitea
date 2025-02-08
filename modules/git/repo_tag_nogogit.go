// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"errors"
	"io"

	"code.gitea.io/gitea/modules/log"
)

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	if repo == nil || name == "" {
		return false
	}

	return repo.IsReferenceExist(TagPrefix + name)
}

// GetTags returns all tags of the repository.
// returning at most limit tags, or all if limit is 0.
func (repo *Repository) GetTags(skip, limit int) (tags []string, err error) {
	tags, _, err = callShowRef(repo.Ctx, repo.Path, TagPrefix, TrustedCmdArgs{TagPrefix, "--sort=-taggerdate"}, skip, limit)
	return tags, err
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id ObjectID) (string, error) {
	wr, rd, cancel, err := repo.CatFileBatchCheck(repo.Ctx)
	if err != nil {
		return "", err
	}
	defer cancel()
	_, err = wr.Write([]byte(id.String() + "\n"))
	if err != nil {
		return "", err
	}
	_, typ, _, err := ReadBatchLine(rd)
	if IsErrNotExist(err) {
		return "", ErrNotExist{ID: id.String()}
	}
	return typ, nil
}

func (repo *Repository) getTag(tagID ObjectID, name string) (*Tag, error) {
	t, ok := repo.tagCache.Get(tagID.String())
	if ok {
		log.Debug("Hit cache: %s", tagID)
		tagClone := *t
		tagClone.Name = name // This is necessary because lightweight tags may have same id
		return &tagClone, nil
	}

	tp, err := repo.GetTagType(tagID)
	if err != nil {
		return nil, err
	}

	// Get the commit ID and tag ID (may be different for annotated tag) for the returned tag object
	commitIDStr, err := repo.GetTagCommitID(name)
	if err != nil {
		// every tag should have a commit ID so return all errors
		return nil, err
	}
	commitID, err := NewIDFromString(commitIDStr)
	if err != nil {
		return nil, err
	}

	// If type is "commit, the tag is a lightweight tag
	if ObjectType(tp) == ObjectCommit {
		commit, err := repo.GetCommit(commitIDStr)
		if err != nil {
			return nil, err
		}
		tag := &Tag{
			Name:    name,
			ID:      tagID,
			Object:  commitID,
			Type:    tp,
			Tagger:  commit.Committer,
			Message: commit.Message(),
		}

		repo.tagCache.Set(tagID.String(), tag)
		return tag, nil
	}

	// The tag is an annotated tag with a message.
	wr, rd, cancel, err := repo.CatFileBatch(repo.Ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	if _, err := wr.Write([]byte(tagID.String() + "\n")); err != nil {
		return nil, err
	}
	_, typ, size, err := ReadBatchLine(rd)
	if err != nil {
		if errors.Is(err, io.EOF) || IsErrNotExist(err) {
			return nil, ErrNotExist{ID: tagID.String()}
		}
		return nil, err
	}
	if typ != "tag" {
		if err := DiscardFull(rd, size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{ID: tagID.String()}
	}

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

	tag, err := parseTagData(tagID.Type(), data)
	if err != nil {
		return nil, err
	}

	tag.Name = name
	tag.ID = tagID
	tag.Type = tp

	repo.tagCache.Set(tagID.String(), tag)
	return tag, nil
}
