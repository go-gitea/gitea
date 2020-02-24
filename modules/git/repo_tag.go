// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"

	"github.com/mcuadros/go-version"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// TagPrefix tags prefix path on the repository
const TagPrefix = "refs/tags/"

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(repoPath, name string) bool {
	return IsReferenceExist(repoPath, TagPrefix+name)
}

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	_, err := repo.gogitRepo.Reference(plumbing.ReferenceName(TagPrefix+name), true)
	return err == nil
}

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, err := NewCommand("tag", "--", name, revision).RunInDir(repo.Path)
	return err
}

// CreateAnnotatedTag create one annotated tag in the repository
func (repo *Repository) CreateAnnotatedTag(name, message, revision string) error {
	_, err := NewCommand("tag", "-a", "-m", message, "--", name, revision).RunInDir(repo.Path)
	return err
}

func (repo *Repository) getTag(id SHA1) (*Tag, error) {
	t, ok := repo.tagCache.Get(id.String())
	if ok {
		log("Hit cache: %s", id)
		tagClone := *t.(*Tag)
		return &tagClone, nil
	}

	// Get tag name
	name, err := repo.GetTagNameBySHA(id.String())
	if err != nil {
		return nil, err
	}

	tp, err := repo.GetTagType(id)
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

	// tagID defaults to the commit ID as the tag ID and then tries to get a tag ID (only annotated tags)
	tagID := commitID
	if tagIDStr, err := repo.GetTagID(name); err != nil {
		// if the err is NotExist then we can ignore and just keep tagID as ID (is lightweight tag)
		// all other errors we return
		if !IsErrNotExist(err) {
			return nil, err
		}
	} else {
		tagID, err = NewIDFromString(tagIDStr)
		if err != nil {
			return nil, err
		}
	}

	// If type is "commit, the tag is a lightweight tag
	if ObjectType(tp) == ObjectCommit {
		commit, err := repo.GetCommit(id.String())
		if err != nil {
			return nil, err
		}
		tag := &Tag{
			Name:    name,
			ID:      tagID,
			Object:  commitID,
			Type:    string(ObjectCommit),
			Tagger:  commit.Committer,
			Message: commit.Message(),
			repo:    repo,
		}

		repo.tagCache.Set(id.String(), tag)
		return tag, nil
	}

	// The tag is an annotated tag with a message.
	data, err := NewCommand("cat-file", "-p", id.String()).RunInDirBytes(repo.Path)
	if err != nil {
		return nil, err
	}

	tag, err := parseTagData(data)
	if err != nil {
		return nil, err
	}

	tag.Name = name
	tag.ID = id
	tag.repo = repo
	tag.Type = tp

	repo.tagCache.Set(id.String(), tag)
	return tag, nil
}

// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
func (repo *Repository) GetTagNameBySHA(sha string) (string, error) {
	if len(sha) < 5 {
		return "", fmt.Errorf("SHA is too short: %s", sha)
	}

	stdout, err := NewCommand("show-ref", "--tags", "-d").RunInDir(repo.Path)
	if err != nil {
		return "", err
	}

	tagRefs := strings.Split(stdout, "\n")
	for _, tagRef := range tagRefs {
		if len(strings.TrimSpace(tagRef)) > 0 {
			fields := strings.Fields(tagRef)
			if strings.HasPrefix(fields[0], sha) && strings.HasPrefix(fields[1], TagPrefix) {
				name := fields[1][len(TagPrefix):]
				// annotated tags show up twice, we should only return if is not the ^{} ref
				if !strings.HasSuffix(name, "^{}") {
					return name, nil
				}
			}
		}
	}
	return "", ErrNotExist{ID: sha}
}

// GetTagID returns the object ID for a tag (annotated tags have both an object SHA AND a commit SHA)
func (repo *Repository) GetTagID(name string) (string, error) {
	stdout, err := NewCommand("show-ref", "--tags", "--", name).RunInDir(repo.Path)
	if err != nil {
		return "", err
	}
	// Make sure exact match is used: "v1" != "release/v1"
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "refs/tags/"+name {
			return fields[0], nil
		}
	}
	return "", ErrNotExist{ID: name}
}

// GetTag returns a Git tag by given name.
func (repo *Repository) GetTag(name string) (*Tag, error) {
	idStr, err := repo.GetTagID(name)
	if err != nil {
		return nil, err
	}

	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos() ([]*Tag, error) {
	// TODO this a slow implementation, makes one git command per tag
	stdout, err := NewCommand("tag").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	tagNames := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	var tags = make([]*Tag, 0, len(tagNames))
	for _, tagName := range tagNames {
		tagName = strings.TrimSpace(tagName)
		if len(tagName) == 0 {
			continue
		}

		tag, err := repo.GetTag(tagName)
		if err != nil {
			return nil, err
		}
		tag.Name = tagName
		tags = append(tags, tag)
	}
	sortTagsByTime(tags)
	return tags, nil
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	var tagNames []string

	tags, err := repo.gogitRepo.Tags()
	if err != nil {
		return nil, err
	}

	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name().String(), TagPrefix))
		return nil
	})

	version.Sort(tagNames)

	// Reverse order
	for i := 0; i < len(tagNames)/2; i++ {
		j := len(tagNames) - i - 1
		tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
	}

	return tagNames, nil
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id SHA1) (string, error) {
	// Get tag type
	stdout, err := NewCommand("cat-file", "-t", id.String()).RunInDir(repo.Path)
	if err != nil {
		return "", err
	}
	if len(stdout) == 0 {
		return "", ErrNotExist{ID: id.String()}
	}
	return strings.TrimSpace(stdout), nil
}

// GetAnnotatedTag returns a Git tag by its SHA, must be an annotated tag
func (repo *Repository) GetAnnotatedTag(sha string) (*Tag, error) {
	id, err := NewIDFromString(sha)
	if err != nil {
		return nil, err
	}

	// Tag type must be "tag" (annotated) and not a "commit" (lightweight) tag
	if tagType, err := repo.GetTagType(id); err != nil {
		return nil, err
	} else if ObjectType(tagType) != ObjectTag {
		// not an annotated tag
		return nil, ErrNotExist{ID: id.String()}
	}

	tag, err := repo.getTag(id)
	if err != nil {
		return nil, err
	}
	return tag, nil
}
