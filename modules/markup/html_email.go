// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"

	"golang.org/x/net/html"
)

// emailAddressProcessor replaces raw email addresses with a mailto: link.
func emailAddressProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := globalVars().emailRegex.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		var nextByte byte
		if len(node.Data) > m[3] {
			nextByte = node.Data[m[3]]
		}
		if strings.IndexByte(":/", nextByte) != -1 {
			// for cases: "git@gitea.com:owner/repo.git", "https://git@gitea.com/owner/repo.git"
			return
		}
		mail := node.Data[m[2]:m[3]]
		replaceContent(node, m[2], m[3], createLink(ctx, "mailto:"+mail, mail, "" /*mailto*/))
		node = node.NextSibling.NextSibling
	}
}
