package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	bot_model "code.gitea.io/gitea/models/bot"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/bot/runner"
	"code.gitea.io/gitea/modules/git"

	//"code.gitea.io/gitea/modules/log"
	//"code.gitea.io/gitea/modules/util"

	"github.com/nektos/act/pkg/model"
	act_runner "github.com/nektos/act/pkg/runner"
)

func init() {
	runner.RegisterRunnerType(new(GithubRunner))
}

type GithubRunner struct {
}

func (gw *GithubRunner) Name() string {
	return "github"
}

func (gw *GithubRunner) Detect(commit *git.Commit, event webhook.HookEventType, ref string) (bool, string, error) {
	tree, err := commit.SubTree(".github/workflow")
	if err != nil {
		return false, "", err
	}
	entries, err := tree.ListEntries()
	if err != nil {
		return false, "", err
	}

	var content = make(map[string]string)
	for _, entry := range entries {
		blob := entry.Blob()
		rd, err := blob.DataAsync()
		if err != nil {
			return false, "", err
		}

		bs, err := io.ReadAll(rd)
		rd.Close()
		if err != nil {
			return false, "", err
		}
		content[entry.Name()] = string(bs)
	}

	res, err := json.Marshal(content)
	if err != nil {
		return false, "", err
	}
	return true, string(res), nil
}

func (gw *GithubRunner) Run(task *bot_model.Task) error {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("%d", task.ID))
	if err != nil {
		return err
	}

	var files = make(map[string]string)
	if err := json.Unmarshal([]byte(task.Content), &files); err != nil {
		return err
	}
	for name, content := range files {
		f, err := os.Create(filepath.Join(tmpDir, name))
		if err != nil {
			return err
		}
		if _, err := f.WriteString(content); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	repo, err := repo_model.GetRepositoryByID(task.RepoID)
	if err != nil {
		return err
	}

	evtFilePath := filepath.Join(tmpDir, "event.json")
	evtFile, err := os.Create(evtFilePath)
	if err != nil {
		return err
	}

	if _, err := evtFile.WriteString(task.EventPayload); err != nil {
		evtFile.Close()
		return err
	}
	evtFile.Close()

	planner, err := model.NewWorkflowPlanner(tmpDir, false)
	if err != nil {
		return err
	}
	plan := planner.PlanEvent(task.Event)

	actor, err := user_model.GetUserByID(task.TriggerUserID)
	if err != nil {
		return err
	}

	// run the plan
	config := &act_runner.Config{
		Actor:         actor.LoginName,
		EventName:     task.Event,
		EventPath:     evtFilePath,
		DefaultBranch: repo.DefaultBranch,
		/*ForcePull:             input.forcePull,
		ForceRebuild:          input.forceRebuild,
		ReuseContainers:       input.reuseContainers,
		Workdir:               input.Workdir(),
		BindWorkdir:           input.bindWorkdir,
		LogOutput:             !input.noOutput,*/
		//Env:                   envs,
		Secrets: map[string]string{
			"token": "614e597274a527b6fcf6ddfe45def79430126f08",
		},
		//InsecureSecrets:       input.insecureSecrets,*/
		Platforms: map[string]string{
			"ubuntu-latest": "node:12-buster-slim",
			"ubuntu-20.04":  "node:12-buster-slim",
			"ubuntu-18.04":  "node:12-buster-slim",
		},
		/*Privileged:            input.privileged,
		UsernsMode:            input.usernsMode,
		ContainerArchitecture: input.containerArchitecture,
		ContainerDaemonSocket: input.containerDaemonSocket,
		UseGitIgnore:          input.useGitIgnore,*/
		GitHubInstance: "gitea.com",
		/*ContainerCapAdd:       input.containerCapAdd,
		ContainerCapDrop:      input.containerCapDrop,
		AutoRemove:            input.autoRemove,
		ArtifactServerPath:    input.artifactServerPath,
		ArtifactServerPort:    input.artifactServerPort,*/
	}
	r, err := act_runner.New(config)
	if err != nil {
		return err
	}

	//ctx, cancel := context.WithTimeout(context.Background(), )

	executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
		//cancel()
		return nil
	})
	return executor(context.Background())
}
