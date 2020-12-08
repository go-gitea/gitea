// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
)

// ___
//  |   _.  _
//  |  (_| (_|
//          _|

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	return git.IsReferenceExist(repo.Path(), git.TagPrefix+name)
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	return callShowRef(repo.Path(), git.TagPrefix, "--tags")
}

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, err := git.NewCommand("tag", "--", name, revision).RunInDir(repo.Path())
	return err
}

// CreateAnnotatedTag create one annotated tag in the repository
func (repo *Repository) CreateAnnotatedTag(name, message, revision string) error {
	_, err := git.NewCommand("tag", "-a", "-m", message, "--", name, revision).RunInDir(repo.Path())
	return err
}

// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
func (repo *Repository) GetTagNameBySHA(sha string) (string, error) {
	if len(sha) < 5 {
		return "", fmt.Errorf("SHA is too short: %s", sha)
	}

	stdout, err := git.NewCommand("show-ref", "--tags", "-d").RunInDir(repo.Path())
	if err != nil {
		return "", err
	}

	tagRefs := strings.Split(stdout, "\n")
	for _, tagRef := range tagRefs {
		if len(strings.TrimSpace(tagRef)) > 0 {
			fields := strings.Fields(tagRef)
			if strings.HasPrefix(fields[0], sha) && strings.HasPrefix(fields[1], git.TagPrefix) {
				name := fields[1][len(git.TagPrefix):]
				// annotated tags show up twice, we should only return if is not the ^{} ref
				if !strings.HasSuffix(name, "^{}") {
					return name, nil
				}
			}
		}
	}
	return "", git.ErrNotExist{ID: sha}
}

// GetTagID returns the object ID for a tag (annotated tags have both an object SHA AND a commit SHA)
func (repo *Repository) GetTagID(name string) (string, error) {
	stdout, err := git.NewCommand("show-ref", "--tags", "--", name).RunInDir(repo.Path())
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
	return "", git.ErrNotExist{ID: name}
}

// GetTag returns a Git tag by given name.
func (repo *Repository) GetTag(name string) (service.Tag, error) {
	idStr, err := repo.GetTagID(name)
	if err != nil {
		return nil, err
	}

	id := StringHash(idStr)

	tag, err := repo.getTag(id)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos(page, pageSize int) ([]service.Tag, error) {
	// FIXME: this a slow implementation, makes one git command per tag
	stdout, err := git.NewCommand("tag").RunInDir(repo.Path())
	if err != nil {
		return nil, err
	}

	tagNames := strings.Split(strings.TrimRight(stdout, "\n"), "\n")

	if page != 0 {
		skip := (page - 1) * pageSize
		if skip >= len(tagNames) {
			return nil, nil
		}
		if (len(tagNames) - skip) < pageSize {
			pageSize = len(tagNames) - skip
		}
		tagNames = tagNames[skip : skip+pageSize]
	}

	var tags = make([]service.Tag, 0, len(tagNames))
	for _, tagName := range tagNames {
		tagName = strings.TrimSpace(tagName)
		if len(tagName) == 0 {
			continue
		}

		tag, err := repo.GetTag(tagName)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	common.SortTagsByTime(tags)
	return tags, nil
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id service.Hash) (string, error) {
	// Get tag type
	stdout, err := git.NewCommand("cat-file", "-t", id.String()).RunInDir(repo.Path())
	if err != nil {
		return "", err
	}
	if len(stdout) == 0 {
		return "", git.ErrNotExist{ID: id.String()}
	}
	return strings.TrimSpace(stdout), nil
}

// GetAnnotatedTag returns a Git tag by its SHA, must be an annotated tag
func (repo *Repository) GetAnnotatedTag(sha string) (service.Tag, error) {
	id, err := StringHash("").FromString(sha)
	if err != nil {
		return nil, err
	}

	// Tag type must be "tag" (annotated) and not a "commit" (lightweight) tag
	if tagType, err := repo.GetTagType(id); err != nil {
		return nil, err
	} else if service.ObjectType(tagType) != service.ObjectTag {
		// not an annotated tag
		return nil, git.ErrNotExist{ID: id.String()}
	}

	tag, err := repo.getTag(id)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (repo *Repository) getTag(id service.Hash) (service.Tag, error) {
	idStr := id.String()

	// Get tag name
	name, err := repo.GetTagNameBySHA(idStr)
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
	commitID := StringHash(commitIDStr)

	// tagID defaults to the commit ID as the tag ID and then tries to get a tag ID (only annotated tags)
	tagID := commitID
	if tagIDStr, err := repo.GetTagID(name); err != nil {
		// if the err is NotExist then we can ignore and just keep tagID as ID (is lightweight tag)
		// all other errors we return
		if !git.IsErrNotExist(err) {
			return nil, err
		}
	} else {
		tagID = StringHash(tagIDStr)
	}

	// If type is "commit, the tag is a lightweight tag
	if service.ObjectType(tp) == service.ObjectCommit {
		commit, err := repo.GetCommit(idStr)
		if err != nil {
			return nil, err
		}
		tag := &Tag{
			Object: &Object{
				hash: tagID,
				repo: repo,
			},
			name:      name,
			tagType:   tp,
			tagObject: commitID,
			tagger:    commit.Committer(),
			message:   commit.Message(),
		}

		return tag, nil
	}

	// The tag is an annotated tag with a message.
	data, err := git.NewCommand("cat-file", "-p", idStr).RunInDirBytes(repo.Path())
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
	tag.tagType = tp

	return tag, nil
}
