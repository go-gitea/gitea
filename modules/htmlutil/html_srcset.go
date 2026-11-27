// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmlutil

import (
	"strings"
)

type SrcSetCandidate struct {
	SrcLink string
	SrcDesc []string
}

const (
	asciiSpaces         = " \t\n\r\f"
	asciiSpacesAndComma = " \t\n\r\f,"
)

func ParseSrcset(srcset string) (candidates []SrcSetCandidate) {
	// ref: https://html.spec.whatwg.org/multipage/images.html#parse-a-srcset-attribute
	// It can't simply use "split" because comma can appear in URLs and descriptors can be parenthesized and contain commas.
	extractLink := func() (string, bool) {
		srcset = strings.TrimLeft(srcset, asciiSpacesAndComma)
		if srcset == "" {
			return "", false
		}
		end := strings.IndexAny(srcset, asciiSpaces)
		if end == -1 {
			end = len(srcset)
		}
		link := srcset[:end]
		srcset = srcset[end:]
		if strings.HasSuffix(link, ",") {
			return strings.TrimRight(link, ","), false
		}
		return link, true
	}
	extractDesc := func() (string, bool) {
		srcset = strings.TrimLeft(srcset, asciiSpaces)
		if srcset == "" {
			return "", false
		}
		inParentheses := false
		for i := range len(srcset) {
			c := srcset[i]
			if inParentheses {
				inParentheses = c != ')'
				continue
			}
			if c == '(' {
				inParentheses = true
				continue
			}
			if strings.IndexByte(asciiSpacesAndComma, c) != -1 {
				desc := srcset[:i]
				srcset = srcset[i+1:]
				return desc, c != ','
			}
		}
		desc := srcset
		srcset = ""
		return desc, false
	}

	for {
		link, hasDesc := extractLink()
		if link == "" {
			break
		}
		var descriptors []string
		if hasDesc {
			for {
				desc, hasMoreDesc := extractDesc()
				if desc != "" {
					descriptors = append(descriptors, desc)
				}
				if !hasMoreDesc {
					break
				}
			}
		}
		candidates = append(candidates, SrcSetCandidate{SrcLink: link, SrcDesc: descriptors})
	}
	return candidates
}

func EncodeSrcset(candidates []SrcSetCandidate) string {
	var sb strings.Builder
	for i, c := range candidates {
		if i != 0 {
			sb.WriteString(", ")
		}
		link := c.SrcLink
		if prefix, ok := strings.CutSuffix(link, ","); ok {
			link = prefix + "%2C"
		}
		sb.WriteString(link)
		for _, d := range c.SrcDesc {
			sb.WriteByte(' ')
			sb.WriteString(d)
		}
	}
	return sb.String()
}
