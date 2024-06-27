package lfstransfer

import (
	"context"
	"fmt"
	"os"
	"strings"

	db_model "code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/lfstransfer/backend"
	"code.gitea.io/gitea/modules/lfstransfer/transfer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

func initServices(ctx context.Context) error {
	setting.MustInstalled()
	setting.LoadDBSetting()
	setting.InitSQLLoggersForCli(log.INFO)
	if err := db_model.InitEngine(ctx); err != nil {
		return fmt.Errorf("unable to initialize the database using configuration [%q]: %w", setting.CustomConf, err)
	}
	if err := storage.Init(); err != nil {
		return fmt.Errorf("unable to initialise storage: %v", err)
	}
	return nil
}

func getRepo(ctx context.Context, path string) (*repo_model.Repository, error) {
	// runServ ensures repoPath is [owner]/[name].git
	pathSeg := strings.Split(path, "/")
	pathSeg[1] = strings.TrimSuffix(pathSeg[1], ".git")
	return repo_model.GetRepositoryByOwnerAndName(ctx, pathSeg[0], pathSeg[1])
}

func Main(ctx context.Context, repoPath string, verb string) error {
	if err := initServices(ctx); err != nil {
		return err
	}

	logger := newLogger()
	pktline := transfer.NewPktline(os.Stdin, os.Stdout, logger)
	repo, err := getRepo(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("unable to get repository: %s Error: %v", repoPath, err)
	}
	giteaBackend := backend.New(ctx, repo, lfs.NewContentStore())

	for _, cap := range backend.Capabilities {
		if err := pktline.WritePacketText(cap); err != nil {
			log.Error("error sending capability [%v] due to error: %v", cap, err)
		}
	}
	if err := pktline.WriteFlush(); err != nil {
		log.Error("error flushing capabilities: %v", err)
	}
	p := transfer.NewProcessor(pktline, giteaBackend, logger)
	defer log.Info("done processing commands")
	switch verb {
	case "upload":
		return p.ProcessCommands(transfer.UploadOperation)
	case "download":
		return p.ProcessCommands(transfer.DownloadOperation)
	default:
		return fmt.Errorf("unknown operation %q", verb)
	}
}
