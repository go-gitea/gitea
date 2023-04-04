package structs

import "time"

type CheckRunStatus string

const (
	// CheckRunStatusQueued queued
	CheckRunStatusQueued CheckRunStatus = "queued"
	// CheckRunStatusInProgress in_progress
	CheckRunStatusInProgress CheckRunStatus = "in_progress"
	// CheckRunStatusQueued completed
	CheckRunStatusCompleted CheckRunStatus = "completed"
)

type CheckRunConclusion string

const (
	// CheckRunConclusionActionRequired action_required
	CheckRunConclusionActionRequired CheckRunConclusion = "action_required"
	// CheckRunConclusionCancelled cancelled
	CheckRunConclusionCancelled CheckRunConclusion = "cancelled"
	// CheckRunConclusionFailure failure
	CheckRunConclusionFailure CheckRunConclusion = "failure"
	// CheckRunConclusionNeutral neutral
	CheckRunConclusionNeutral CheckRunConclusion = "neutral"
	// CheckRunConclusionNeutral success
	CheckRunConclusionSuccess CheckRunConclusion = "success"
	// CheckRunConclusionSkipped skipped
	CheckRunConclusionSkipped CheckRunConclusion = "skipped"
	// CheckRunConclusionStale stale
	CheckRunConclusionStale CheckRunConclusion = "stale"
	// CheckRunConclusionTimedOut timed_out
	CheckRunConclusionTimedOut CheckRunConclusion = "timed_out"
)

type CheckRunAnnotationLevel string

const (
	// CheckRunAnnotationLevelNotice notice
	CheckRunAnnotationLevelNotice CheckRunAnnotationLevel = "notice"
	// CheckRunAnnotationWarning warning
	CheckRunAnnotationWarning CheckRunAnnotationLevel = "warning"
	// CheckRunAnnotationLevelFailure failure
	CheckRunAnnotationLevelFailure CheckRunAnnotationLevel = "failure"
)

// CheckRunAnnotation represents an annotation object for a CheckRun output.
type CheckRunAnnotation struct {
	Path            *string                 `json:"path,omitempty"`
	StartLine       *int                    `json:"start_line,omitempty"`
	EndLine         *int                    `json:"end_line,omitempty"`
	StartColumn     *int                    `json:"start_column,omitempty"`
	EndColumn       *int                    `json:"end_column,omitempty"`
	AnnotationLevel CheckRunAnnotationLevel `json:"annotation_level"`
	Message         string                  `json:"message"`
	Title           string                  `json:"title"`
	RawDetails      *string                 `json:"raw_details,omitempty"`
}

// CheckRunOutput represents the output of a CheckRun.
type CheckRunOutput struct {
	Title            string                `json:"title"`
	Summary          string                `json:"summary"`
	Text             *string               `json:"text,omitempty"`
	AnnotationsCount *int                  `json:"annotations_count,omitempty"`
	AnnotationsURL   *string               `json:"annotations_url,omitempty"`
	Annotations      []*CheckRunAnnotation `json:"annotations,omitempty"`
}

// CreateCheckRunOptions options needed to create a CheckRun.
type CreateCheckRunOptions struct {
	Name       string              `json:"name"`
	HeadSHA    string              `json:"head_sha"`
	DetailsURL *string             `json:"details_url,omitempty"`
	ExternalID *string             `json:"external_id,omitempty"`
	Status     CheckRunStatus      `json:"status"`
	Conclusion *CheckRunConclusion `json:"conclusion,omitempty"`
	// swagger:strfmt date-time
	StartedAt *time.Time `json:"started_at,omitempty"`
	// swagger:strfmt date-time
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Output      *CheckRunOutput `json:"output,omitempty"`
}

// CheckRun represents a check run on a repository
type CheckRun struct {
	ID         int64   `json:"id"`
	NodeID     string  `json:"node_id"`
	HeadSHA    string  `json:"head_sha"`
	ExternalID *string `json:"external_id,omitempty"`
	URL        *string `json:"url,omitempty"`
	DetailsURL *string `json:"details_url,omitempty"`
	Status     *string `json:"status,omitempty"`
	Conclusion *string `json:"conclusion,omitempty"`
	// swagger:strfmt date-time
	StartedAt *time.Time `json:"started_at,omitempty"`
	// swagger:strfmt date-time
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	Output       *CheckRunOutput `json:"output,omitempty"`
	Name         *string         `json:"name,omitempty"`
	PullRequests []*PullRequest  `json:"pull_requests,omitempty"`
}
