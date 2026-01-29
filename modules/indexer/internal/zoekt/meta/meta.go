// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build unix

package meta

import (
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/json"
)

const metaFilename = "zoekt_meta.json"

func indexMetadataPath(dir string) string {
	return filepath.Join(dir, metaFilename)
}

type IndexMetadata struct {
	Version int `json:"version"`
}

func readJSON(path string, meta any) error {
	metaBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(metaBytes, meta)
}

func writeJSON(path string, meta any) error {
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(path, metaBytes, 0o666)
}

// ReadIndexMetadata returns the metadata for the index at the specified path.
// If no such index metadata exists, an empty metadata and a nil error are
// returned.
func ReadIndexMetadata(path string) (*IndexMetadata, error) {
	meta := &IndexMetadata{}
	metaPath := indexMetadataPath(path)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return meta, nil
	} else if err != nil {
		return nil, err
	}
	return meta, readJSON(metaPath, meta)
}

// WriteIndexMetadata writes metadata for the index at the specified path.
func WriteIndexMetadata(path string, meta *IndexMetadata) error {
	return writeJSON(indexMetadataPath(path), meta)
}
