// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// DiffLineSectionInfo represents diff line section meta data
type DiffLineSectionInfo struct {
	Path          string `json:"path"`
	LastLeftIdx   int    `json:"last_left_idx"`
	LastRightIdx  int    `json:"last_right_idx"`
	LeftIdx       int    `json:"left_idx"`
	RightIdx      int    `json:"right_idx"`
	LeftHunkSize  int    `json:"left_hunk_size"`
	RightHunkSize int    `json:"right_hunk_size"`
}

// DiffLine represents a line difference in a DiffSection.
type DiffLine struct {
	LeftIdx     int                  `json:"left_idx"`
	RightIdx    int                  `json:"right_idx"`
	Type        int                  `json:"type"`
	Content     string               `json:"content"`
	Comments    []*Comment           `json:"comments"`
	SectionInfo *DiffLineSectionInfo `json:"section_info"`
}

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	Name  string      `json:"name"`
	Lines []*DiffLine `json:"lines"`
}

// DiffFile represents a file diff.
type DiffFile struct {
	Name         string         `json:"name"`
	OldName      string         `json:"old_name"`
	Index        int            `json:"index"`
	Addition     int            `json:"addition"`
	Deletion     int            `json:"deletion"`
	Type         int            `json:"type"`
	IsCreated    bool           `json:"is_created"`
	IsDeleted    bool           `json:"is_deleted"`
	IsBin        bool           `json:"is_bin"`
	IsLFSFile    bool           `json:"is_lfs_file"`
	IsRenamed    bool           `json:"is_renamed"`
	IsSubmodule  bool           `json:"is_submoudule"`
	Sections     []*DiffSection `json:"sections"`
	IsIncomplete bool           `json:"is_incomplete"`
}

// Diff represents a difference between two git trees.
type Diff struct {
	TotalAddition int         `json:"total_addition"`
	TotalDeletion int         `json:"total_deletion"`
	Files         []*DiffFile `json:"files"`
	IsIncomplete  bool        `json:"is_incomplete"`
}

// Compare information.
type Compare struct {
	Title        string    `json:"title"`
	Commits      []*Commit `json:"commits"`
	CommitCount  int       `json:"commit_count"`
	Diff         *Diff     `json:"diff"`
	BaseCommitID string    `json:"base_commit_id"`
	HeadCommitID string    `json:"head_commit_id"`
}
