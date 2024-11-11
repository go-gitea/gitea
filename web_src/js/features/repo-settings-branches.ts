import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';

export function initRepoBranchesSettings() {
  const protectedBranchesList = document.querySelector('#protected-branches-list');
  if (!protectedBranchesList) return;

  createSortable(protectedBranchesList, {
    handle: '.drag-handle',
    animation: 150,
    onEnd: async () => { // eslint-disable-line @typescript-eslint/no-misused-promises
      const newOrder = Array.from(protectedBranchesList.children, (item) => {
        const id = item.getAttribute('data-id');
        return parseInt(id);
      }).filter((id) => !Number.isNaN(id));

      try {
        await POST(protectedBranchesList.getAttribute('data-priority-url'), {
          data: {
            ids: newOrder,
          },
        });
      } catch (err) {
        console.error('Failed to update branch protection rule priority:', err);
      }
    },
  });
}
