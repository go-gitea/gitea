// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wordsfilter

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/charset"

	stdcharset "golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

const cmdDiffHead = "diff --git "

// CheckPatchWords check git patch
func CheckPatchWords(reader io.Reader) ([]string, error) {
	input := bufio.NewReader(reader)
	line, err := input.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	// 1. A patch file always begins with `diff --git ` + `a/path b/path` (possibly quoted)
	// if it does not we have bad input!
	if !strings.HasPrefix(line, cmdDiffHead) {
		return nil, nil
	}

	var inSection bool
	var sb bytes.Buffer
	for {
		line, err = input.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
		if strings.HasPrefix(line, "+ ") {
			inSection = true
			sb.WriteString(line[1:])
			continue
		}

		if inSection {
			charsetLabel, err := charset.DetectEncoding(sb.Bytes())
			if charsetLabel != "UTF-8" && err == nil {
				encoding, _ := stdcharset.Lookup(charsetLabel)
				if encoding != nil {
					d := encoding.NewDecoder()
					c, _, err := transform.String(d, sb.String())
					if err != nil {
						return nil, err
					}
					matches := Search(c)
					if len(matches) > 0 {
						return matches, nil
					}
				}
			}
			inSection = false
		}
	}

	return nil, nil
}
