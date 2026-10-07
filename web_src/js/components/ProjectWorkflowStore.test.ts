import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {createProjectWorkflowStore, toWorkflowForm, toWorkflowRules} from './ProjectWorkflowStore.ts';
import type {ProjectWorkflow, ProjectWorkflowLocale} from './ProjectWorkflowStore.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn(), POST: vi.fn()}));
vi.mock('../modules/toast.ts', () => ({showErrorToast: vi.fn()}));

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

const jsonResponse = (body: unknown, status = 200) => Response.json(body, {status});

async function setup(workflows: ProjectWorkflow[]) {
  vi.mocked(GET).mockResolvedValueOnce(jsonResponse({
    events: [
      {event: 'item_opened', display_name: 'Item opened', filters: ['issue_type', 'labels'], actions: ['column']},
      {event: 'item_closed', display_name: 'Item closed', filters: [], actions: ['column']},
    ],
    workflows,
    columns: [{id: 1, title: 'Backlog'}, {id: 2, title: 'Done'}],
    labels: [{id: 7, name: 'bug', color: 'ee0701'}, {id: 8, name: 'docs', color: '0075ca'}],
  }));
  const store = createProjectWorkflowStore({projectLink: '/o/r/projects/1', canWrite: true, locale});
  await store.load();
  return store;
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.spyOn(window.history, 'pushState').mockImplementation(() => {});
  vi.spyOn(window.history, 'replaceState').mockImplementation(() => {});
});

test('rows, display names and deep link resolution', async () => {
  const store = await setup([workflow(3, 'item_opened'), workflow(5, 'item_opened')]);
  expect(store.rows.map((r) => [r.key, r.displayName])).toEqual([
    ['3', 'Item opened #1'],
    ['5', 'Item opened #2'],
    ['item_closed', 'Item closed'],
  ]);
  expect(store.resolve('5')!.key).toBe('5');
  expect(store.resolve('item_opened')!.key).toBe('3');
  expect(store.resolve('item_closed')!.workflow).toBeUndefined();
  expect(store.resolve('42')).toBeUndefined();
  expect(store.urlFor(store.rows[2])).toBe('/o/r/projects/1/workflows/item_closed');
});

test('summary', async () => {
  const store = await setup([workflow(3, 'item_opened', {filters: {issue_type: 'issue', source_column_id: 2, label_ids: [8, 7]}})]);
  expect(store.summary(store.rows[0])).toBe('(Issues only) (Source: Done) (Labels: bug, docs)');
  expect(store.summary(store.rows[1])).toBe('');
});

test('edits to a saved workflow are dropped on switching, unsaved drafts are kept', async () => {
  const store = await setup([workflow(3, 'item_opened')]);
  store.select(store.resolve('3')!);
  store.editMode = true;
  store.form.column_id = 2;
  store.select(store.resolve('item_closed')!);
  expect(store.editing).toBe(true);
  store.form.column_id = 2;
  store.select(store.resolve('3')!);
  expect(store.editing).toBe(false);
  expect(store.form.column_id).toBe(1);
  store.select(store.resolve('item_closed')!);
  expect(store.form.column_id).toBe(2);
});

test('the form drops columns and labels deleted since saving', async () => {
  const store = await setup([workflow(3, 'item_opened', {filters: {label_ids: [7, 9]}, actions: {column_id: 4, add_label_ids: [9]}})]);
  store.select(store.resolve('3')!);
  expect(store.form.label_ids).toEqual([7]);
  expect(store.form.column_id).toBe(0);
  expect(store.form.add_label_ids).toEqual([]);
});

test('clone is numbered after its source and cancel returns to the source', async () => {
  const store = await setup([workflow(3, 'item_opened'), workflow(5, 'item_opened', {actions: {column_id: 2}})]);
  store.select(store.resolve('5')!);
  store.clone();
  expect(store.selected!.displayName).toBe('Item opened #3');
  expect(store.editing).toBe(true);
  expect(store.form.column_id).toBe(2);
  expect(store.canClone(store.resolve('5')!)).toBe(false);
  store.cancel();
  expect(store.selectedKey).toBe('5');
  expect(store.rows).toHaveLength(3);
});

test('saving a placeholder creates the workflow and blocks selection meanwhile', async () => {
  const store = await setup([]);
  store.select(store.resolve('item_closed')!);
  store.form.column_id = 2;
  let resolvePost!: (resp: Response) => void;
  vi.mocked(POST).mockReturnValueOnce(new Promise((resolve) => resolvePost = resolve));
  const saving = store.save();
  store.select(store.resolve('item_opened')!);
  expect(store.selectedKey).toBe('item_closed');
  resolvePost(jsonResponse(workflow(9, 'item_closed', {actions: {column_id: 2}})));
  await saving;

  expect(POST).toHaveBeenCalledWith('/o/r/projects/1/workflows', {data: {
    event: 'item_closed',
    filters: {label_ids: []},
    actions: {column_id: 2, add_label_ids: [], remove_label_ids: []},
  }});
  expect(store.selectedKey).toBe('9');
  expect(window.history.replaceState).toHaveBeenCalledWith({key: '9', event: 'item_closed'}, '', '/o/r/projects/1/workflows/9');
  expect(store.unsaved.find((u) => u.key === 'item_closed')!.draft.column_id).toBe(0);
});

test('a failed save shows the server error and keeps the draft', async () => {
  const store = await setup([]);
  store.select(store.resolve('item_opened')!);
  vi.mocked(POST).mockResolvedValueOnce(jsonResponse({errorMessage: 'At least one action must be configured'}, 400));
  await store.save();
  expect(showErrorToast).toHaveBeenCalledWith('At least one action must be configured');
  expect(store.selectedKey).toBe('item_opened');
  expect(store.editing).toBe(true);
});

test('delete selects the next row of the same event only on success', async () => {
  const store = await setup([workflow(3, 'item_opened'), workflow(5, 'item_opened')]);
  store.select(store.resolve('3')!);
  vi.mocked(POST).mockResolvedValueOnce(new Response('oops', {status: 500}));
  await store.remove();
  expect(store.rows).toHaveLength(3);
  expect(store.selectedKey).toBe('3');

  vi.mocked(POST).mockResolvedValueOnce(jsonResponse({}));
  await store.remove();
  expect(store.rows.map((r) => r.key)).toEqual(['5', 'item_closed']);
  expect(store.selectedKey).toBe('5');
});

test('form conversion omits unset values', () => {
  const form = toWorkflowForm({filters: {issue_type: 'pull_request', target_column_id: 2, label_ids: [7]}, actions: {issue_state: 'close'}});
  expect(toWorkflowRules(form)).toEqual({
    filters: {issue_type: 'pull_request', target_column_id: 2, label_ids: [7]},
    actions: {add_label_ids: [], remove_label_ids: [], issue_state: 'close'},
  });
});
