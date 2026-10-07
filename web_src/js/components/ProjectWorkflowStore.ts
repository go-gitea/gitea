import {computed, reactive} from 'vue';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {trString} from '../modules/i18n.ts';
import type {Label} from '../types.ts';

type ProjectWorkflowFilters = {
  issue_type?: 'issue' | 'pull_request';
  source_column_id?: number;
  target_column_id?: number;
  label_ids?: number[];
};

type ProjectWorkflowActions = {
  column_id?: number;
  add_label_ids?: number[];
  remove_label_ids?: number[];
  issue_state?: 'close' | 'reopen';
};

export type ProjectWorkflow = {
  id: number;
  event: string;
  enabled: boolean;
  filters: ProjectWorkflowFilters;
  actions: ProjectWorkflowActions;
};

type ProjectWorkflowEvent = {
  event: string;
  display_name: string;
  filters: string[];
  actions: string[];
};

type ProjectColumn = {id: number; title: string};

type ProjectWorkflowData = {
  events: ProjectWorkflowEvent[];
  workflows: ProjectWorkflow[];
  columns: ProjectColumn[];
  labels: Label[];
};

// flat, always-populated editor state: 0 means "any" / "none", '' means unset
type WorkflowForm = {
  issue_type: '' | 'issue' | 'pull_request';
  source_column_id: number;
  target_column_id: number;
  label_ids: number[];
  column_id: number;
  add_label_ids: number[];
  remove_label_ids: number[];
  issue_state: '' | 'close' | 'reopen';
};

export type WorkflowRow = {
  key: string;
  event: ProjectWorkflowEvent;
  displayName: string;
} & ({workflow: ProjectWorkflow, draft?: never} | {workflow?: never, draft: WorkflowForm});

// placeholders use the event name as key, clones remember the row they were cloned from
type UnsavedWorkflow = {key: string; event: string; draft: WorkflowForm; sourceKey?: string};

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
  actions: string;
  moveToColumn: string;
  selectColumn: string;
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

export function toWorkflowForm({filters, actions}: Pick<ProjectWorkflow, 'filters' | 'actions'> = {filters: {}, actions: {}}): WorkflowForm {
  return {
    issue_type: filters.issue_type ?? '',
    source_column_id: filters.source_column_id ?? 0,
    target_column_id: filters.target_column_id ?? 0,
    label_ids: [...filters.label_ids ?? []],
    column_id: actions.column_id ?? 0,
    add_label_ids: [...actions.add_label_ids ?? []],
    remove_label_ids: [...actions.remove_label_ids ?? []],
    issue_state: actions.issue_state ?? '',
  };
}

export function toWorkflowRules(form: WorkflowForm): Pick<ProjectWorkflow, 'filters' | 'actions'> {
  return {
    filters: {
      issue_type: form.issue_type || undefined,
      source_column_id: form.source_column_id || undefined,
      target_column_id: form.target_column_id || undefined,
      label_ids: form.label_ids,
    },
    actions: {
      column_id: form.column_id || undefined,
      add_label_ids: form.add_label_ids,
      remove_label_ids: form.remove_label_ids,
      issue_state: form.issue_state || undefined,
    },
  };
}

// every event lists its saved workflows then its clones, or its placeholder when it has neither
function buildWorkflowRows(events: ProjectWorkflowEvent[], workflows: ProjectWorkflow[], unsaved: UnsavedWorkflow[]): WorkflowRow[] {
  return events.flatMap((event) => {
    const drafts = unsaved.filter((u) => u.event === event.event);
    const group: Array<Pick<WorkflowRow, 'key' | 'workflow' | 'draft'>> = [
      ...workflows.filter((wf) => wf.event === event.event).map((workflow) => ({key: String(workflow.id), workflow})),
      ...drafts.filter((u) => u.sourceKey).map(({key, draft}) => ({key, draft})),
    ];
    if (!group.length) group.push(...drafts.map(({key, draft}) => ({key, draft})));
    return group.map((row, idx) => ({
      ...row,
      event,
      displayName: group.length > 1 ? `${event.display_name} #${idx + 1}` : event.display_name,
    } as WorkflowRow));
  });
}

let cloneSeq = 0;

