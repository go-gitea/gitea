import {createSortable} from '../modules/sortable.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {queryElemChildren} from '../utils/dom.ts';

export function initRepoSettingsBranchesDrag() {
  const protectedBranchesList = document.querySelector('#protected-branches-list');
  if (!protectedBranchesList) return;

  createSortable(protectedBranchesList, {
    handle: '.drag-handle',
    animation: 150,

    onEnd: () => {
      (async () => {
        const itemElems = queryElemChildren(protectedBranchesList, '.item[data-id]');
        const itemIds = Array.from(itemElems, (el) => parseInt(el.getAttribute('data-id')));

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
