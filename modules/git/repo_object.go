// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"

	"gitea.dev/modules/git/gitcmd"
)

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

	str, err := repo.hashObjectBytes(ctx, nil, false)
	if err != nil {
		return nil, err
	}
	hash, err := NewIDFromString(str)
	if err != nil {
		return nil, err
	}

	repo.objectFormatCache = hash.Type()

	return repo.objectFormatCache, nil
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
		WithDir(repo.Path).
		WithStdinBytes(buf).
		RunStdString(ctx)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}
