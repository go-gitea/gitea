import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {createProjectWorkflowStore} from './ProjectWorkflowStore.ts';
import type {ProjectWorkflow, ProjectWorkflowLocale} from './ProjectWorkflowStore.ts';

vi.mock('../modules/fetch-action.ts', () => ({performFetchActionRequest: vi.fn()}));

const locale = {
  issuesOnly: 'Issues only',
  pullRequestsOnly: 'Pull requests only',
  summarySource: 'Source: %s',
  summaryTarget: 'Target: %s',
  summaryLabels: 'Labels: %s',
} as ProjectWorkflowLocale;

const workflow = (id: number, event: string, extra: Partial<ProjectWorkflow> = {}): ProjectWorkflow => ({
  id, event, enabled: true, filters: {}, actions: {column_id: 1}, ...extra,
});

function setup(workflows: ProjectWorkflow[]) {
  window.config.pageData.projectWorkflows = {
    events: [
      {event: 'item_added_to_project', display_name: 'Item added to project', supported_filters: ['issue_type', 'labels'], supported_actions: ['column']},
      {event: 'item_closed', display_name: 'Item closed', supported_filters: [], supported_actions: ['column']},
    ],
    workflows,
    columns: [{id: 1, title: 'Backlog'}, {id: 2, title: 'Done'}],
    labels: [{id: 7, name: 'bug', html: ''}, {id: 8, name: 'docs', html: ''}],
  };
  return createProjectWorkflowStore({el: document.createElement('div'), projectLink: '/o/r/projects/1', canWrite: true, locale});
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
  vi.spyOn(window.history, 'replaceState').mockImplementation(() => {});
});

test('rows, display names, summaries and deep link resolution', () => {
  const store = setup([
    workflow(3, 'item_added_to_project', {filters: {issue_type: 'issue', source_column_id: 2, target_column_id: 1, label_ids: [8, 7]}}),
    workflow(5, 'item_added_to_project'),
  ]);
  expect(store.rows.map((r) => [r.key, r.displayName])).toEqual([
    ['3', 'Item added to project #1'],
    ['5', 'Item added to project #2'],
    ['item_closed', 'Item closed'],
  ]);
  expect(store.resolve('5')!.key).toBe('5');
  expect(store.resolve('item_added_to_project')!.key).toBe('3');
  expect(store.resolve('item_closed')!.workflow).toBeUndefined();
  expect(store.resolve('42')).toBeUndefined();
  expect(store.urlFor(store.rows[2])).toBe('/o/r/projects/1/workflows?event=item_closed');
  expect(store.summary(store.rows[0])).toBe('(Issues only) (Source: Done) (Target: Backlog) (Labels: bug, docs)');
  expect(store.summary(store.rows[1])).toBe('');
  expect(store.summary(store.rows[2])).toBe('');
});

test('edits to a saved workflow are dropped on switching, unsaved drafts are kept', () => {
  const store = setup([workflow(3, 'item_added_to_project')]);
  store.select(store.resolve('3')!);
  store.editMode = true;
  store.form.actions.column_id = 2;
  store.cancel();
  expect(store.editing).toBe(false);
  expect(store.form.actions.column_id).toBe(1);
  store.editMode = true;
  store.form.actions.column_id = 2;
  store.select(store.resolve('item_closed')!);
  expect(store.editing).toBe(true);
  store.form.actions.column_id = 2;
  store.select(store.resolve('3')!);
  expect(store.editing).toBe(false);
  expect(store.form.actions.column_id).toBe(1);
  store.select(store.resolve('item_closed')!);
  expect(store.form.actions.column_id).toBe(2);
});

test('the form drops columns and labels deleted since saving', () => {
  const store = setup([workflow(3, 'item_added_to_project', {filters: {label_ids: [7, 9]}, actions: {column_id: 4, add_label_ids: [9]}})]);
  store.select(store.resolve('3')!);
  expect(store.form.filters.label_ids).toEqual([7]);
  expect(store.form.actions.column_id).toBe(0);
  expect(store.form.actions.add_label_ids).toEqual([]);
});

