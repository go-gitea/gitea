// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pipeline

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
)

// NameRevStdin runs name-rev --stdin
func NameRevStdin(shasToNameReader *io.PipeReader, nameRevStdinWriter *io.PipeWriter, wg *sync.WaitGroup, tmpBasePath string) {
	defer wg.Done()
	defer shasToNameReader.Close()
	defer nameRevStdinWriter.Close()

	stderr := new(bytes.Buffer)
	var errbuf strings.Builder
	if err := git.NewCommand("name-rev", "--stdin", "--name-only", "--always").RunInDirFullPipeline(tmpBasePath, nameRevStdinWriter, stderr, shasToNameReader); err != nil {
		_ = shasToNameReader.CloseWithError(fmt.Errorf("git name-rev [%s]: %v - %s", tmpBasePath, err, errbuf.String()))
	}
}
