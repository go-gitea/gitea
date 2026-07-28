// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"
	notify_service "gitea.dev/services/notify"
)

func init() {
	notify_service.RegisterNotifier(&workflowNotifier{})
}

type workflowNotifier struct {
	notify_service.NullNotifier
}

var _ notify_service.Notifier = &workflowNotifier{}

// NewNotifier create a new workflowNotifier notifier
func NewNotifier() notify_service.Notifier {
	return &workflowNotifier{}
}

func (m *workflowNotifier) NewIssue(ctx context.Context, issue *issues_model.Issue, mentions []*user_model.User) {
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("NewIssue: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("NewIssue: LoadProject: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("NewIssue: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemOpened event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemOpened {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (m *workflowNotifier) NewPullRequest(ctx context.Context, pr *issues_model.PullRequest, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("NewPullRequest: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue
	m.NewIssue(ctx, issue, mentions)
}

func (m *workflowNotifier) IssueChangeStatus(ctx context.Context, doer *user_model.User, commitID string, issue *issues_model.Issue, actionComment *issues_model.Comment, isClosed bool) {
	// Skip state changes triggered by workflow actions to prevent cascade loops
	// (same guard as feed/notifier.go).
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadRepo: %v", err)
		return
	}
	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("IssueChangeStatus: LoadProject: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("IssueChangeStatus: FindWorkflowsByProjectID: %v", err)
			return
		}

		workflowEvent := util.Iif(isClosed, project_model.WorkflowEventItemClosed, project_model.WorkflowEventItemReopened)
		// Find workflows for the specific event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == workflowEvent {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (*workflowNotifier) IssueChangeProjects(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldProjectColumnMap map[int64]int64, newProjects []*project_model.Project) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	// Both loops below reach fireIssueWorkflow, whose actions create timeline
	// comments against issue.Repo, so the repo has to be loaded for either path.
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeProjects: LoadRepo: %v", err)
		return
	}

	// Collect into slices sorted by project ID: iterating a map would run the
	// projects' workflows in a different order on every request, which is
	// observable whenever two projects' workflows touch the same item.
	var addedProjects []*project_model.Project
	for _, newProject := range newProjects {
		// Use key presence check; column ID 0 is technically valid.
		if _, ok := oldProjectColumnMap[newProject.ID]; ok {
			continue
		}
		addedProjects = append(addedProjects, newProject)
	}
	slices.SortFunc(addedProjects, func(a, b *project_model.Project) int {
		return cmp.Compare(a.ID, b.ID)
	})

	var removedProjectIDs []int64
	for projectID := range oldProjectColumnMap {
		found := false
		for _, newProject := range newProjects {
			if newProject.ID == projectID {
				found = true
				break
			}
		}
		if !found {
			removedProjectIDs = append(removedProjectIDs, projectID)
		}
	}
	slices.Sort(removedProjectIDs)

	for _, removedProjectID := range removedProjectIDs {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, removedProjectID)
		if err != nil {
			log.Error("IssueChangeProjects: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemRemovedFromProject event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemRemovedFromProject {
				fireIssueWorkflow(ctx, workflow, issue, removedProjectID, 0, 0)
			}
		}
	}

	for _, newProject := range addedProjects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, newProject.ID)
		if err != nil {
			log.Error("IssueChangeProjects: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the ItemOpened event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventItemAddedToProject {
				fireIssueWorkflow(ctx, workflow, issue, newProject.ID, 0, 0)
			}
		}
	}
}

