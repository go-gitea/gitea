// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-git/go-git/v5/plumbing"
)

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	_, err := repo.gogitRepo.Reference(plumbing.ReferenceName(TagPrefix+name), true)
	return err == nil
}

// GetTags returns all tags of the repository.
// returning at most limit tags, or all if limit is 0.
func (repo *Repository) GetTags(skip, limit int) ([]string, error) {
	var tagNames []string

	tags, err := repo.gogitRepo.Tags()
	if err != nil {
		return nil, err
	}

	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name().String(), TagPrefix))
		return nil
	})

	// Reverse order
	for i := 0; i < len(tagNames)/2; i++ {
		j := len(tagNames) - i - 1
		tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
	}

	// since we have to reverse order we can paginate only afterwards
	if len(tagNames) < skip {
		tagNames = []string{}
	} else {
		tagNames = tagNames[skip:]
	}
	if limit != 0 && len(tagNames) > limit {
		tagNames = tagNames[:limit]
	}

	return tagNames, nil
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id ObjectID) (string, error) {
	// Get tag type
	obj, err := repo.gogitRepo.Object(plumbing.AnyObject, plumbing.Hash(id.RawValue()))
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return "", &ErrNotExist{ID: id.String()}
		}
		return "", err
	}

	return obj.Type().String(), nil
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

	gogitTag, err := repo.gogitRepo.TagObject(plumbing.Hash(tagID.RawValue()))
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil, &ErrNotExist{ID: tagID.String()}
		}

		return nil, err
	}

	tag := &Tag{
		Name:    name,
		ID:      tagID,
		Object:  commitID.Type().MustID(gogitTag.Target[:]),
		Type:    tp,
		Tagger:  &gogitTag.Tagger,
		Message: gogitTag.Message,
	}

	repo.tagCache.Set(tagID.String(), tag)
	return tag, nil
}