export function createProjectWorkflowStore(props: {projectLink: string, canWrite: boolean, locale: ProjectWorkflowLocale}) {
  const baseUrl = `${props.projectLink}/workflows`;

  const store = reactive({
    canWrite: props.canWrite,
    events: [] as ProjectWorkflowEvent[],
    workflows: [] as ProjectWorkflow[],
    columns: [] as ProjectColumn[],
    labels: [] as Label[],
    unsaved: [] as UnsavedWorkflow[],

    selectedKey: '',
    editMode: false,
    saving: false,
    form: toWorkflowForm(),

    rows: computed((): WorkflowRow[] => buildWorkflowRows(store.events, store.workflows, store.unsaved)),

    get selected(): WorkflowRow | undefined {
      return store.rows.find((row) => row.key === store.selectedKey);
    },

    get editing(): boolean {
      const row = store.selected;
      if (!store.canWrite || !row) return false;
      return store.editMode || !row.workflow;
    },

    // saved workflow id, event name (its first row) or clone key
    resolve: (key: string): WorkflowRow | undefined => store.rows.find((row) => row.key === key) ?? store.rows.find((row) => row.event.event === key),

    urlFor: (row: WorkflowRow): string => `${baseUrl}/${row.workflow ? row.workflow.id : row.event.event}`,

    columnTitle: (id: number): string | undefined => store.columns.find((c) => c.id === id)?.title,

    summary(row: WorkflowRow): string {
      if (!row.workflow) return '';
      const {issue_type, source_column_id, target_column_id, label_ids} = row.workflow.filters;
      const sourceTitle = source_column_id && store.columnTitle(source_column_id);
      const targetTitle = target_column_id && store.columnTitle(target_column_id);
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

    async load() {
      const resp = await GET(`${baseUrl}/data`);
      if (!resp.ok) {
        showErrorToast(window.config.i18n.error_occurred);
        return;
      }
      const data: ProjectWorkflowData = await resp.json();
      store.events = data.events;
      store.workflows = data.workflows;
      store.columns = data.columns;
      store.labels = data.labels;
      store.unsaved = data.events.map((e) => ({key: e.event, event: e.event, draft: toWorkflowForm()}));
    },

    select(row: WorkflowRow, history?: 'push' | 'replace') {
      if (store.saving) return;
      store.selectedKey = row.key;
      store.editMode = false;
      // unsaved rows are edited in place so their changes survive switching rows
      store.form = row.workflow ? formFor(row.workflow) : row.draft;
      const state = {key: row.key, event: row.event.event};
      if (history === 'push') window.history.pushState(state, '', store.urlFor(row));
      else if (history === 'replace') window.history.replaceState(state, '', store.urlFor(row));
    },

    cancel() {
      const row = store.selected!;
      const clone = store.unsaved.find((u) => u.key === row.key && u.sourceKey);
      if (!clone) {
        store.select(row);
        return;
      }
      store.unsaved = store.unsaved.filter((u) => u !== clone);
      store.select((store.resolve(clone.sourceKey!) ?? store.resolve(clone.event))!);
    },

    clone() {
      const source = store.selected!;
      const key = `clone-${++cloneSeq}`;
      store.unsaved.push({key, event: source.event.event, sourceKey: source.key, draft: formFor(source.workflow!)});
      store.select(store.resolve(key)!);
    },

    canClone: (row: WorkflowRow): boolean => store.unsaved.every((u) => u.sourceKey !== row.key),

    async save() {
      const row = store.selected!;
      const rules = toWorkflowRules(store.form);
      const saved = row.workflow ?
        await post<ProjectWorkflow>(`${baseUrl}/${row.workflow.id}`, rules) :
        await post<ProjectWorkflow>(baseUrl, {event: row.event.event, ...rules});
      if (!saved) return;
      if (row.workflow) {
        replaceWorkflow(saved);
      } else {
        store.workflows.push(saved);
        const unsaved = store.unsaved.find((u) => u.key === row.key)!;
        if (unsaved.sourceKey) store.unsaved = store.unsaved.filter((u) => u !== unsaved);
        else unsaved.draft = toWorkflowForm();
      }
      store.select(store.resolve(String(saved.id))!, row.workflow ? undefined : 'replace');
    },

    async setEnabled(enabled: boolean) {
      const workflow = store.selected!.workflow!;
      const saved = await post<ProjectWorkflow>(`${baseUrl}/${workflow.id}`, {enabled});
      if (saved) replaceWorkflow(saved);
    },

    async remove() {
      const row = store.selected!;
      const workflow = row.workflow!;
      if (!await post(`${baseUrl}/${workflow.id}/delete`, {})) return;
      store.workflows = store.workflows.filter((wf) => wf.id !== workflow.id);
      store.select(store.resolve(row.event.event)!, 'replace');
    },
  });

  // drops columns and labels deleted since the workflow was saved, the server would reject them
  function formFor(workflow: ProjectWorkflow): WorkflowForm {
    const form = toWorkflowForm(workflow);
    const hasColumn = (id: number) => store.columns.some((c) => c.id === id);
    const hasLabel = (id: number) => store.labels.some((l) => l.id === id);
    for (const key of ['source_column_id', 'target_column_id', 'column_id'] as const) {
      if (!hasColumn(form[key])) form[key] = 0;
    }
    for (const key of ['label_ids', 'add_label_ids', 'remove_label_ids'] as const) {
      form[key] = form[key].filter(hasLabel);
    }
    return form;
  }

  async function post<T>(url: string, data: Record<string, unknown>): Promise<T | null> {
    store.saving = true;
    try {
      const resp = await POST(url, {data});
      if (resp.ok) return await resp.json();
      let message = window.config.i18n.error_occurred;
      try {
        message = (await resp.json()).errorMessage || message;
      } catch {}
      showErrorToast(message);
      return null;
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
