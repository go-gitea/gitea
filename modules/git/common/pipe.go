// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git"
)

// PipeCommand runs a git command as a pipe
func PipeCommand(cmd *git.Command, path string, stdout *io.PipeWriter, stdin io.Reader) {
	stderr := strings.Builder{}
	err := cmd.
		RunInDirFullPipeline(path, stdout, &stderr, stdin)
	if err != nil {
		_ = stdout.CloseWithError(git.ConcatenateError(err, (&stderr).String()))
	} else {
		_ = stdout.Close()
	}
}
