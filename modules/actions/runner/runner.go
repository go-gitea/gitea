package runner

import (
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
)

var runnerTypes = make(map[string]RunnerType)

type RunnerType interface {
	Name() string
	Detect(commit *git.Commit, event webhook.HookEventType, ref string) (bool, string, error)
	Run(task *bots_model.Task) error
}

func RegisterRunnerType(runnerType RunnerType) {
	runnerTypes[runnerType.Name()] = runnerType
}

func GetRunnerType(name string) RunnerType {
	return runnerTypes[name]
}

func GetRunnerTypes() map[string]RunnerType {
	return runnerTypes
}
