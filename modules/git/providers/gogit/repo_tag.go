// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/providers/native"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// ___
//  |   _.  _
//  |  (_| (_|
//          _|

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	gogitrepo, err := GetGoGitRepo(repo)
	if err != nil {
		return false
	}
	_, err = gogitrepo.Reference(plumbing.ReferenceName(git.TagPrefix+name), true)
	return err == nil
}

// GetTags returns all tags of the repository.
func (repo *Repository) GetTags() ([]string, error) {
	var tagNames []string
	gogitRepo, err := GetGoGitRepo(repo)
	if err != nil {
		return nil, err
	}

	tags, err := gogitRepo.Tags()
	if err != nil {
		return nil, err
	}

	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name().String(), git.TagPrefix))
		return nil
	})

	// Reverse order
	for i := 0; i < len(tagNames)/2; i++ {
		j := len(tagNames) - i - 1
		tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
	}

	return tagNames, nil
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
	tagsIter, err := repo.gogitRepo.TagObjects()
	if err != nil {
		return nil, err
	}
	defer tagsIter.Close()

	var returnable []service.Tag
	limit := -1

	if page > 0 {
		for i := 0; i < (page-1)*pageSize; i++ {
			_, err := tagsIter.EncodedObjectIter.Next() // Avoid loading the tag as there is no point
			if err != nil {
				if err == storer.ErrStop {
					return nil, nil
				}
				return nil, err
			}
		}
		limit = pageSize
		returnable = make([]service.Tag, 0, pageSize)
	} else {
		returnable = make([]service.Tag, 0, 20)
	}

loop:
	for {
		tag, err := tagsIter.Next()
		if err != nil {
			if err == storer.ErrStop {
				break loop
			}
			return nil, err
		}
		returnable = append(returnable, convertTag(repo, tag))
		if limit > 0 && len(returnable) >= limit {
			break loop
		}
	}
	return returnable, nil
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id service.Hash) (string, error) {
	obj, err := repo.gogitRepo.Object(plumbing.AnyObject, ToPlumbingHash(id))
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			return "", git.ErrNotExist{ID: id.String()}
		}
		return "", err
	}
	return obj.Type().String(), nil
}

// GetAnnotatedTag returns a Git tag by its SHA, must be an annotated tag
func (repo *Repository) GetAnnotatedTag(sha string) (service.Tag, error) {
	hash, err := SHA1{}.FromString(sha)
	if err != nil {
		return nil, err
	}

	tag, err := repo.gogitRepo.TagObject(ToPlumbingHash(hash))
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			return nil, git.ErrNotExist{
				ID: sha,
			}
		}
		return nil, err
	}
	return convertTag(repo, tag), nil
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

		return native.NewTag(
			&Object{
				hash: tagID,
				repo: repo,
			},
			name,
			commitID,
			tp,
			commit.Committer(),
			commit.Message(),
			nil,
		), nil
	}

	// The tag is an annotated tag with a message.
	tag, err := repo.gogitRepo.TagObject(ToPlumbingHash(id))
	if err != nil {
		return nil, err
	}

	return convertTag(repo, tag), nil
}

func convertTag(repo *Repository, tag *object.Tag) service.Tag {
	return native.NewTag(
		&Object{
			hash: fromPlumbingHash(tag.Hash),
			repo: repo,
		},
		tag.Name,
		fromPlumbingHash(tag.Target),
		tag.TargetType.String(),
		convertSignature(&tag.Tagger),
		tag.Message,
		convertTagPGPSignature(tag),
	)
}

func convertTagPGPSignature(tag *object.Tag) *service.GPGSignature {
	if tag.PGPSignature == "" {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w, "object %s\n", tag.Target.String()); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "type %s\n", tag.TargetType.String()); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "tag %s\n", tag.Name); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "tagger "); err != nil {
		return nil
	}

	if err = tag.Tagger.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n%s", tag.Message); err != nil {
		return nil
	}

	return &service.GPGSignature{
		Signature: tag.PGPSignature,
		Payload:   w.String(),
	}
}
