// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

// MaxGitObjectSize is used to avoid OOM when reading a large git object.
// GitHub has a limit (100M) for pushing, but Gitea doesn't have such a limit yet.
var MaxGitObjectSize int64 = 100 * 1024 * 1024

// ObjectType git object type
type ObjectType string

const (
	// ObjectCommit commit object type
	ObjectCommit ObjectType = "commit"
	// ObjectTree tree object type
	ObjectTree ObjectType = "tree"
	// ObjectBlob blob object type
	ObjectBlob ObjectType = "blob"
	// ObjectTag tag object type
	ObjectTag ObjectType = "tag"
	// ObjectBranch branch object type
	ObjectBranch ObjectType = "branch"
)

// Bytes returns the byte array for the Object Type
func (o ObjectType) Bytes() []byte {
	return []byte(o)
}

func (repo *Repository) GetObjectFormat(ctx context.Context) (ObjectFormat, error) {
	if repo.objectFormatCache != nil {
		return repo.objectFormatCache, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	batch, cancel, err := repo.CatFileBatch()
	if err != nil {
		return nil, err
	}
	defer cancel()
	for _, objectFormat := range []ObjectFormat{Sha1ObjectFormat, Sha256ObjectFormat} {
		// git always knows the empty tree of the repository's own object format, a ref named like it resolves to another ID
		emptyTree := objectFormat.EmptyTree().String()
		info, err := batch.QueryInfo(emptyTree)
		if err == nil && info.ID == emptyTree {
			repo.objectFormatCache = objectFormat
			return objectFormat, nil
		} else if err != nil && !IsErrNotExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("unknown object format for repository %s", repo.LogString())
}

// HashObjectBytes returns hash for the content
func (repo *Repository) HashObjectBytes(ctx context.Context, buf []byte) (ObjectID, error) {
	idStr, err := repo.hashObjectBytes(ctx, buf, true)
	if err != nil {
		return nil, err
	}
	return NewIDFromString(idStr)
}

func (repo *Repository) hashObjectBytes(ctx context.Context, buf []byte, save bool) (string, error) {
	var cmd *gitcmd.Command
	if save {
		cmd = gitcmd.NewCommand("hash-object", "-w", "--stdin")
	} else {
		cmd = gitcmd.NewCommand("hash-object", "--stdin")
	}
	stdout, _, err := cmd.
		WithRepo(repo).
		WithStdinBytes(buf).
		RunStdString(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
