// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfstransfer

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"gitea.dev/modules/lfstransfer/backend"

	"github.com/charmbracelet/git-lfs-transfer/transfer"
)

func Main(ctx context.Context, ownerName, repoName, verb, token string) error {
	logger := newLogger()
	backendReqPath := fmt.Sprintf("api/internal/repo/%s/%s.git/info/lfs", url.PathEscape(ownerName), url.PathEscape(repoName))
	giteaBackend, err := backend.New(ctx, backendReqPath, verb, token, logger)
	if err != nil {
		return err
	}

	pktLine := transfer.NewPktline(os.Stdin, os.Stdout, logger)
	for _, cap := range backend.Capabilities {
		if err := pktLine.WritePacketText(cap); err != nil {
			logger.Log("error sending capability due to error:", err)
		}
	}
	if err := pktLine.WriteFlush(); err != nil {
		logger.Log("error flushing capabilities:", err)
	}
	p := transfer.NewProcessor(pktLine, giteaBackend, logger)
	defer logger.Log("done processing commands")
	switch verb {
	case "upload":
		return p.ProcessCommands(transfer.UploadOperation)
	case "download":
		return p.ProcessCommands(transfer.DownloadOperation)
	default:
		return fmt.Errorf("unknown operation %q", verb)
	}
}
