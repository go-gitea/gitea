// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"os"
	"path/filepath"

	"gitea.dev/modules/util"
)

type FileHelper struct {
	baseDir string
}

func NewFileHelper(baseDir string) *FileHelper {
	return &FileHelper{baseDir: baseDir}
}

func (h *FileHelper) fullPath(path string) string {
	return filepath.Join(h.baseDir, path)
}

func (h *FileHelper) AssertExists(t TestingT, path string, expected bool) {
	t.Helper()
	_, err := os.Stat(h.fullPath(path))
	errIsNotExist := os.IsNotExist(err)
	if expected != !errIsNotExist {
		t.Fatalf("file %s existence expected %v", path, expected)
	}
}

func (h *FileHelper) AssertFileExists(t TestingT, path string, expected *string) {
	t.Helper()
	if expected == nil {
		h.AssertExists(t, path, false)
		return
	}
	h.AssertFileContent(t, path, *expected)
}

func (h *FileHelper) AssertFileContent(t TestingT, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(h.fullPath(path))
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("file %s expected %q but got %q", path, expected, string(data))
	}
}

func (h *FileHelper) AssertSymLink(t TestingT, path, expected string) {
	t.Helper()
	link, err := os.Readlink(h.fullPath(path))
	if err != nil {
		t.Fatalf("failed to read symlink %s: %v", path, err)
	}
	if link != h.fullPath(expected) {
		t.Fatalf("symlink %s expected %q but got %q", path, expected, link)
	}
}

func (h *FileHelper) WriteFile(t TestingT, path, content string, optMode ...os.FileMode) {
	t.Helper()
	err := os.WriteFile(h.fullPath(path), []byte(content), util.OptionalArg(optMode, 0o644))
	if err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func (h *FileHelper) MkdirAll(t TestingT, path string, optMode ...os.FileMode) {
	t.Helper()
	err := os.MkdirAll(h.fullPath(path), util.OptionalArg(optMode, 0o755))
	if err != nil {
		t.Fatalf("failed to mkdir all %s: %v", path, err)
	}
}

func (h *FileHelper) Symlink(t TestingT, oldName, newName string) {
	t.Helper()
	err := os.Symlink(h.fullPath(oldName), h.fullPath(newName))
	if err != nil {
		t.Fatalf("failed to symlink from %s to %s: %v", oldName, newName, err)
	}
}
