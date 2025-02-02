// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	notify_service "code.gitea.io/gitea/services/notify"
)

func initGlobalRunnerToken(ctx context.Context) error {
	// use the same env name as the runner, for consistency
	token := os.Getenv("GITEA_RUNNER_REGISTRATION_TOKEN")
	tokenFile := os.Getenv("GITEA_RUNNER_REGISTRATION_TOKEN_FILE")
	if token != "" && tokenFile != "" {
		return errors.New("both GITEA_RUNNER_REGISTRATION_TOKEN and GITEA_RUNNER_REGISTRATION_TOKEN_FILE are set, only one can be used")
	}
	if tokenFile != "" {
		file, err := os.ReadFile(tokenFile)
		if err != nil {
			return fmt.Errorf("unable to read GITEA_RUNNER_REGISTRATION_TOKEN_FILE: %w", err)
		}
		token = strings.TrimSpace(string(file))
	}
	if token == "" {
		return nil
	}

	if len(token) < 32 {
		return errors.New("GITEA_RUNNER_REGISTRATION_TOKEN must be at least 32 random characters")
	}

	existing, err := actions_model.GetRunnerToken(ctx, token)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return fmt.Errorf("unable to check existing token: %w", err)
	}
	if existing != nil {
		if !existing.IsActive {
			log.Warn("The token defined by GITEA_RUNNER_REGISTRATION_TOKEN is already invalidated, please use the latest one from web UI")
		}
		return nil
	}
	_, err = actions_model.NewRunnerTokenWithValue(ctx, 0, 0, token)
	return err
}

func Init(ctx context.Context) error {
	if !setting.Actions.Enabled {
		return nil
	}

	jobEmitterQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "actions_ready_job", jobEmitterQueueHandler)
	if jobEmitterQueue == nil {
		return errors.New("unable to create actions_ready_job queue")
	}
	go graceful.GetManager().RunWithCancel(jobEmitterQueue)

	notify_service.RegisterNotifier(NewNotifier())
	return initGlobalRunnerToken(ctx)
}