func (*workflowNotifier) IssueChangeProjectColumn(ctx context.Context, doer *user_model.User, issue *issues_model.Issue, oldColumnID, newColumnID int64) {
	// Skip column moves triggered by workflow actions to prevent cascade loops.
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if oldColumnID == newColumnID {
		return
	}
	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("IssueChangeProjectColumn: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("IssueChangeProjectColumn: LoadProjects: %v", err)
		return
	}

	newColumn, err := project_model.GetColumn(ctx, newColumnID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: GetColumn: %v", err)
		return
	}

	// project_issue.project_board_id is stored as 0 for issues sitting in a
	// project's default/unassigned column (see LoadProjectIssueColumnMap), so an
	// oldColumnID of 0 here means "the default column", not "no column". Resolve
	// it to the real column so the lookup below doesn't fail and skip the workflow.
	var oldColumn *project_model.Column
	if oldColumnID == 0 {
		project, err := project_model.GetProjectByID(ctx, newColumn.ProjectID)
		if err != nil {
			log.Error("IssueChangeProjectColumn: GetProjectByID: %v", err)
			return
		}
		oldColumn, err = project.MustDefaultColumn(ctx)
		if err != nil {
			log.Error("IssueChangeProjectColumn: MustDefaultColumn: %v", err)
			return
		}
		oldColumnID = oldColumn.ID
	} else {
		oldColumn, err = project_model.GetColumn(ctx, oldColumnID)
		if err != nil {
			log.Error("IssueChangeProjectColumn: GetColumn: %v", err)
			return
		}
	}
	// The early oldColumnID == newColumnID check above only catches a no-op move
	// when the raw board_id values already matched. Once oldColumnID==0 has been
	// resolved to the project's real default column, it can turn out to equal
	// newColumnID too (e.g. reordering within the default column), so the no-op
	// check has to be repeated here against the resolved ID.
	if oldColumnID == newColumnID {
		return
	}
	if oldColumn.ProjectID != newColumn.ProjectID {
		return
	}
	found := false
	for _, project := range issue.Projects {
		if project.ID == oldColumn.ProjectID {
			found = true
			break
		}
	}
	if !found {
		return
	}

	workflows, err := project_model.FindWorkflowsByProjectID(ctx, oldColumn.ProjectID)
	if err != nil {
		log.Error("IssueChangeProjectColumn: FindWorkflowsByProjectID: %v", err)
		return
	}

	// Find workflows for the ItemColumnChanged event
	for _, workflow := range workflows {
		if workflow.WorkflowEvent == project_model.WorkflowEventItemColumnChanged {
			fireIssueWorkflow(ctx, workflow, issue, oldColumn.ProjectID, oldColumnID, newColumnID)
		}
	}
}