test('clone is numbered after its source, cancel returns to the source, saving adds a history entry', async () => {
  const store = setup([workflow(3, 'item_added_to_project'), workflow(5, 'item_added_to_project', {actions: {column_id: 2}})]);
  store.select(store.resolve('5')!);
  store.clone();
  expect(store.selected!.displayName).toBe('Item added to project #3');
  expect(store.editing).toBe(true);
  expect(store.form.actions.column_id).toBe(2);
  expect(store.canClone(store.resolve('5')!)).toBe(false);
  store.cancel();
  expect(store.selectedKey).toBe('5');
  expect(store.rows).toHaveLength(3);
  expect(window.history.replaceState).toHaveBeenCalledWith({key: '5', event: 'item_added_to_project'}, '', '/o/r/projects/1/workflows/5');

  store.clone();
  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(Response.json(workflow(9, 'item_added_to_project', {actions: {column_id: 2}})));
  await store.save();
  expect(store.selected!.displayName).toBe('Item added to project #3');
  expect(window.history.pushState).toHaveBeenCalledWith({key: '9', event: 'item_added_to_project'}, '', '/o/r/projects/1/workflows/9');
});

test('saving a placeholder retains the draft on failure and creates the workflow on success', async () => {
  const store = setup([]);
  store.select(store.resolve('item_closed')!);
  store.form.actions.column_id = 2;
  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(null);
  await store.save();
  expect(store.selectedKey).toBe('item_closed');
  expect(store.editing).toBe(true);
  expect(store.form.actions.column_id).toBe(2);

  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(Response.json(workflow(9, 'item_closed', {actions: {column_id: 2}})));
  await store.save();

  expect(performFetchActionRequest).toHaveBeenCalledWith(expect.any(HTMLElement), {url: '/o/r/projects/1/workflows', method: 'POST', data: {
    event: 'item_closed',
    filters: {issue_type: '', source_column_id: 0, target_column_id: 0, label_ids: []},
    actions: {column_id: 2, add_label_ids: [], remove_label_ids: [], issue_state: ''},
  }});
  expect(store.selectedKey).toBe('9');
  expect(window.history.replaceState).toHaveBeenCalledWith({key: '9', event: 'item_closed'}, '', '/o/r/projects/1/workflows/9');
  expect(store.drafts.find((d) => d.key === 'item_closed')!.draft.actions.column_id).toBe(0);
});

test.each(['save', 'setEnabled'] as const)('%s preserves selection changed through history during the request', async (operation) => {
  const store = setup([workflow(3, 'item_added_to_project'), workflow(5, 'item_added_to_project', {actions: {column_id: 2}})]);
  store.select(store.resolve('3')!);
  const response = Promise.withResolvers<Response>();
  vi.mocked(performFetchActionRequest).mockReturnValueOnce(response.promise);
  const request = operation === 'save' ? store.save() : store.setEnabled(false);
  store.select(store.resolve('5')!);
  response.resolve(Response.json(workflow(3, 'item_added_to_project', {enabled: false})));
  await request;
  expect(store.workflows[0].enabled).toBe(false);
  expect(store.selectedKey).toBe('5');
  expect(store.form.actions.column_id).toBe(2);
  expect(window.history.pushState).not.toHaveBeenCalled();
  expect(window.history.replaceState).not.toHaveBeenCalled();
});

test('delete keeps its target after selection changes and selects the next row only when still selected', async () => {
  const store = setup([workflow(3, 'item_added_to_project'), workflow(5, 'item_added_to_project')]);
  store.select(store.resolve('3')!);
  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(null);
  await store.remove(store.selected!.workflow!);
  expect(store.rows).toHaveLength(3);
  expect(store.selectedKey).toBe('3');

  const workflowToDelete = store.selected!.workflow!;
  store.select(store.resolve('5')!);
  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(Response.json({}));
  await store.remove(workflowToDelete);
  expect(performFetchActionRequest).toHaveBeenLastCalledWith(expect.any(HTMLElement), {url: '/o/r/projects/1/workflows/3/delete', method: 'POST', data: {}});
  expect(store.rows.map((r) => r.key)).toEqual(['5', 'item_closed']);
  expect(store.selectedKey).toBe('5');
  expect(window.history.replaceState).not.toHaveBeenCalled();

  vi.mocked(performFetchActionRequest).mockResolvedValueOnce(Response.json({}));
  await store.remove(store.selected!.workflow!);
  expect(store.selectedKey).toBe('item_added_to_project');
  expect(window.history.replaceState).toHaveBeenCalledWith({key: 'item_added_to_project', event: 'item_added_to_project'}, '', '/o/r/projects/1/workflows?event=item_added_to_project');
});
