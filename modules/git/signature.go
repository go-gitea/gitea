// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"strconv"
	"time"

	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Signature represents the Author or Committer information.
type Signature = object.Signature

const (
	// GitTimeLayout is the (default) time layout used by git.
	GitTimeLayout = "Mon Jan _2 15:04:05 2006 -0700"
)

// Helper to get a signature from the commit line, which looks like these:
//     author Patrick Gundlach <gundlach@speedata.de> 1378823654 +0200
//     author Patrick Gundlach <gundlach@speedata.de> Thu, 07 Apr 2005 22:13:13 +0200
// but without the "author " at the beginning (this method should)
// be used for author and committer.
//
// FIXME: include timezone for timestamp!
func newSignatureFromCommitline(line []byte) (_ *Signature, err error) {
	sig := new(Signature)
	emailStart := bytes.IndexByte(line, '<')
	sig.Name = string(line[:emailStart-1])
	emailEnd := bytes.IndexByte(line, '>')
	sig.Email = string(line[emailStart+1 : emailEnd])

	// Check date format.
	if len(line) > emailEnd+2 {
		firstChar := line[emailEnd+2]
		if firstChar >= 48 && firstChar <= 57 {
			timestop := bytes.IndexByte(line[emailEnd+2:], ' ')
			timestring := string(line[emailEnd+2 : emailEnd+2+timestop])
			seconds, _ := strconv.ParseInt(timestring, 10, 64)
			sig.When = time.Unix(seconds, 0)
		} else {
			sig.When, err = time.Parse(GitTimeLayout, string(line[emailEnd+2:]))
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Fall back to unix 0 time
		sig.When = time.Unix(0, 0)
	}
	return sig, nil
}
