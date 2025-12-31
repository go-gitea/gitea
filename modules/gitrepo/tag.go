// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/foreachref"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/util"
)

// IsTagExist returns true if given tag exists in the repository.
func IsTagExist(ctx context.Context, repo Repository, name string) bool {
	return IsReferenceExist(ctx, repo, git.TagPrefix+name)
}

// GetTagInfos returns all tag infos of the repository.
func GetTagInfos(ctx context.Context, repo Repository, page, pageSize int) ([]*git.Tag, int, error) {
	// Generally, refname:short should be equal to refname:lstrip=2 except core.warnAmbiguousRefs is used to select the strict abbreviation mode.
	// https://git-scm.com/docs/git-for-each-ref#Documentation/git-for-each-ref.txt-refname
	forEachRefFmt := foreachref.NewFormat("objecttype", "refname:lstrip=2", "object", "objectname", "creator", "contents", "contents:signature")

	stdoutReader, stdoutWriter := io.Pipe()
	defer stdoutReader.Close()
	defer stdoutWriter.Close()
	stderr := strings.Builder{}

	go func() {
		err := RunCmd(ctx, repo, gitcmd.NewCommand("for-each-ref").
			AddOptionFormat("--format=%s", forEachRefFmt.Flag()).
			AddArguments("--sort", "-*creatordate", "refs/tags").
			WithStdout(stdoutWriter).
			WithStderr(&stderr))
		if err != nil {
			_ = stdoutWriter.CloseWithError(gitcmd.ConcatenateError(err, stderr.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	var tags []*git.Tag
	parser := forEachRefFmt.Parser(stdoutReader)
	for {
		ref := parser.Next()
		if ref == nil {
			break
		}

		tag, err := parseTagRef(ref)
		if err != nil {
			return nil, 0, fmt.Errorf("GetTagInfos: parse tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := parser.Err(); err != nil {
		return nil, 0, fmt.Errorf("GetTagInfos: parse output: %w", err)
	}

	sortTagsByTime(tags)
	tagsTotal := len(tags)
	if page != 0 {
		tags = util.PaginateSlice(tags, page, pageSize).([]*git.Tag)
	}

	return tags, tagsTotal, nil
}

// parseTagRef parses a tag from a 'git for-each-ref'-produced reference.
func parseTagRef(ref map[string]string) (tag *git.Tag, err error) {
	tag = &git.Tag{
		Type: ref["objecttype"],
		Name: ref["refname:lstrip=2"],
	}

	tag.ID, err = git.NewIDFromString(ref["objectname"])
	if err != nil {
		return nil, fmt.Errorf("parse objectname '%s': %w", ref["objectname"], err)
	}

	if tag.Type == "commit" {
		// lightweight tag
		tag.Object = tag.ID
	} else {
		// annotated tag
		tag.Object, err = git.NewIDFromString(ref["object"])
		if err != nil {
			return nil, fmt.Errorf("parse object '%s': %w", ref["object"], err)
		}
	}

	tag.Tagger = git.ParseSignatureFromCommitLine(ref["creator"])
	tag.Message = ref["contents"]

	// strip any signature if present in contents field
	_, tag.Message, _ = git.ParsePayloadSignature(util.UnsafeStringToBytes(tag.Message), 0)

	// annotated tag with GPG signature
	if tag.Type == "tag" && ref["contents:signature"] != "" {
		payload := fmt.Sprintf("object %s\ntype commit\ntag %s\ntagger %s\n\n%s\n",
			tag.Object, tag.Name, ref["creator"], strings.TrimSpace(tag.Message))
		tag.Signature = &git.CommitSignature{
			Signature: ref["contents:signature"],
			Payload:   payload,
		}
	}

	return tag, nil
}

type tagSorter []*git.Tag

func (ts tagSorter) Len() int {
	return len([]*git.Tag(ts))
}

func (ts tagSorter) Less(i, j int) bool {
	return []*git.Tag(ts)[i].Tagger.When.After([]*git.Tag(ts)[j].Tagger.When)
}

func (ts tagSorter) Swap(i, j int) {
	[]*git.Tag(ts)[i], []*git.Tag(ts)[j] = []*git.Tag(ts)[j], []*git.Tag(ts)[i]
}

// sortTagsByTime
func sortTagsByTime(tags []*git.Tag) {
	sorter := tagSorter(tags)
	sort.Sort(sorter)
}
