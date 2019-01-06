// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"

	"github.com/mcuadros/go-version"
)

// TagPrefix tags prefix path on the repository
const TagPrefix = "refs/tags/"

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(repoPath, name string) bool {
	return IsReferenceExist(repoPath, TagPrefix+name)
}

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	return IsTagExist(repo.Path, name)
}

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, err := NewCommand("tag", name, revision).RunInDir(repo.Path)
	return err
}

func (repo *Repository) getTag(id SHA1) (*Tag, error) {
	t, ok := repo.tagCache.Get(id.String())
	if ok {
		log("Hit cache: %s", id)
		return t.(*Tag), nil
	}

	// Get tag type
	tp, err := NewCommand("cat-file", "-t", id.String()).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}
	tp = strings.TrimSpace(tp)

	// Tag is a commit.
	if ObjectType(tp) == ObjectCommit {
		tag := &Tag{
			ID:     id,
			Object: id,
			Type:   string(ObjectCommit),
			repo:   repo,
		}

		repo.tagCache.Set(id.String(), tag)
		return tag, nil
	}

	// Tag with message.
	data, err := NewCommand("cat-file", "-p", id.String()).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	tag, err := parseTagData(data)
	if err != nil {
		return nil, err
	}

	tag.ID = id
	tag.repo = repo

	repo.tagCache.Set(id.String(), tag)
	return tag, nil
}

// GetTag returns a Git tag by given name.
func (repo *Repository) GetTag(name string) (*Tag, error) {
	stdout, err := NewCommand("show-ref", "--tags", name).RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	id, err := NewIDFromString(strings.Split(stdout, " ")[0])
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id)
	if err != nil {
		return nil, err
	}
	tag.Name = name
	return tag, nil
}

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos() ([]*Tag, error) {
	// TODO this a slow implementation, makes one git command per tag
	stdout, err := NewCommand("tag").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	tagNames := strings.Split(stdout, "\n")
	var tags []*Tag
	for _, tagName := range tagNames {
		tagName = strings.TrimSpace(tagName)
		if len(tagName) == 0 {
			continue
		}
		commitID, err := NewCommand("rev-parse", tagName).RunInDir(repo.Path)
		if err != nil {
			return nil, err
		}
		commit, err := repo.GetCommit(commitID)
		if err != nil {
			return nil, err
		}
		tags = append(tags, &Tag{
			Name:    tagName,
			Message: commit.Message(),
			Object:  commit.ID,
			Tagger:  commit.Author,
		})
	}
	sortTagsByTime(tags)
	return tags, nil
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	cmd := NewCommand("tag", "-l")
	if version.Compare(gitVersion, "2.0.0", ">=") {
		cmd.AddArguments("--sort=-v:refname")
	}

	stdout, err := cmd.RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	tags := strings.Split(stdout, "\n")
	tags = tags[:len(tags)-1]

	if version.Compare(gitVersion, "2.0.0", "<") {
		version.Sort(tags)

		// Reverse order
		for i := 0; i < len(tags)/2; i++ {
			j := len(tags) - i - 1
			tags[i], tags[j] = tags[j], tags[i]
		}
	}

	return tags, nil
}
