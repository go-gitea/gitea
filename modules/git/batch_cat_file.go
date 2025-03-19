// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"
)

type BatchCatFile struct {
	cmd       *exec.Cmd
	startTime time.Time
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	cancel    context.CancelFunc
	finished  process.FinishedFunc
}

// NewBatchCatFile opens git cat-file --batch or --batch-check in the provided repo and returns a stdin pipe, a stdout reader and cancel function
// isCheck is true for --batch-check, false for --batch isCheck will only get metadata, --batch will get metadata and content
func NewBatchCatFile(ctx context.Context, repoPath string, isCheck bool) (*BatchCatFile, error) {
	// Now because of some insanity with git cat-file not immediately failing if not run in a valid git directory we need to run git rev-parse first!
	if err := ensureValidGitRepository(ctx, repoPath); err != nil {
		return nil, err
	}

	callerInfo := util.CallerFuncName(1 /* util */ + 1 /* this */ + 1 /* parent */)
	if pos := strings.LastIndex(callerInfo, "/"); pos >= 0 {
		callerInfo = callerInfo[pos+1:]
	}

	batchArg := util.Iif(isCheck, "--batch-check", "--batch")

	a := make([]string, 0, 4)
	a = append(a, debugQuote(GitExecutable))
	if len(globalCommandArgs) > 0 {
		a = append(a, "...global...")
	}
	a = append(a, "cat-file", batchArg)
	cmdLogString := strings.Join(a, " ")

	// these logs are for debugging purposes only, so no guarantee of correctness or stability
	desc := fmt.Sprintf("git.Run(by:%s, repo:%s): %s", callerInfo, logArgSanitize(repoPath), cmdLogString)
	log.Debug("git.BatchCatFile: %s", desc)

	ctx, cancel, finished := process.GetManager().AddContext(ctx, desc)

	args := make([]string, 0, len(globalCommandArgs)+2)
	for _, arg := range globalCommandArgs {
		args = append(args, string(arg))
	}
	args = append(args, "cat-file", batchArg)
	cmd := exec.CommandContext(ctx, GitExecutable, args...)
	cmd.Env = append(os.Environ(), CommonGitCmdEnvs()...)
	cmd.Dir = repoPath
	process.SetSysProcAttribute(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &BatchCatFile{
		cmd:       cmd,
		startTime: time.Now(),
		stdin:     stdin,
		stdout:    stdout,
		cancel:    cancel,
		finished:  finished,
	}, nil
}

func (b *BatchCatFile) Input(refs ...string) error {
	var buf bytes.Buffer
	for _, ref := range refs {
		if _, err := buf.WriteString(ref + "\n"); err != nil {
			return err
		}
	}

	_, err := b.stdin.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (b *BatchCatFile) Reader() *bufio.Reader {
	return bufio.NewReader(b.stdout)
}

func (b *BatchCatFile) Escaped() time.Duration {
	return time.Since(b.startTime)
}

func (b *BatchCatFile) Cancel() {
	b.cancel()
}

func (b *BatchCatFile) Close() error {
	b.finished()
	_ = b.stdin.Close()
	log.Debug("git.BatchCatFile: %v", b.Escaped())
	return b.cmd.Wait()
}
