package gitea

import (
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/bot/runner"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/json"
)

func init() {
	runner.RegisterRunnerType(new(GiteaRunner))
}

type GiteaRunner struct {
}

func (gw *GiteaRunner) Name() string {
	return "gitea"
}

func (gw *GiteaRunner) Detect(commit *git.Commit, event webhook.HookEventType, ref string) (bool, string, error) {
	tree, err := commit.SubTree(".gitea/workflow")
	if err != nil {
		return false, "", err
	}
	entries, err := tree.ListEntries()
	if err != nil {
		return false, "", err
	}

	var wfs []*Workflow
	for _, entry := range entries {
		blob := entry.Blob()
		rd, err := blob.DataAsync()
		if err != nil {
			return false, "", err
		}
		defer rd.Close()
		wf, err := ReadWorkflow(rd)
		if err != nil {
			log.Error("ReadWorkflow file %s failed: %v", entry.Name(), err)
			continue
		}

		// FIXME: we have to convert the event type to github known name
		if !util.IsStringInSlice(string(event), wf.On()) {
			continue
		}

		wfs = append(wfs, wf)
	}

	wfBs, err := json.Marshal(wfs)
	if err != nil {
		return false, "", err
	}
	return true, string(wfBs), nil
}
