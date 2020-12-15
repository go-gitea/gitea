// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
)

// LogHashFormat represents the pretty format that just reports the hash
const LogHashFormat = "--pretty=format:%H"

func callShowRef(repoPath, prefix, arg string) ([]string, error) {
	var branchNames []string

	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(
		git.NewCommand("show-ref", arg),
		repoPath,
		stdoutWriter,
		nil)

	bufReader := bufio.NewReader(stdoutReader)
	for {
		// The output of show-ref is simply a list:
		// <sha> SP <ref> LF
		_, err := bufReader.ReadSlice(' ')
		for err == bufio.ErrBufferFull {
			// This shouldn't happen but we'll tolerate it for the sake of peace
			_, err = bufReader.ReadSlice(' ')
		}
		if err == io.EOF {
			return branchNames, nil
		}
		if err != nil {
			return nil, err
		}

		branchName, err := bufReader.ReadString('\n')
		if err == io.EOF {
			// This shouldn't happen... but we'll tolerate it for the sake of peace
			return branchNames, nil
		}
		if err != nil {
			return nil, err
		}
		branchName = strings.TrimPrefix(branchName, prefix)
		if len(branchName) > 0 {
			branchName = branchName[:len(branchName)-1]
		}
		branchNames = append(branchNames, branchName)
	}
}
