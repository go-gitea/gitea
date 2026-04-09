// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import "html/template"

// HTMLDiff produces a rich inline diff between two rendered HTML documents.
// Inserted runs are wrapped in <span class="added-code">, deleted runs in
// <span class="removed-code">. It is intended for rendered content such as the
// Markdown diff view where both the old and new output should be visible
// simultaneously, and delegates to the shared placeholder-based diff logic in
// highlightCodeDiff.
func HTMLDiff(oldHTML, newHTML template.HTML) template.HTML {
	hcd := newHighlightCodeDiff()
	return hcd.diffHTMLWithHighlight(oldHTML, newHTML)
}
