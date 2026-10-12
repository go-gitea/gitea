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

const asciiSpaces = " \t\n\r\f"

func isAsciiSpace(r byte) bool {
	return strings.IndexByte(asciiSpaces, r) >= 0
}

func ParseSrcset(srcset string) (candidates []SrcSetCandidate) {
	// ref: https://html.spec.whatwg.org/multipage/images.html#parse-a-srcset-attribute
	extractLink := func() (string, bool) {
		srcset = strings.TrimLeft(srcset, asciiSpaces)
		if srcset == "" {
			return "", false
		}
		for i := range len(srcset) {
			c := srcset[i]
			curIsSpace := isAsciiSpace(c)
			if c == ',' || curIsSpace {
				nextIsSep := i == len(srcset)-1 || isAsciiSpace(srcset[i+1])
				if curIsSpace || nextIsSep {
					link := srcset[:i]
					srcset = srcset[i+1:]
					hasDesc := curIsSpace
					return link, hasDesc
				}
			}
		}
		link := srcset
		srcset = ""
		return link, false
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
			if c == ',' || isAsciiSpace(c) {
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
		sb.WriteString(c.SrcLink)
		for _, d := range c.SrcDesc {
			sb.WriteByte(' ')
			sb.WriteString(d)
		}
	}
	return sb.String()
}
