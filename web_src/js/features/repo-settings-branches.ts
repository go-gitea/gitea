import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';

export function initRepoBranchesSettings() {
  const protectedBranchesList = document.getElementById('protected-branches-list');
  if (!protectedBranchesList) return;

  createSortable(protectedBranchesList, {
    handle: '.drag-handle',
    animation: 150,
    onEnd: async (evt) => {
      const newOrder = Array.from(protectedBranchesList.children).map((item) => {
        const id = item.dataset.id;
        return id ? parseInt(id, 10) : NaN;
      }).filter(id => !isNaN(id));

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
