package cicdfeedback

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/json"

	"github.com/6543/cicd_feedback"
)

const timeout = time.Second * 2

func DetectHeaders(h http.Header) (*WorkflowInfo, bool) {
	detect := false
	info := &WorkflowInfo{}
	for k, v := range h {
		if strings.EqualFold(k, cicd_feedback.HeaderFeedback) && len(v) > 0 && len(v[0]) > 0 {
			info.PipelineURI = v[0]
			detect = true
		}
		if strings.EqualFold(k, cicd_feedback.HeaderAuthorization) && len(v) > 0 && len(v[0]) > 0 {
			info.PipelineURI = v[0]
		}
		if detect && info.Authorization != "" {
			return info, detect
		}
	}
	return info, detect
}

func GetFeedbackToActionRun(ctx context.Context, draft *actions_model.ActionRun, draftInfo *WorkflowInfo) ([]*actions_model.ActionRun, error) {
	feedbackResponse, err := doAPICall(ctx, draftInfo)
	if err != nil {
		return nil, err
	}
	return ConvertFeedbackToActionRun(ctx, feedbackResponse, draft, draftInfo)
}

func GetExternalRunJobs(ctx context.Context, run *actions_model.ActionRun) ([]*actions_model.ActionRunJob, *WorkflowInfo, error) {
	if !run.External {
		return nil, nil, fmt.Errorf("ActionRun is not an external one")
	}

	info := &WorkflowInfo{}
	if err := json.Unmarshal([]byte(run.EventPayload), &info); err != nil {
		return nil, nil, fmt.Errorf("could parse payload: %w", err)
	}

	feedbackResponse, err := doAPICall(ctx, info)
	if err != nil {
		return nil, nil, err
	}

	jobs, err := ExtractJobsFromFeedback(ctx, feedbackResponse, info, run)
	return jobs, info, err
}

func doAPICall(ctx context.Context, info *WorkflowInfo) (*cicd_feedback.PipelineResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.PipelineURI, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	if info.Authorization != "" {
		req.Header.Set(cicd_feedback.HeaderAuthorization, info.Authorization)
	}

	payload, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch external cicd_feedback pipeline response: %w", err)
	}

	payloadBytes, err := io.ReadAll(payload.Body)
	_ = payload.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("could not read external cicd_feedback pipeline response: %w", err)
	}
	feedbackResponse := cicd_feedback.PipelineResponse{}
	if err := json.Unmarshal(payloadBytes, &feedbackResponse); err != nil {
		return nil, fmt.Errorf("could not deserialize external cicd_feedback pipeline response: %w", err)
	}
	return &feedbackResponse, nil
}

func LoadLogs(ctx context.Context, step *cicd_feedback.Step, info *WorkflowInfo) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	builder := &strings.Builder{}
	for i := range step.Outputs.Logs {

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, step.Outputs.Logs[i].URI, nil)
		if err != nil {
			return "", fmt.Errorf("error creating request: %w", err)
		}

		payload, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("could not fetch external cicd_feedback log: %w", err)
		}

		if _, err := io.Copy(builder, payload.Body); err != nil {
			return "", fmt.Errorf("could not copy external cicd_feedback log: %w", err)
		}
	}
	return builder.String(), nil
}
