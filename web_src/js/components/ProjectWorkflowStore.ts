import {computed, reactive} from 'vue';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {trString} from '../modules/i18n.ts';

type ProjectWorkflowFilters = {
  issue_type?: '' | 'issue' | 'pull_request';
  source_column_id?: number;
  target_column_id?: number;
  label_ids?: number[];
};

type ProjectWorkflowActions = {
  column_id?: number;
  add_label_ids?: number[];
  remove_label_ids?: number[];
  issue_state?: '' | 'close' | 'reopen';
};

export type ProjectWorkflow = {
  id: number;
  event: string;
  enabled: boolean;
  filters: ProjectWorkflowFilters;
  actions: ProjectWorkflowActions;
};

export type ProjectWorkflowFilterType = 'issue_type' | 'source_column' | 'target_column' | 'labels';
export type ProjectWorkflowActionType = 'column' | 'add_labels' | 'remove_labels' | 'issue_state';

type ProjectWorkflowEvent = {
  event: string;
  display_name: string;
  supported_filters: ProjectWorkflowFilterType[];
  supported_actions: ProjectWorkflowActionType[];
};

type ProjectColumn = {id: number; title: string};

type ProjectLabel = {id: number; name: string; html: string};

export type ProjectWorkflowData = {
  events: ProjectWorkflowEvent[];
  workflows: ProjectWorkflow[];
  columns: ProjectColumn[];
  labels: ProjectLabel[];
};

type WorkflowForm = {
  filters: Required<ProjectWorkflowFilters>;
  actions: Required<ProjectWorkflowActions>;
};

export type WorkflowRow = {
  key: string;
  event: ProjectWorkflowEvent;
  displayName: string;
} & ({workflow: ProjectWorkflow, draft?: never} | {workflow?: never, draft: WorkflowForm});

type WorkflowDraft = {key: string; event: string; draft: WorkflowForm; sourceKey?: string};

export type ProjectWorkflowLocale = {
  defaultWorkflows: string;
  enable: string;
  disable: string;
  clone: string;
  deleteConfirm: string;
  filters: string;
  applyTo: string;
  issuesAndPullRequests: string;
  issuesOnly: string;
  pullRequestsOnly: string;
  whenMovedFromColumn: string;
  whenMovedToColumn: string;
  anyColumn: string;
  onlyIfHasLabels: string;
  anyLabel: string;
  filterLabels: string;
  clearLabels: string;
  actions: string;
  moveToColumn: string;
  addLabels: string;
  removeLabels: string;
  none: string;
  issueState: string;
  noChange: string;
  closeIssue: string;
  reopenIssue: string;
  summarySource: string;
  summaryTarget: string;
  summaryLabels: string;
  edit: string;
  save: string;
  cancel: string;
  remove: string;
  enabled: string;
  disabled: string;
};

function toWorkflowForm({filters, actions}: Pick<ProjectWorkflow, 'filters' | 'actions'> = {filters: {}, actions: {}}): WorkflowForm {
  return {
    filters: {
      issue_type: filters.issue_type ?? '',
      source_column_id: filters.source_column_id ?? 0,
      target_column_id: filters.target_column_id ?? 0,
      label_ids: [...filters.label_ids ?? []],
    },
    actions: {
      column_id: actions.column_id ?? 0,
      add_label_ids: [...actions.add_label_ids ?? []],
      remove_label_ids: [...actions.remove_label_ids ?? []],
      issue_state: actions.issue_state ?? '',
    },
  };
}

function buildWorkflowRows(events: ProjectWorkflowEvent[], workflows: ProjectWorkflow[], drafts: WorkflowDraft[]): WorkflowRow[] {
  return events.flatMap((event) => {
    const eventDrafts = drafts.filter((d) => d.event === event.event);
    const group: Array<Pick<WorkflowRow, 'key' | 'workflow' | 'draft'>> = [
      ...workflows.filter((wf) => wf.event === event.event).map((workflow) => ({key: String(workflow.id), workflow})),
      ...eventDrafts.filter((d) => d.sourceKey).map(({key, draft}) => ({key, draft})),
    ];
    if (!group.length) group.push(...eventDrafts.map(({key, draft}) => ({key, draft})));
    return group.map((row, idx) => ({
      ...row,
      event,
      displayName: group.length > 1 ? `${event.display_name} #${idx + 1}` : event.display_name,
    } as WorkflowRow));
  });
}

let cloneSeq = 0;

