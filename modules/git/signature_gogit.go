// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// Signature represents the Author or Committer information.
type Signature = object.Signature

// Helper to get a signature from the commit line, which looks like these:
//
//	author Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
//	author Patrick Gundlach <gundlach@speedata.de> Thu Apr 07 22:13:13 2005 +0200
//
// but without the "author " at the beginning (this method should)
// be used for author and committer.
func newSignatureFromCommitline(line []byte) (_ *Signature, err error) {
	sig := new(Signature)
	emailStart := bytes.IndexByte(line, '<')
	emailEnd := bytes.IndexByte(line, '>')
	if emailStart == -1 || emailEnd == -1 || emailEnd < emailStart {
		return sig, err
	}

	if emailStart > 0 { // Empty name has already occurred, even if it shouldn't
		sig.Name = strings.TrimSpace(string(line[:emailStart-1]))
	}
	sig.Email = string(line[emailStart+1 : emailEnd])

	// Check date format.
	if len(line) > emailEnd+2 {
		firstChar := line[emailEnd+2]
		if firstChar >= 48 && firstChar <= 57 {
			timestop := bytes.IndexByte(line[emailEnd+2:], ' ')
			if timestop < 0 {
				return sig, nil
			}

			timestring := string(line[emailEnd+2 : emailEnd+2+timestop])
			seconds, _ := strconv.ParseInt(timestring, 10, 64)
			sig.When = time.Unix(seconds, 0)

			timestop += emailEnd + 3
			if timestop >= len(line) || timestop+5 > len(line) {
				return sig, nil
			}

			timezone := string(line[timestop : timestop+5])
			tzhours, err1 := strconv.ParseInt(timezone[0:3], 10, 64)
			tzmins, err2 := strconv.ParseInt(timezone[3:], 10, 64)
			if err1 != nil || err2 != nil {
				return sig, err
			}
			if tzhours < 0 {
				tzmins *= -1
			}
			tz := time.FixedZone("", int(tzhours*60*60+tzmins*60))
			sig.When = sig.When.In(tz)

		} else {
			sig.When, err = time.Parse(GitTimeLayout, string(line[emailEnd+2:]))
			if err != nil {
				return nil, err
			}
		}
	}
	return sig, nil
}
