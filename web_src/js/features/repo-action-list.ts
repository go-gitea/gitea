import {queryElems, toggleElem} from '../utils/dom.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import {showErrorToast} from '../modules/toast.ts';
import {POST} from '../modules/fetch.ts';

function initRepoActionListCheckboxes() {
  const actionListSelectAll = document.querySelector<HTMLInputElement>('.action-checkbox-all');
  if (!actionListSelectAll) return; // logged out state
  const issueCheckboxes = document.querySelectorAll<HTMLInputElement>('.action-checkbox:not([disabled])');
  const actionDelete = document.querySelector('#action-delete');
  const syncIssueSelectionState = () => {
    const enabledCheckboxes = Array.from(issueCheckboxes).filter((el) => !el.disabled);
    const checkedCheckboxes = enabledCheckboxes.filter((el) => el.checked);
    const anyChecked = Boolean(checkedCheckboxes.length);
    const allChecked = anyChecked && checkedCheckboxes.length === enabledCheckboxes.length;

    if (allChecked) {
      actionListSelectAll.checked = true;
      actionListSelectAll.indeterminate = false;
    } else if (anyChecked) {
      actionListSelectAll.checked = false;
      actionListSelectAll.indeterminate = true;
    } else {
      actionListSelectAll.checked = false;
      actionListSelectAll.indeterminate = false;
    }
    if (actionDelete) {
      toggleElem('#action-delete', anyChecked);
    }
  };

  for (const el of issueCheckboxes) {
    el.addEventListener('change', syncIssueSelectionState);
  }

  actionListSelectAll.addEventListener('change', () => {
    for (const el of issueCheckboxes) {
      if (!el.disabled) {
        el.checked = actionListSelectAll.checked;
      }
    }
    syncIssueSelectionState();
  });

  queryElems(document, '.action-action', (el) => el.addEventListener('click',
    async (e: MouseEvent) => {
      e.preventDefault();

      const action = el.getAttribute('data-action');
      const url = el.getAttribute('data-url');
      const actionIDList: number[] = [];
      const radix = 10;
      for (const el of document.querySelectorAll<HTMLInputElement>('.action-checkbox:checked:not([disabled])')) {
        const id = el.getAttribute('data-action-id');
        if (id) {
          actionIDList.push(parseInt(id, radix));
        }
      }
      if (actionIDList.length < 1) return;

      // for delete
      if (action === 'delete') {
        const confirmText = el.getAttribute('data-action-delete-confirm');
        if (!await confirmModal({content: confirmText, confirmButtonColor: 'red'})) {
          return;
        }
      }

      try {
        await deleteActions(url, actionIDList);
        window.location.reload();
      } catch (err) {
        showErrorToast(err.responseJSON?.error ?? err.message);
      }
    },
  ));
}

async function deleteActions(url: string, actionIds: number[]) {
  try {
    const response = await POST(url, {
      data: {
        actionIds,
      },
    });
    if (!response.ok) {
      throw new Error('failed to delete actions');
    }
  } catch (error) {
    console.error(error);
  }
}
export function initRepoActionList() {
  if (document.querySelector('.page-content.repository.actions')) {
    initRepoActionListCheckboxes();
  }
}
