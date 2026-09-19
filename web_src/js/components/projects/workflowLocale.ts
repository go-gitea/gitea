// Every translated string the workflow components need. The server renders them
// into `data-locale-*` attributes (see templates/projects/workflows.tmpl) because
// the frontend has no access to the translation catalogue; this module is the one
// place the shape is declared, so the components can share it instead of each
// restating all of the keys.
export type WorkflowLocale = {
  defaultWorkflows: string;
  moveToColumn: string;
  viewWorkflowConfiguration: string;
  configureWorkflow: string;
  when: string;
  runWhen: string;
  filters: string;
  applyTo: string;
  whenMovedFromColumn: string;
  whenMovedToColumn: string;
  onlyIfHasLabels: string;
  actions: string;
  addLabels: string;
  removeLabels: string;
  anyLabel: string;
  anyColumn: string;
  issueState: string;
  none: string;
  noChange: string;
  edit: string;
  delete: string;
  save: string;
  clone: string;
  cancel: string;
  disable: string;
  disabled: string;
  enabled: string;
  enable: string;
  issuesAndPullRequests: string;
  issuesOnly: string;
  pullRequestsOnly: string;
  selectColumn: string;
  closeIssue: string;
  reopenIssue: string;
  saveWorkflowFailed: string;
  updateWorkflowFailed: string;
  deleteWorkflowFailed: string;
  unexpectedResponseFormat: string;
  unexpectedError: string;
  atLeastOneActionRequired: string;
  cloneTooltip: string;
  deleteConfirm: string;
};

export function readWorkflowLocale(el: Element): WorkflowLocale {
  return {
    defaultWorkflows: el.getAttribute('data-locale-default-workflows')!,
    moveToColumn: el.getAttribute('data-locale-move-to-column')!,
    viewWorkflowConfiguration: el.getAttribute('data-locale-view-workflow-configuration')!,
    configureWorkflow: el.getAttribute('data-locale-configure-workflow')!,
    when: el.getAttribute('data-locale-when')!,
    runWhen: el.getAttribute('data-locale-run-when')!,
    filters: el.getAttribute('data-locale-filters')!,
    applyTo: el.getAttribute('data-locale-apply-to')!,
    whenMovedFromColumn: el.getAttribute('data-locale-when-moved-from-column')!,
    whenMovedToColumn: el.getAttribute('data-locale-when-moved-to-column')!,
    onlyIfHasLabels: el.getAttribute('data-locale-only-if-has-labels')!,
    actions: el.getAttribute('data-locale-actions')!,
    addLabels: el.getAttribute('data-locale-add-labels')!,
    removeLabels: el.getAttribute('data-locale-remove-labels')!,
    anyLabel: el.getAttribute('data-locale-any-label')!,
    anyColumn: el.getAttribute('data-locale-any-column')!,
    issueState: el.getAttribute('data-locale-issue-state')!,
    none: el.getAttribute('data-locale-none')!,
    noChange: el.getAttribute('data-locale-no-change')!,
    edit: el.getAttribute('data-locale-edit')!,
    delete: el.getAttribute('data-locale-delete')!,
    save: el.getAttribute('data-locale-save')!,
    clone: el.getAttribute('data-locale-clone')!,
    cancel: el.getAttribute('data-locale-cancel')!,
    disable: el.getAttribute('data-locale-disable')!,
    disabled: el.getAttribute('data-locale-disabled')!,
    enabled: el.getAttribute('data-locale-enabled')!,
    enable: el.getAttribute('data-locale-enable')!,
    issuesAndPullRequests: el.getAttribute('data-locale-issues-and-pull-requests')!,
    issuesOnly: el.getAttribute('data-locale-issues-only')!,
    pullRequestsOnly: el.getAttribute('data-locale-pull-requests-only')!,
    selectColumn: el.getAttribute('data-locale-select-column')!,
    closeIssue: el.getAttribute('data-locale-close-issue')!,
    reopenIssue: el.getAttribute('data-locale-reopen-issue')!,
    saveWorkflowFailed: el.getAttribute('data-locale-save-workflow-failed')!,
    updateWorkflowFailed: el.getAttribute('data-locale-update-workflow-failed')!,
    deleteWorkflowFailed: el.getAttribute('data-locale-delete-workflow-failed')!,
    unexpectedResponseFormat: el.getAttribute('data-locale-unexpected-response-format')!,
    unexpectedError: el.getAttribute('data-locale-unexpected-error')!,
    atLeastOneActionRequired: el.getAttribute('data-locale-at-least-one-action-required')!,
    cloneTooltip: el.getAttribute('data-locale-clone-tooltip')!,
    deleteConfirm: el.getAttribute('data-locale-delete-confirm')!,
  };
}
