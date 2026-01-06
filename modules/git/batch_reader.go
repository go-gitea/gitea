// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"errors"

	"code.gitea.io/gitea/modules/git/catfile"
)

// ParseCatFileTreeLine reads an entry from a tree in a cat-file --batch stream.
func ParseCatFileTreeLine(objectFormat ObjectFormat, rd catfile.ReadCloseDiscarder, modeBuf, fnameBuf, shaBuf []byte) (mode, fname, sha []byte, n int, err error) {
	mode, fname, sha, n, err = catfile.ParseCatFileTreeLine(objectFormat, rd, modeBuf, fnameBuf, shaBuf)
	return mode, fname, sha, n, convertCatfileError(err, nil)
}

func convertCatfileError(err error, defaultID []byte) error {
	if err == nil {
		return nil
	}
	var notFound catfile.ErrObjectNotFound
	if errors.As(err, &notFound) {
		if notFound.ID == "" && len(defaultID) > 0 {
			notFound.ID = string(defaultID)
		}
		return ErrNotExist{ID: notFound.ID}
	}
	return err
}
