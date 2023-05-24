// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/log"
	"github.com/google/licensecheck"
)

// GetLicenseStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLicenseStats(commitID string) ([]string, error) {
	// We will feed the commit IDs in order into cat-file --batch, followed by blobs as necessary.
	// so let's create a batch stdin and stdout
	batchStdinWriter, batchReader, cancel := repo.CatFileBatch(repo.Ctx)
	defer cancel()

	writeID := func(id string) error {
		_, err := batchStdinWriter.Write([]byte(id + "\n"))
		return err
	}

	if err := writeID(commitID); err != nil {
		return nil, err
	}
	shaBytes, typ, size, err := ReadBatchLine(batchReader)
	if typ != "commit" {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, ErrNotExist{commitID, ""}
	}

	sha, err := NewIDFromString(string(shaBytes))
	if err != nil {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, ErrNotExist{commitID, ""}
	}

	commit, err := CommitFromReader(repo, sha, io.LimitReader(batchReader, size))
	if err != nil {
		log.Debug("Unable to get commit for: %s. Err: %v", commitID, err)
		return nil, err
	}
	if _, err = batchReader.Discard(1); err != nil {
		return nil, err
	}

	tree := commit.Tree

	entries, err := tree.ListEntriesRecursiveWithSize()
	if err != nil {
		return nil, err
	}

	var licenses []string
	for _, f := range entries {
		if f.Size() == 0 {
			continue
		}

		// TODO: support ext
		if f.Name() == "LICENSE" {
			// Need to read all of them or can not get the right result from licensecheck.Scan
			if err := writeID(f.ID.String()); err != nil {
				return nil, err
			}
			_, _, size, err := ReadBatchLine(batchReader)
			if err != nil {
				log.Debug("Error reading blob: %s Err: %v", f.ID.String(), err)
				return nil, err
			}
			contentBuf := bytes.Buffer{}
			_, err = contentBuf.ReadFrom(io.LimitReader(batchReader, size))
			if err != nil {
				return nil, err
			}
			cov := licensecheck.Scan(contentBuf.Bytes())
			for _, m := range cov.Match {
				licenses = append(licenses, m.ID)
			}
			break
		}
	}

	return licenses, nil
}
