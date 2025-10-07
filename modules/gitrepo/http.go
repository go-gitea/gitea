// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"io"
	"os"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

func serviceCmd(service string) *gitcmd.Command {
	switch service {
	case "receive-pack":
		return gitcmd.NewCommand("receive-pack")
	case "upload-pack":
		return gitcmd.NewCommand("upload-pack")
	default:
		// the service should be checked before invoking this function
		panic("unknown service: " + service)
	}
}

func StatelessRPC(ctx context.Context, storageRepo Repository, service string, extraEnvs []string, input io.Reader, output io.Writer) (string, error) {
	var stderr bytes.Buffer
	if err := serviceCmd(service).AddArguments("--stateless-rpc").
		AddDynamicArguments(repoPath(storageRepo)).
		WithDir(repoPath(storageRepo)).
		WithEnv(append(os.Environ(), extraEnvs...)).
		WithStderr(&stderr).
		WithStdin(input).
		WithStdout(output).
		WithUseContextTimeout(true).
		Run(ctx); err != nil {
		return stderr.String(), err
	}
	return "", nil
}

func StatelessRPCAdvertiseRefs(ctx context.Context, storageRepo Repository, service string, extraEnvs []string) ([]byte, error) {
	refs, _, err := serviceCmd(service).AddArguments("--stateless-rpc", "--advertise-refs", ".").
		WithEnv(append(os.Environ(), extraEnvs...)).
		WithDir(repoPath(storageRepo)).
		RunStdBytes(ctx)
	return refs, err
}
