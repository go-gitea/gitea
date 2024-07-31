package cicdfeedback

import (
	"context"
	"fmt"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/json"

	"github.com/6543/cicd_feedback"
)

func ExtractJobsFromFeedback(ctx context.Context, in *cicd_feedback.PipelineResponse, info *WorkflowInfo, run *actions_model.ActionRun) ([]*actions_model.ActionRunJob, error) {
	if info == nil {
		if err := json.Unmarshal([]byte(run.EventPayload), &info); err != nil {
			return nil, fmt.Errorf("could parse payload: %w", err)
		}
	}

	workflows := FlatWorkflows(in.Workflows)
	var workflow *cicd_feedback.Workflow
	for i := range workflows {
		if workflows[i].ID == info.ID {
			workflow = &workflows[i]
			break
		}
	}
	if workflow == nil {
		return nil, fmt.Errorf("try to extract workflow '%s' but could not find one for %s", run.WorkflowID, run.EventPayload)
	}

	result := make([]*actions_model.ActionRunJob, 0, len(workflow.SubWorkflows))
	for _, job := range workflow.SubWorkflows {
		status, err := ConvertStatus(job.Status)
		if err != nil {
			return nil, err
		}

		stepData, err := json.Marshal(job.Steps)
		if err != nil {
			return nil, err
		}

		result = append(result, &actions_model.ActionRunJob{
			// workflow infos
			RunID:     run.ID,
			Run:       run,
			RepoID:    run.RepoID,
			OwnerID:   run.OwnerID,
			CommitSHA: run.CommitSHA,
			// job infos
			Name:            job.Name,
			Status:          status,
			External:        true,
			WorkflowPayload: stepData,
		})
	}

	return result, nil
}

func ConvertFeedbackToActionRun(ctx context.Context, in *cicd_feedback.PipelineResponse, draft *actions_model.ActionRun, draftInfo *WorkflowInfo) ([]*actions_model.ActionRun, error) {
	workflows := FlatWorkflows(in.Workflows)
	result := make([]*actions_model.ActionRun, 0, len(workflows))

	for _, workflow := range workflows {
		status, err := ConvertStatus(workflow.Status)
		if err != nil {
			return nil, err
		}

		info, err := json.Marshal(&WorkflowInfo{
			ID:            workflow.ID,
			PipelineURI:   draftInfo.PipelineURI,
			Authorization: draftInfo.Authorization,
		})
		if err != nil {
			return nil, err
		}

		title := in.Title
		if in.Title == "" {
			title = workflow.Name
		}
		title = "[extern] " + title

		result = append(result, &actions_model.ActionRun{
			External:     true,
			Title:        title,
			WorkflowID:   workflow.Name,
			EventPayload: string(info),
			Status:       status,
			NeedApproval: in.RequiresManualAction,
			// we copy gitea integration stuff ...
			Event:         draft.Event,
			RepoID:        draft.RepoID,
			Repo:          draft.Repo,
			OwnerID:       draft.OwnerID,
			TriggerUserID: draft.TriggerUserID,
			TriggerEvent:  draft.TriggerEvent,
			CommitSHA:     draft.CommitSHA,
			Ref:           draft.Ref,
		})
	}
	return result, nil
}

func ConvertStatus(in cicd_feedback.Status) (actions_model.Status, error) {
	switch in {
	case cicd_feedback.StatusSuccess:
		return actions_model.StatusSuccess, nil
	case cicd_feedback.StatusFailed:
		return actions_model.StatusFailure, nil
	case cicd_feedback.StatusKilled, cicd_feedback.StatusDeclined:
		return actions_model.StatusCancelled, nil
	case cicd_feedback.StatusSkipped:
		return actions_model.StatusSkipped, nil
	case cicd_feedback.StatusPending:
		return actions_model.StatusWaiting, nil
	case cicd_feedback.StatusRunning:
		return actions_model.StatusRunning, nil
	case cicd_feedback.StatusManual:
		return actions_model.StatusBlocked, nil
	default:
		return actions_model.StatusUnknown, fmt.Errorf("unknown status: %v", in)
	}
}
