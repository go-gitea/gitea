package core

// BuildStatus represents a build status
type BuildStatus string
type RunnerStatus string

// enumerate all the statuses of bot build
const (
	// Build status
	StatusSkipped  BuildStatus = "skipped"
	StatusBlocked  BuildStatus = "blocked"
	StatusDeclined BuildStatus = "declined"
	StatusWaiting  BuildStatus = "waiting_on_dependencies"
	StatusPending  BuildStatus = "pending"
	StatusRunning  BuildStatus = "running"
	StatusPassing  BuildStatus = "success"
	StatusFailing  BuildStatus = "failure"
	StatusKilled   BuildStatus = "killed"
	StatusError    BuildStatus = "error"

	// Runner status
	StatusIdle    RunnerStatus = "idle"
	StatusActive  RunnerStatus = "active"
	StatusOffline RunnerStatus = "offline"
)

func (status BuildStatus) IsPending() bool {
	return status == StatusPending
}

func (status BuildStatus) IsRunning() bool {
	return status == StatusRunning
}

func (status BuildStatus) IsFailed() bool {
	return status == StatusFailing || status == StatusKilled || status == StatusError
}

func (status BuildStatus) IsSuccess() bool {
	return status == StatusPassing
}
