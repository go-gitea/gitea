import {beforeEach, describe, expect, test, vi} from 'vitest';
import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {POST} from '../modules/fetch.ts';
import {createSortable} from '../modules/sortable.ts';

vi.mock('../modules/fetch.ts', () => ({
  POST: vi.fn(),
}));

vi.mock('../modules/sortable.ts', () => ({
  createSortable: vi.fn(),
}));

describe('Repository Branch Settings', () => {
  beforeEach(() => {
    document.body.innerHTML = `
      <div id="protected-branches-list" data-update-priority-url="some/repo/branches/priority">
        <div class="flex-item tw-items-center item" data-id="1" >
          <div class="drag-handle"></div>
        </div>
        <div class="flex-item tw-items-center item" data-id="2" >
          <div class="drag-handle"></div>
        </div>
        <div class="flex-item tw-items-center item" data-id="3" >
          <div class="drag-handle"></div>
        </div>
      </div>
    `;

    vi.clearAllMocks();
  });

  test('should initialize sortable for protected branches list', () => {
    initRepoSettingsBranchesDrag();

    expect(createSortable).toHaveBeenCalledWith(
      document.querySelector('#protected-branches-list'),
      expect.objectContaining({
        handle: '.drag-handle',
        animation: 150,
      }),
    );
  });

  test('should not initialize if protected branches list is not present', () => {
    document.body.innerHTML = '';

    initRepoSettingsBranchesDrag();

    expect(createSortable).not.toHaveBeenCalled();
  });

  test('should post new order after sorting', async () => {
    vi.mocked(POST).mockResolvedValue({ok: true} as Response);

    // Mock createSortable to capture and execute the onEnd callback
    vi.mocked(createSortable).mockImplementation((_el, options) => {
      options.onEnd();
      return {destroy: vi.fn()};
    });

    initRepoSettingsBranchesDrag();

    expect(POST).toHaveBeenCalledWith(
      'some/repo/branches/priority',
      expect.objectContaining({
        data: {ids: [1, 2, 3]},
      }),
    );
  });
});
