// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// TagPrefix tags prefix path on the repository
const TagPrefix = "refs/tags/"

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(ctx context.Context, repoPath, name string) bool {
	return IsReferenceExist(ctx, repoPath, TagPrefix+name)
}

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, err := NewCommand(repo.Ctx, "tag", "--", name, revision).RunInDir(repo.Path)
	return err
}

// CreateAnnotatedTag create one annotated tag in the repository
func (repo *Repository) CreateAnnotatedTag(name, message, revision string) error {
	_, err := NewCommand(repo.Ctx, "tag", "-a", "-m", message, "--", name, revision).RunInDir(repo.Path)
	return err
}

// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
func (repo *Repository) GetTagNameBySHA(sha string) (string, error) {
	if len(sha) < 5 {
		return "", fmt.Errorf("SHA is too short: %s", sha)
	}

	stdout, err := NewCommand(repo.Ctx, "show-ref", "--tags", "-d").RunInDir(repo.Path)
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
	stdout, err := NewCommand(repo.Ctx, "show-ref", "--tags", "--", name).RunInDir(repo.Path)
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

const (
	dualNullChar         = "\x00\x00"
	forEachRefTagsFormat = `type %(objecttype)%00tag %(refname:short)%00object %(object)%00objectname %(objectname)%00tagger %(creator)%00message %(contents)%00signature %(contents:signature)%00%00`
)

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos(page, pageSize int) ([]*Tag, int, error) {
	stdout, err := NewCommand(repo.Ctx, "for-each-ref", "--format", forEachRefTagsFormat, "--sort", "-*creatordate", "refs/tags").RunInDir(repo.Path)
	if err != nil {
		return nil, 0, err
	}

	refBlocks := strings.Split(stdout, dualNullChar)
	var tags []*Tag
	for _, refBlock := range refBlocks {
		refBlock = strings.TrimSpace(refBlock)
		if refBlock == "" {
			break
		}

		tag, err := parseTagRef(refBlock)
		if err != nil {
			return nil, 0, err
		}

		tags = append(tags, tag)
	}

	tagsTotal := len(tags)
	if page != 0 {
		tags = util.PaginateSlice(tags, page, pageSize).([]*Tag)
	}
	// TODO shouldn't be necessary
	sortTagsByTime(tags)
	return tags, tagsTotal, nil
}

// note: relies on output being formatted using forEachRefFormat
func parseTagRef(ref string) (*Tag, error) {
	var tag Tag
	items := strings.Split(ref, "\x00")
	for _, item := range items {
		// item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		var field string
		var value string
		firstSpace := strings.Index(item, " ")
		if firstSpace > 0 {
			field = item[:firstSpace]
			value = item[firstSpace+1:]
		} else {
			field = item
		}

		if value == "" {
			continue
		}

		switch field {
		case "type":
			tag.Type = value
		case "tag":
			tag.Name = value
		case "objectname":
			var err error
			tag.ID, err = NewIDFromString(value)
			if err != nil {
				return nil, fmt.Errorf("parse objectname '%s': %v", value, err)
			}
			if tag.Type == "commit" {
				tag.Object = tag.ID
			}
		case "object":
			var err error
			tag.Object, err = NewIDFromString(value)
			if err != nil {
				return nil, fmt.Errorf("parse object '%s': %v", value, err)
			}
		case "tagger":
			var err error
			tag.Tagger, err = newSignatureFromCommitline([]byte(value))
			if err != nil {
				return nil, fmt.Errorf("parse tagger: %w", err)
			}
		case "message":
			tag.Message = value
			// srtip PGP signature if present in contents field
			pgpStart := strings.Index(value, beginpgp)
			if pgpStart >= 0 {
				tag.Message = tag.Message[0:pgpStart]
			}
			// tag.Message += "\n"
		case "signature":
			tag.Signature = &CommitGPGSignature{
				Signature: value,
				// TODO: don't know what to do about Payload. Is
				// it even relevant in this context?
			}
		}
	}

	return &tag, nil
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
