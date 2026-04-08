import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {POST} from '../modules/fetch.ts';
import {createSortable} from '../modules/sortable.ts';
import type {SortableEvent} from 'sortablejs';

vi.mock('../modules/fetch.ts', () => ({
  POST: vi.fn(),
}));

vi.mock('../modules/sortable.ts', () => ({
  createSortable: vi.fn(),
}));

const branchesHTML = `
  <div id="protected-branches-list" data-update-priority-url="some/repo/branches/priority">
    <div class="flex-item tw-items-center item" data-id="1">
      <div class="drag-handle"></div>
    </div>
    <div class="flex-item tw-items-center item" data-id="2">
      <div class="drag-handle"></div>
    </div>
    <div class="flex-item tw-items-center item" data-id="3">
      <div class="drag-handle"></div>
    </div>
  </div>
`;

describe('Repository Branch Settings', () => {
  test('should initialize sortable for protected branches list', () => {
    document.body.innerHTML = branchesHTML;
    const callsBefore = vi.mocked(createSortable).mock.calls.length;
    initRepoSettingsBranchesDrag();
    const newCalls = vi.mocked(createSortable).mock.calls.slice(callsBefore);
    expect(newCalls).toHaveLength(1);
    expect(newCalls[0][0]).toBe(document.querySelector('#protected-branches-list'));
    expect(newCalls[0][1]).toMatchObject({handle: '.drag-handle', animation: 150});
  });

  test('should not initialize if protected branches list is not present', () => {
    document.querySelector('#protected-branches-list')?.remove();
    const callsBefore = vi.mocked(createSortable).mock.calls.length;
    initRepoSettingsBranchesDrag();
    expect(vi.mocked(createSortable).mock.calls.length).toBe(callsBefore);
  });

  test('should post new order after sorting', () => {
    document.body.innerHTML = branchesHTML;
    vi.mocked(POST).mockResolvedValue({ok: true} as Response);
    const callsBefore = vi.mocked(createSortable).mock.calls.length;
    initRepoSettingsBranchesDrag();
    const onEnd = vi.mocked(createSortable).mock.calls[callsBefore][1]!.onEnd!;
    onEnd(new Event('SortableEvent') as SortableEvent);
    expect(POST).toHaveBeenCalledWith(
      'some/repo/branches/priority',
      expect.objectContaining({data: {ids: [1, 2, 3]}}),
    );
  });
});
