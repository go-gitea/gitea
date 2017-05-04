// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"
	"time"

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

// TagOption describes tag options
type TagOption struct {
}

// parseTag parse the line
// 2016-10-14 20:54:25 +0200  (tag: translation/20161014.01) d3b76dcf2 Dirk Baeumer dirkb@microsoft.com Merge in translations
func parseTag(line string, opt TagOption) (*Tag, error) {
	line = strings.TrimSpace(line)
	if len(line) < 40 {
		return nil, nil
	}

	var (
		err error
		tag Tag
		sig Signature
	)
	sig.When, err = time.Parse("2006-01-02 15:04:05 -0700", line[0:25])
	if err != nil {
		return nil, err
	}

	left := strings.TrimSpace(line[25:])
	start := strings.Index(left, "tag: ")
	if start < 0 {
		return nil, nil
	}
	end := strings.LastIndexByte(left[start+1:], ')')
	if end < 0 {
		return nil, nil
	}
	end = end + start + 1
	part := strings.IndexByte(left[start+5:end], ',')
	if part > 0 {
		tag.Name = strings.TrimSpace(left[start+5 : start+5+part])
	} else {
		tag.Name = strings.TrimSpace(left[start+5 : end])
	}
	next := strings.IndexByte(left[end+2:], ' ')
	if next < 0 {
		return nil, nil
	}
	tag.Object = MustIDFromString(strings.TrimSpace(left[end+2 : end+2+next]))
	next = end + 2 + next

	emailStart := strings.IndexByte(left[next:], '<')
	sig.Name = strings.TrimSpace(left[next:][:emailStart-1])
	emailEnd := strings.IndexByte(left[next:], '>')
	sig.Email = strings.TrimSpace(left[next:][emailStart+1 : emailEnd])
	tag.Tagger = &sig
	tag.Message = strings.TrimSpace(left[next+emailEnd+1:])
	return &tag, nil
}

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos(opt TagOption) ([]*Tag, error) {
	cmd := NewCommand("log", "--tags", "--simplify-by-decoration", `--pretty=format:"%ci %d %H %cn<%ce> %s"`)
	stdout, err := cmd.RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	tagSlices := strings.Split(stdout, "\n")
	var tags []*Tag
	for _, line := range tagSlices {
		line := strings.Trim(line, `"`)
		tag, err := parseTag(line, opt)
		if err != nil {
			return nil, err
		}
		if tag != nil {
			tag.repo = repo
			tags = append(tags, tag)
		}
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
