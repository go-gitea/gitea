// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// TagPrefix tags prefix path on the repository
const TagPrefix = "refs/tags/"

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, _, err := gitcmd.NewCommand("tag").AddDashesAndList(name, revision).WithDir(repo.Path).RunStdString(repo.Ctx)
	return err
}

// CreateAnnotatedTag create one annotated tag in the repository
func (repo *Repository) CreateAnnotatedTag(name, message, revision string) error {
	_, _, err := gitcmd.NewCommand("tag", "-a", "-m").
		AddDynamicArguments(message).
		AddDashesAndList(name, revision).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	return err
}

// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
func (repo *Repository) GetTagNameBySHA(sha string) (string, error) {
	if len(sha) < 5 {
		return "", fmt.Errorf("SHA is too short: %s", sha)
	}

	stdout, _, err := gitcmd.NewCommand("show-ref", "--tags", "-d").WithDir(repo.Path).RunStdString(repo.Ctx)
	if err != nil {
		return "", err
	}

	tagRefs := strings.SplitSeq(stdout, "\n")
	for tagRef := range tagRefs {
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
	stdout, _, err := gitcmd.NewCommand("show-ref", "--tags").AddDashesAndList(name).WithDir(repo.Path).RunStdString(repo.Ctx)
	if err != nil {
		return "", err
	}
	// Make sure exact match is used: "v1" != "release/v1"
	for line := range strings.SplitSeq(stdout, "\n") {
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

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// GetTagWithID returns a Git tag by given name and ID
func (repo *Repository) GetTagWithID(idStr, name string) (*Tag, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
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

	// Get tag name
	name, err := repo.GetTagNameBySHA(id.String())
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
}
