// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"net/url"

	"gitea.dev/modules/markup"
)

// SetRichDiffLinks sets the rendered diff link for the files whose markup renders inline (markdown, orgmode, ...)
func (diff *Diff) SetRichDiffLinks(repoLink, beforeCommitID, afterCommitID string) {
	for _, file := range diff.Files {
		if file.IsBin || file.IsLFSFile || file.IsSubmodule || !CanRichDiff(file.Name) {
			continue
		}
		q := url.Values{}
		if file.Type != DiffFileAdd {
			q.Set("before", beforeCommitID)
			q.Set("old_path", file.OldName)
		}
		if file.Type != DiffFileDel {
			q.Set("after", afterCommitID)
			q.Set("path", file.Name)
		}
		file.RichDiffLink = repoLink + "/rich_diff?" + q.Encode()
	}
}

// CanRichDiff reports whether the file is rendered by a markup renderer whose output is shown inline
func CanRichDiff(filename string) bool {
	renderer := markup.DetectRendererTypeByFilename(filename)
	if renderer == nil || !markup.RendererNeedPostProcess(renderer) {
		return false
	}
	opts, isExternal := markup.GetExternalRendererOptions(renderer)
	return !isExternal || !opts.DisplayInIframe
}