export function createProjectWorkflowStore(props: {el: HTMLElement, projectLink: string, canWrite: boolean, locale: ProjectWorkflowLocale}) {
  const baseUrl = `${props.projectLink}/workflows`;
  const data = window.config.pageData.projectWorkflows!;

  const store = reactive({
    canWrite: props.canWrite,
    ...data,
    drafts: data.events.map((event): WorkflowDraft => ({key: event.event, event: event.event, draft: toWorkflowForm()})),

    selectedKey: '',
    editMode: false,
    saving: false,
    form: toWorkflowForm(),

    rows: computed((): WorkflowRow[] => buildWorkflowRows(store.events, store.workflows, store.drafts)),

    get selected(): WorkflowRow | undefined {
      return store.rows.find((row) => row.key === store.selectedKey);
    },

    get editing(): boolean {
      const row = store.selected;
      if (!store.canWrite || !row) return false;
      return store.editMode || !row.workflow;
    },

    resolve: (key: string): WorkflowRow | undefined => store.rows.find((row) => row.key === key) ?? store.rows.find((row) => row.event.event === key),

    urlFor: (row: WorkflowRow): string => row.workflow ? `${baseUrl}/${row.workflow.id}` : `${baseUrl}?event=${encodeURIComponent(row.event.event)}`,

    summary(row: WorkflowRow): string {
      if (!row.workflow) return '';
      const {issue_type, source_column_id, target_column_id, label_ids} = row.workflow.filters;
      const sourceTitle = source_column_id && columnTitle(source_column_id);
      const targetTitle = target_column_id && columnTitle(target_column_id);
      const labelNames = store.labels.filter((l) => label_ids?.includes(l.id)).map((l) => l.name);
      const parts = [
        issue_type === 'issue' && props.locale.issuesOnly,
        issue_type === 'pull_request' && props.locale.pullRequestsOnly,
        sourceTitle && trString(props.locale.summarySource, sourceTitle),
        targetTitle && trString(props.locale.summaryTarget, targetTitle),
        labelNames.length && trString(props.locale.summaryLabels, labelNames.join(', ')),
      ];
      return parts.filter(Boolean).map((part) => `(${part})`).join(' ');
    },

    select(row: WorkflowRow, history?: 'push' | 'replace') {
      store.selectedKey = row.key;
      store.editMode = false;
      store.form = row.workflow ? formFor(row.workflow) : row.draft;
      const state = {key: row.key, event: row.event.event};
      if (history === 'push') window.history.pushState(state, '', store.urlFor(row));
      else if (history === 'replace') window.history.replaceState(state, '', store.urlFor(row));
    },

    cancel() {
      const row = store.selected!;
      const clone = store.drafts.find((d) => d.key === row.key && d.sourceKey);
      if (clone) {
        store.drafts = store.drafts.filter((d) => d !== clone);
        store.select((store.resolve(clone.sourceKey!) ?? store.resolve(clone.event))!, 'replace');
        return;
      }
      store.editMode = false;
      store.form = formFor(row.workflow!);
    },

    clone() {
      const source = store.selected!;
      const key = `clone-${++cloneSeq}`;
      store.drafts.push({key, event: source.event.event, sourceKey: source.key, draft: formFor(source.workflow!)});
      store.select(store.resolve(key)!);
    },

    canClone: (row: WorkflowRow): boolean => store.drafts.every((d) => d.sourceKey !== row.key),

    async save() {
      const row = store.selected!;
      const saved = row.workflow ?
        await post<ProjectWorkflow>(`${baseUrl}/${row.workflow.id}`, store.form) :
        await post<ProjectWorkflow>(baseUrl, {event: row.event.event, ...store.form});
      if (!saved) return;
      let history: 'push' | 'replace' | undefined;
      if (row.workflow) {
        replaceWorkflow(saved);
      } else {
        store.workflows.push(saved);
        const draft = store.drafts.find((d) => d.key === row.key)!;
        if (draft.sourceKey) {
          store.drafts = store.drafts.filter((d) => d !== draft);
          history = 'push'; // a clone has no URL of its own, keep its source's entry
        } else {
          draft.draft = toWorkflowForm();
          history = 'replace';
        }
      }
      if (store.selectedKey === row.key) store.select(store.resolve(String(saved.id))!, history);
    },

    async setEnabled(enabled: boolean) {
      const workflow = store.selected!.workflow!;
      const saved = await post<ProjectWorkflow>(`${baseUrl}/${workflow.id}`, {enabled});
      if (saved) replaceWorkflow(saved);
    },

    async remove(workflow: ProjectWorkflow) {
      if (!await post(`${baseUrl}/${workflow.id}/delete`, {})) return;
      store.workflows = store.workflows.filter((wf) => wf.id !== workflow.id);
      if (store.selectedKey === String(workflow.id)) store.select(store.resolve(workflow.event)!, 'replace');
    },
  });

  function columnTitle(id: number): string | undefined {
    return store.columns.find((c) => c.id === id)?.title;
  }

  // drops columns and labels deleted since the workflow was saved, the server would reject them
  function formFor(workflow: ProjectWorkflow): WorkflowForm {
    const form = toWorkflowForm(workflow);
    const hasColumn = (id: number) => store.columns.some((c) => c.id === id);
    const hasLabel = (id: number) => store.labels.some((l) => l.id === id);
    if (!hasColumn(form.filters.source_column_id)) form.filters.source_column_id = 0;
    if (!hasColumn(form.filters.target_column_id)) form.filters.target_column_id = 0;
    if (!hasColumn(form.actions.column_id)) form.actions.column_id = 0;
    form.filters.label_ids = form.filters.label_ids.filter(hasLabel);
    form.actions.add_label_ids = form.actions.add_label_ids.filter(hasLabel);
    form.actions.remove_label_ids = form.actions.remove_label_ids.filter(hasLabel);
    return form;
  }

  async function post<T>(url: string, data: Record<string, unknown>): Promise<T | null> {
    store.saving = true;
    try {
      const resp = await performFetchActionRequest(props.el, {url, method: 'POST', data});
      return resp ? await resp.json() : null;
    } finally {
      store.saving = false;
    }
  }

  function replaceWorkflow(saved: ProjectWorkflow) {
    store.workflows = store.workflows.map((wf) => wf.id === saved.id ? saved : wf);
  }

  return store;
}

export type ProjectWorkflowStore = ReturnType<typeof createProjectWorkflowStore>;
