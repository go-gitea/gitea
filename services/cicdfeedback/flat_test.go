package cicdfeedback

import (
	"testing"

	"github.com/6543/cicd_feedback"
	"github.com/stretchr/testify/assert"
)

func TestFlatWorkflows(t *testing.T) {
	workflows := FlatWorkflows([]cicd_feedback.Workflow{{
		ID:     "w1",
		Name:   "lint",
		Status: cicd_feedback.StatusSuccess,
		Steps: []cicd_feedback.Step{{
			ID:   "s1",
			Name: "clone",
		}},
	}, {
		ID:     "w2",
		Name:   "test",
		Status: cicd_feedback.StatusDeclined,
		SubWorkflows: []cicd_feedback.Workflow{{
			ID:   "sw1",
			Name: "test-backend",
			Steps: []cicd_feedback.Step{{
				ID:   "s2",
				Name: "clone2",
			}},
		}},
	}, {
		ID:     "w3",
		Name:   "frontend",
		Status: cicd_feedback.StatusDeclined,
		SubWorkflows: []cicd_feedback.Workflow{{
			ID:   "sw2",
			Name: "test-frontend",
			SubWorkflows: []cicd_feedback.Workflow{{
				ID:   "ssw2",
				Name: "e2e",
				Steps: []cicd_feedback.Step{{
					ID:   "s3",
					Name: "clone3",
				}},
			}},
		}},
	}})
	assert.EqualValues(t, []cicd_feedback.Workflow{{
		ID:     "simulated_workflow/w1",
		Name:   "lint",
		Status: cicd_feedback.StatusSuccess,
		SubWorkflows: []cicd_feedback.Workflow{{
			ID:     "w1",
			Name:   "lint",
			Status: cicd_feedback.StatusSuccess,
			Steps: []cicd_feedback.Step{{
				ID:   "s1",
				Name: "clone",
			}},
		}},
	}, {
		ID:     "w2",
		Name:   "test",
		Status: cicd_feedback.StatusDeclined,
		SubWorkflows: []cicd_feedback.Workflow{{
			ID:   "sw1",
			Name: "test-backend",
			Steps: []cicd_feedback.Step{{
				ID:   "s2",
				Name: "clone2",
			}},
		}},
	}, {
		ID:     "w3",
		Name:   "frontend",
		Status: cicd_feedback.StatusDeclined,
		SubWorkflows: []cicd_feedback.Workflow{{
			ID:   "sw2",
			Name: "test-frontend",
			Steps: []cicd_feedback.Step{{
				ID:   "s3",
				Name: "test-frontend > clone3",
			}},
		}},
	}}, workflows)
}
