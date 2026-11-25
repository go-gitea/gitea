// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"golang.org/x/net/html"
)

// emailAddressProcessor replaces raw email addresses with a mailto: link.
func emailAddressProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next {
		if !strings.Contains(node.Data[start:], "@") { // the regexp is slow, skip texts which can't match
			return
		}
		m := globalVars().emailRegex.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[2], m[3] = m[2]+start, m[3]+start

		var nextByte byte
		if len(node.Data) > m[3] {
			nextByte = node.Data[m[3]]
		}
		if strings.IndexByte(":/", nextByte) != -1 {
			// for cases: "git@gitea.com:owner/repo.git", "https://git@gitea.com/owner/repo.git"
			start = m[3]
			continue
		}
		mail := node.Data[m[2]:m[3]]
		replaceContent(node, m[2], m[3], createLink(ctx, "mailto:"+mail, mail, "" /*mailto*/))
		start = 0
		node = node.NextSibling.NextSibling
	}
}
