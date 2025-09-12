// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"context"
	"io"
	"os"

	"code.gitea.io/gitea/modules/git"
)

func serviceCmd(service string) *git.Command {
	switch service {
	case "receive-pack":
		return git.NewCommand("receive-pack")
	case "upload-pack":
		return git.NewCommand("upload-pack")
	default:
		// the service should be checked before invoking this function
		panic("unknown service: " + service)
	}
}

func StatelessRPC(ctx context.Context, storageRepo Repository, service string, extraEnvs []string, input io.Reader, output io.Writer) (string, error) {
	var stderr bytes.Buffer
	if err := serviceCmd(service).
		AddArguments("--stateless-rpc").
		AddDynamicArguments(repoPath(storageRepo)).
		Run(ctx, &git.RunOpts{
			Dir:               repoPath(storageRepo),
			Env:               append(os.Environ(), extraEnvs...),
			Stdout:            output,
			Stdin:             input,
			Stderr:            &stderr,
			UseContextTimeout: true,
		}); err != nil {
		return stderr.String(), err
	}
	return "", nil
}

func StatelessRPCAdvertiseRefs(ctx context.Context, storageRepo Repository, service string, extraEnvs []string) ([]byte, error) {
	refs, _, err := serviceCmd(service).AddArguments("--stateless-rpc", "--advertise-refs", ".").
		RunStdBytes(ctx, &git.RunOpts{
			Dir: repoPath(storageRepo),
			Env: append(os.Environ(), extraEnvs...),
		})
	return refs, err
}
