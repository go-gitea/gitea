package cicdfeedback

import "github.com/6543/cicd_feedback"

const (
	simulatedWorkflow = "simulated_workflow/"
	maxLevel          = 5
)

// FlatWorkflows converts the cicd_feedback.Workflow witch can have any level of hierarchy into a three level one
func FlatWorkflows(workflows []cicd_feedback.Workflow) []cicd_feedback.Workflow {
	for i := range workflows {
		// gitea can not have steps in level 0 so we move them in a sub workflow
		if workflows[i].Steps != nil {
			workflows[i].SubWorkflows = []cicd_feedback.Workflow{{
				ID:     workflows[i].ID,
				Name:   workflows[i].Name,
				Status: workflows[i].Status,
				Steps:  workflows[i].Steps,
			}}
			workflows[i].ID = simulatedWorkflow + workflows[i].ID
			workflows[i].Steps = nil
		}

		// ok and all this sub workflows should have only steps
		for j := range workflows[i].SubWorkflows {
			workflows[i].SubWorkflows[j].Steps = flatSubWorkflowsToSteps(&workflows[i].SubWorkflows[j], 0, "")
			workflows[i].SubWorkflows[j].SubWorkflows = nil // would be invalid but we do sanitize it non the less
		}
	}
	return workflows
}

func flatSubWorkflowsToSteps(workflows *cicd_feedback.Workflow, level int, prefix string) []cicd_feedback.Step {
	if level > maxLevel {
		return nil
	} else if workflows.Steps != nil {
		if prefix != "" {
			for i := range workflows.Steps {
				workflows.Steps[i].Name = prefix + workflows.Steps[i].Name
			}
		}
		return workflows.Steps
	}

	var result []cicd_feedback.Step
	for _, subFlow := range workflows.SubWorkflows {
		result = append(result, flatSubWorkflowsToSteps(&subFlow, level+1, workflows.Name+" > ")...)
	}
	return result
}