func (*workflowNotifier) MergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("MergePullRequest: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("MergePullRequest: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("MergePullRequest: LoadProjects: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("MergePullRequest: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the PullRequestMerged event
		for _, workflow := range workflows {
			if workflow.WorkflowEvent == project_model.WorkflowEventPullRequestMerged {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func (m *workflowNotifier) AutoMergePullRequest(ctx context.Context, doer *user_model.User, pr *issues_model.PullRequest) {
	if issues_model.IsProjectWorkflowDoer(doer) {
		return
	}
	m.MergePullRequest(ctx, doer, pr)
}

func (*workflowNotifier) PullRequestReview(ctx context.Context, pr *issues_model.PullRequest, review *issues_model.Review, comment *issues_model.Comment, mentions []*user_model.User) {
	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("PullRequestReview: LoadIssue: %v", err)
		return
	}
	issue := pr.Issue

	if err := issue.LoadRepo(ctx); err != nil {
		log.Error("PullRequestReview: LoadRepo: %v", err)
		return
	}

	if err := issue.LoadProjects(ctx); err != nil {
		log.Error("PullRequestReview: LoadProjects: %v", err)
		return
	}
	if len(issue.Projects) == 0 {
		return
	}

	for _, project := range issue.Projects {
		workflows, err := project_model.FindWorkflowsByProjectID(ctx, project.ID)
		if err != nil {
			log.Error("PullRequestReview: FindWorkflowsByProjectID: %v", err)
			return
		}

		// Find workflows for the PullRequestMerged event
		for _, workflow := range workflows {
			if (workflow.WorkflowEvent == project_model.WorkflowEventCodeChangesRequested && review.Type == issues_model.ReviewTypeReject) ||
				(workflow.WorkflowEvent == project_model.WorkflowEventCodeReviewApproved && review.Type == issues_model.ReviewTypeApprove) {
				fireIssueWorkflow(ctx, workflow, issue, project.ID, 0, 0)
			}
		}
	}
}

func fireIssueWorkflow(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue, projectID, sourceColumnID, targetColumnID int64) {
	if !workflow.Enabled {
		return
	}

	// Load issue labels for labels filter
	if err := issue.LoadLabels(ctx); err != nil {
		log.Error("LoadLabels: %v", err)
		return
	}

	if !matchWorkflowsFilters(workflow, issue, sourceColumnID, targetColumnID) {
		return
	}

	executeWorkflowActions(ctx, workflow, issue, projectID)
}

// matchWorkflowsFilters checks if the issue matches all filters of the workflow.
//
// Filters fail closed: anything that cannot be positively evaluated -- a filter type
// the event does not support, an unparsable or non-positive ID, or a column the
// caller never supplied -- makes the workflow NOT match. A restrictive filter must
// never silently degrade into "match everything", because that would run a
// workflow's actions on items the user deliberately excluded.
func matchWorkflowsFilters(workflow *project_model.Workflow, issue *issues_model.Issue, sourceColumnID, targetColumnID int64) bool {
	// A stored filter that the event does not declare cannot be evaluated meaningfully
	// (e.g. a source_column filter on item_opened, where no column IDs are ever
	// supplied), so treat its presence as a non-match rather than skipping over it.
	// The capability matrix is the same source of truth the write paths validate
	// against, so this only rejects rows written by a different version or by hand.
	capabilities := project_model.GetWorkflowEventCapabilities()[workflow.WorkflowEvent]
	allowedFilters := make(map[project_model.WorkflowFilterType]bool, len(capabilities.AvailableFilters))
	for _, filterType := range capabilities.AvailableFilters {
		allowedFilters[filterType] = true
	}

	for _, filter := range workflow.WorkflowFilters {
		if !allowedFilters[filter.Type] {
			log.Error("Workflow %d: filter type %q is not supported by event %q, refusing to match", workflow.ID, filter.Type, workflow.WorkflowEvent)
			return false
		}

		switch filter.Type {
		case project_model.WorkflowFilterTypeIssueType:
			// An empty value is the legitimate "applies to both issues and pull
			// requests" setting, so matching everything here is intended.
			if filter.Value == "" {
				continue
			}
			// Filter value can be "issue" or "pull_request", anything else never matches
			switch filter.Value {
			case project_model.WorkflowIssueTypeIssue:
				if issue.IsPull {
					return false
				}
			case project_model.WorkflowIssueTypePullRequest:
				if !issue.IsPull {
					return false
				}
			default:
				log.Error("Invalid issue type filter value: %s", filter.Value)
				return false
			}
		case project_model.WorkflowFilterTypeTargetColumn:
			if !matchColumnFilter(workflow, filter, targetColumnID) {
				return false
			}
		case project_model.WorkflowFilterTypeSourceColumn:
			if !matchColumnFilter(workflow, filter, sourceColumnID) {
				return false
			}
		case project_model.WorkflowFilterTypeLabels:
			labelID, err := strconv.ParseInt(filter.Value, 10, 64)
			if err != nil || labelID <= 0 {
				log.Error("Invalid label ID: %s", filter.Value)
				return false
			}
			// Check if issue has this label
			hasLabel := false
			for _, label := range issue.Labels {
				if label.ID == labelID {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				return false
			}
		default:
			// Unreachable while the capability matrix and this switch agree, but kept
			// as the backstop for the next filter type: a type added to the matrix and
			// forgotten here must not fall through the switch and match everything.
			log.Error("Unsupported filter type: %s", filter.Type)
			return false
		}
	}
	return true
}

// matchColumnFilter compares a stored source/target column filter against the column
// ID the notifier resolved for this event. Both sides must be a real column ID: the
// notifier resolves the "default column" sentinel 0 to the project's actual default
// column before matching (see IssueChangeProjectColumn), so a 0 arriving here means
// the caller supplied no column at all and the filter cannot be satisfied.
func matchColumnFilter(workflow *project_model.Workflow, filter project_model.WorkflowFilter, columnID int64) bool {
	filterColumnID, err := strconv.ParseInt(filter.Value, 10, 64)
	if err != nil || filterColumnID <= 0 {
		log.Error("Workflow %d: invalid %s filter value %q", workflow.ID, filter.Type, filter.Value)
		return false
	}
	if columnID <= 0 {
		log.Error("Workflow %d: event %q supplied no column for the %s filter, refusing to match", workflow.ID, workflow.WorkflowEvent, filter.Type)
		return false
	}
	return columnID == filterColumnID
}

// resolveWorkflowActionLabel loads the label referenced by an add_labels/remove_labels
// action and re-checks that it still belongs to the workflow's project. Ownership is
// validated when the workflow is saved, but a repository transfer (or a label moved
// between orgs) invalidates that decision afterwards, and GetLabelByID resolves by ID
// alone with no repo/org scoping -- without this check a workflow would keep applying
// a label belonging to the previous owner's org. Returns nil if the label must be
// skipped; the caller logs nothing further.
func resolveWorkflowActionLabel(ctx context.Context, workflow *project_model.Workflow, actionValue string) *issues_model.Label {
	labelID, err := strconv.ParseInt(actionValue, 10, 64)
	if err != nil || labelID <= 0 {
		log.Error("Invalid label ID: %s", actionValue)
		return nil
	}
	// Without the project we cannot establish ownership, so fail closed rather than
	// applying an unscoped label.
	if workflow.Project == nil {
		log.Error("Workflow %d: project not loaded, refusing to apply label %d", workflow.ID, labelID)
		return nil
	}
	// Same ownership rule the write paths use, so execution can never be more
	// permissive than what the editor would have accepted.
	if !CanProjectAddLabel(ctx, workflow.Project, labelID) {
		log.Error("Workflow %d: label %d no longer belongs to project %d, skipping", workflow.ID, labelID, workflow.Project.ID)
		return nil
	}
	label, err := issues_model.GetLabelByID(ctx, labelID)
	if err != nil {
		log.Error("GetLabelByID: %v", err)
		return nil
	}
	return label
}

func executeWorkflowActions(ctx context.Context, workflow *project_model.Workflow, issue *issues_model.Issue, projectID int64) {
	if err := workflow.LoadProject(ctx); err != nil {
		log.Error("LoadProject: %v", err)
	}

	title := "(untitled project)"
	if workflow.Project != nil {
		title = workflow.Project.Title
	}

	doer := issues_model.NewProjectWorkflowDoer(title, workflow.ID, workflow.WorkflowEvent)
	var toAddedLabels, toRemovedLabels []*issues_model.Label

	for _, action := range workflow.WorkflowActions {
		switch action.Type {
		case project_model.WorkflowActionTypeColumn:
			columnID, _ := strconv.ParseInt(action.Value, 10, 64)
			if columnID == 0 {
				log.Error("Invalid column ID: %s", action.Value)
				continue
			}
			column, err := project_model.GetColumnByIDAndProjectID(ctx, columnID, projectID)
			if err != nil {
				log.Error("GetColumnByIDAndProjectID: %v", err)
				continue
			}
			if err := MoveIssueToAnotherColumn(ctx, doer, issue, column); err != nil {
				log.Error("MoveIssueToAnotherColumn: %v", err)
				continue
			}
		case project_model.WorkflowActionTypeAddLabels:
			label := resolveWorkflowActionLabel(ctx, workflow, action.Value)
			if label == nil {
				continue
			}
			toAddedLabels = append(toAddedLabels, label)
		case project_model.WorkflowActionTypeRemoveLabels:
			label := resolveWorkflowActionLabel(ctx, workflow, action.Value)
			if label == nil {
				continue
			}
			toRemovedLabels = append(toRemovedLabels, label)
		case project_model.WorkflowActionTypeIssueState:
			if strings.EqualFold(action.Value, "reopen") {
				if issue.IsClosed {
					if err := issue_service.ReopenIssue(ctx, issue, doer, ""); err != nil {
						log.Error("ReopenIssue: %v", err)
						continue
					}
				}
			} else if strings.EqualFold(action.Value, "close") {
				if !issue.IsClosed {
					if err := issue_service.CloseIssue(ctx, issue, doer, ""); err != nil {
						log.Error("CloseIssue: %v", err)
						continue
					}
				}
			}
		default:
			log.Error("Unsupported action type: %s", action.Type)
		}
	}

	if len(toAddedLabels)+len(toRemovedLabels) > 0 {
		if err := issue_service.AddRemoveLabels(ctx, issue, doer, toAddedLabels, toRemovedLabels); err != nil {
			log.Error("AddRemoveLabels: %v", err)
		}
	}
}
