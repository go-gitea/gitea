import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

export function initRepoBranchesSettings() {
  const protectedBranchesList = document.querySelector('#protected-branches-list');
  if (!protectedBranchesList) return;

  createSortable(protectedBranchesList, {
    handle: '.drag-handle',
    animation: 150,

    onEnd: () => {
      (async () => {
        const itemIds = Array.from(protectedBranchesList.children, (item) => {
          const id = item.getAttribute('data-id');
          return parseInt(id);
        });

        try {
          await POST(protectedBranchesList.getAttribute('data-update-priority-url'), {
            data: {
              ids: itemIds,
            },
          });
        } catch (err) {
          const errorMessage = String(err);
          showErrorToast(`Failed to update branch protection rule priority:, error: ${errorMessage}`);
        }
      })();
    },
  });
}
