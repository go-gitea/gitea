import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {POST} from '../modules/fetch.ts';
import {createSortable} from '../modules/sortable.ts';
import type {SortableEvent} from 'sortablejs';

vi.mock('../modules/fetch.ts', () => ({POST: vi.fn()}));
vi.mock('../modules/sortable.ts', () => ({createSortable: vi.fn()}));

const branchesHTML = `
  <div id="protected-branches-list" data-update-priority-url="some/repo/branches/priority">
    <div class="item" data-id="1">
      <div class="drag-handle"></div>
    </div>
    <div class="item" data-id="2">
      <div class="drag-handle"></div>
    </div>
    <div class="item" data-id="3">
      <div class="drag-handle"></div>
    </div>
  </div>
`;

describe('Repository Branch Settings', () => {
  beforeEach(() => {
    vi.mocked(createSortable).mockClear();
    vi.mocked(POST).mockClear();
  });

  test('should initialize sortable for protected branches list', () => {
    document.body.innerHTML = branchesHTML;
    initRepoSettingsBranchesDrag();
    expect(createSortable).toHaveBeenCalledTimes(1);
    expect(createSortable).toHaveBeenCalledWith(document.querySelector('#protected-branches-list'), expect.objectContaining({handle: '.drag-handle', animation: 150}));
  });

  test('should not initialize if protected branches list is not present', () => {
    document.body.replaceChildren();
    initRepoSettingsBranchesDrag();
    expect(createSortable).toHaveBeenCalledTimes(0);
  });

  test('should post new order after sorting', () => {
    document.body.innerHTML = branchesHTML;
    vi.mocked(POST).mockResolvedValue({ok: true} as Response);
    initRepoSettingsBranchesDrag();
    const onEnd = vi.mocked(createSortable).mock.calls[0][1]!.onEnd!;
    onEnd(new Event('SortableEvent') as SortableEvent);
    expect(POST).toHaveBeenCalledWith(
      'some/repo/branches/priority',
      expect.objectContaining({data: {ids: [1, 2, 3]}}),
    );
  });
});
