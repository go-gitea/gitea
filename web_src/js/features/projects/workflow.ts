import {createApp} from 'vue';
import ProjectWorkflow from '../../components/projects/ProjectWorkflow.vue';

export async function initProjectWorkflow() {
  const workflowDiv = document.querySelector('#project-workflows');
  if (!workflowDiv) return;

  try {
    const locale = {
      defaultWorkflows: workflowDiv.getAttribute('data-locale-default-workflows'),
      moveToColumn: workflowDiv.getAttribute('data-locale-move-to-column'),
      viewWorkflowConfiguration: workflowDiv.getAttribute('data-locale-view-workflow-configuration'),
      configureWorkflow: workflowDiv.getAttribute('data-locale-configure-workflow'),
      when: workflowDiv.getAttribute('data-locale-when'),
      runWhen: workflowDiv.getAttribute('data-locale-run-when'),
      filters: workflowDiv.getAttribute('data-locale-filters'),
      applyTo: workflowDiv.getAttribute('data-locale-apply-to'),
      whenMovedFromColumn: workflowDiv.getAttribute('data-locale-when-moved-from-column'),
      whenMovedToColumn: workflowDiv.getAttribute('data-locale-when-moved-to-column'),
      onlyIfHasLabels: workflowDiv.getAttribute('data-locale-only-if-has-labels'),
      actions: workflowDiv.getAttribute('data-locale-actions'),
      addLabels: workflowDiv.getAttribute('data-locale-add-labels'),
      removeLabels: workflowDiv.getAttribute('data-locale-remove-labels'),
      anyLabel: workflowDiv.getAttribute('data-locale-any-label'),
      anyColumn: workflowDiv.getAttribute('data-locale-any-column'),
      issueState: workflowDiv.getAttribute('data-locale-issue-state'),
      none: workflowDiv.getAttribute('data-locale-none'),
      noChange: workflowDiv.getAttribute('data-locale-no-change'),
      edit: workflowDiv.getAttribute('data-locale-edit'),
      delete: workflowDiv.getAttribute('data-locale-delete'),
      save: workflowDiv.getAttribute('data-locale-save'),
      clone: workflowDiv.getAttribute('data-locale-clone'),
      cancel: workflowDiv.getAttribute('data-locale-cancel'),
      disable: workflowDiv.getAttribute('data-locale-disable'),
      disabled: workflowDiv.getAttribute('data-locale-disabled'),
      enabled: workflowDiv.getAttribute('data-locale-enabled'),
      enable: workflowDiv.getAttribute('data-locale-enable'),
      issuesAndPullRequests: workflowDiv.getAttribute('data-locale-issues-and-pull-requests'),
      issuesOnly: workflowDiv.getAttribute('data-locale-issues-only'),
      pullRequestsOnly: workflowDiv.getAttribute('data-locale-pull-requests-only'),
      selectColumn: workflowDiv.getAttribute('data-locale-select-column'),
      closeIssue: workflowDiv.getAttribute('data-locale-close-issue'),
      reopenIssue: workflowDiv.getAttribute('data-locale-reopen-issue'),
      saveWorkflowFailed: workflowDiv.getAttribute('data-locale-save-workflow-failed'),
      updateWorkflowFailed: workflowDiv.getAttribute('data-locale-update-workflow-failed'),
      deleteWorkflowFailed: workflowDiv.getAttribute('data-locale-delete-workflow-failed'),
      atLeastOneActionRequired: workflowDiv.getAttribute('data-locale-at-least-one-action-required'),
    };

    const View = createApp(ProjectWorkflow, {
      projectLink: workflowDiv.getAttribute('data-project-link'),
      eventID: workflowDiv.getAttribute('data-event-id'),
      locale,
    });
    View.mount(workflowDiv);
  } catch (err) {
    console.error('Project Workflow failed to load', err);
    workflowDiv.textContent = 'Project Workflow failed to load';
  }
}
