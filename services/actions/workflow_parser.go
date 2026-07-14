// Package actions provides workflow parsing utilities.
// Extended to parse permissions block. Modified by LAC | Ludwig investing
package actions

import (
	"gopkg.in/yaml.v2"
)

// RawWorkflow represents the structure of a workflow YAML file.
// Extended to include permissions. Modified by LAC | Ludwig investing
type RawWorkflow struct {
	Name        string                       `yaml:"name"`
	On          interface{}                  `yaml:"on"`
	Permissions *WorkflowPermissions         `yaml:"permissions"`
	Jobs        map[string]*RawJob           `yaml:"jobs"`
}

// WorkflowPermissions defines the permissions block in a workflow or job.
// Added to support configurable permissions. Modified by LAC | Ludwig investing
type WorkflowPermissions struct {
	Actions            string `yaml:"actions"`
	Checks             string `yaml:"checks"`
	Contents           string `yaml:"contents"`
	Deployments        string `yaml:"deployments"`
	Issues             string `yaml:"issues"`
	Packages           string `yaml:"packages"`
	PullRequests       string `yaml:"pull-requests"`
	RepositoryProjects string `yaml:"repository-projects"`
	Statuses           string `yaml:"statuses"`
}

// RawJob represents a job in the workflow.
// Extended to include permissions. Modified by LAC | Ludwig investing
type RawJob struct {
	Name        string                `yaml:"name"`
	RunsOn      interface{}           `yaml:"runs-on"`
	Permissions *WorkflowPermissions  `yaml:"permissions"`
	Steps       []*RawStep            `yaml:"steps"`
	// other fields...
}

// RawStep represents a step in a job.
type RawStep struct {
	Name   string            `yaml:"name"`
	Uses   string            `yaml:"uses"`
	With   map[string]string `yaml:"with"`
	// ...
}

// ParseWorkflow parses a workflow YAML file.
// Modified to extract permissions. Modified by LAC | Ludwig investing
func ParseWorkflow(content []byte) (*RawWorkflow, error) {
	wf := &RawWorkflow{}
	err := yaml.Unmarshal(content, wf)
	if err != nil {
		return nil, err
	}
	return wf, nil
}

// GetJobPermissions returns the effective permissions for a job by merging workflow and job permissions.
// Added to support job-level permissions. Modified by LAC | Ludwig investing
func GetJobPermissions(wf *RawWorkflow, jobID string) map[Scope]Permission {
	job, ok := wf.Jobs[jobID]
	if !ok {
		return nil
	}
	// Merge: job permissions override workflow permissions
	merged := make(map[Scope]Permission)
	if wf.Permissions != nil {
		mergePermissions(merged, wf.Permissions)
	}
	if job.Permissions != nil {
		mergePermissions(merged, job.Permissions)
	}
	return merged
}

// mergePermissions converts a WorkflowPermissions struct into a map and merges into target.
// Added to support merging. Modified by LAC | Ludwig investing
func mergePermissions(target map[Scope]Permission, wp *WorkflowPermissions) {
	if wp.Actions != "" {
		target[ScopeActions] = PermissionFromString(wp.Actions)
	}
	if wp.Checks != "" {
		target[ScopeChecks] = PermissionFromString(wp.Checks)
	}
	if wp.Contents != "" {
		target[ScopeContents] = PermissionFromString(wp.Contents)
	}
	if wp.Deployments != "" {
		target[ScopeDeployments] = PermissionFromString(wp.Deployments)
	}
	if wp.Issues != "" {
		target[ScopeIssues] = PermissionFromString(wp.Issues)
	}
	if wp.Packages != "" {
		target[ScopePackages] = PermissionFromString(wp.Packages)
	}
	if wp.PullRequests != "" {
		target[ScopePullRequests] = PermissionFromString(wp.PullRequests)
	}
	if wp.RepositoryProjects != "" {
		target[ScopeRepositoryProjects] = PermissionFromString(wp.RepositoryProjects)
	}
	if wp.Statuses != "" {
		target[ScopeStatuses] = PermissionFromString(wp.Statuses)
	}
}
