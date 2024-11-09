import {fomanticQuery} from '../modules/fomantic/base.ts';
import {POST} from '../modules/fetch.ts';
import {queryElemChildren, toggleElem} from '../utils/dom.ts';

// if there are draft comments, confirm before reloading, to avoid losing comments
export function issueSidebarReloadConfirmDraftComment() {
  const commentTextareas = [
    document.querySelector<HTMLTextAreaElement>('.edit-content-zone:not(.tw-hidden) textarea'),
    document.querySelector<HTMLTextAreaElement>('#comment-form textarea'),
  ];
  for (const textarea of commentTextareas) {
    // Most users won't feel too sad if they lose a comment with 10 chars, they can re-type these in seconds.
    // But if they have typed more (like 50) chars and the comment is lost, they will be very unhappy.
    if (textarea && textarea.value.trim().length > 10) {
      textarea.parentElement.scrollIntoView();
      if (!window.confirm('Page will be reloaded, but there are draft comments. Continuing to reload will discard the comments. Continue?')) {
        return;
      }
      break;
    }
  }
  window.location.reload();
}

function collectCheckedValues(elDropdown: HTMLElement) {
  return Array.from(elDropdown.querySelectorAll('.menu > .item.checked'), (el) => el.getAttribute('data-value'));
}

export function initIssueSidebarComboList(container: HTMLElement) {
  if (!container) return;

  const updateUrl = container.getAttribute('data-update-url');
  const elDropdown = container.querySelector<HTMLElement>(':scope > .ui.dropdown');
  const elList = container.querySelector<HTMLElement>(':scope > .ui.list');
  const elComboValue = container.querySelector<HTMLInputElement>(':scope > .combo-value');
  const initialValues = collectCheckedValues(elDropdown);

  elDropdown.addEventListener('click', (e) => {
    const elItem = (e.target as HTMLElement).closest('.item');
    if (!elItem) return;
    e.preventDefault();
    if (elItem.getAttribute('data-can-change') !== 'true') return;
    elItem.classList.toggle('checked');
    elComboValue.value = collectCheckedValues(elDropdown).join(',');
  });

  const updateToBackend = async (changedValues) => {
    let changed = false;
    for (const value of initialValues) {
      if (!changedValues.includes(value)) {
        await POST(updateUrl, {data: new URLSearchParams({action: 'detach', id: value})});
        changed = true;
      }
    }
    for (const value of changedValues) {
      if (!initialValues.includes(value)) {
        await POST(updateUrl, {data: new URLSearchParams({action: 'attach', id: value})});
        changed = true;
      }
    }
    if (changed) issueSidebarReloadConfirmDraftComment();
  };

  const syncList = (changedValues) => {
    const elEmptyTip = elList.querySelector('.item.empty-list');
    queryElemChildren(elList, '.item:not(.empty-list)', (el) => el.remove());
    for (const value of changedValues) {
      const el = elDropdown.querySelector<HTMLElement>(`.menu > .item[data-value="${value}"]`);
      const listItem = el.cloneNode(true) as HTMLElement;
      listItem.querySelector('svg.octicon-check')?.remove();
      elList.append(listItem);
    }
    const hasItems = Boolean(elList.querySelector('.item:not(.empty-list)'));
    toggleElem(elEmptyTip, !hasItems);
  };

  fomanticQuery(elDropdown).dropdown({
    action: 'nothing', // do not hide the menu if user presses Enter
    fullTextSearch: 'exact',
    async onHide() {
      const changedValues = collectCheckedValues(elDropdown);
      if (updateUrl) {
        await updateToBackend(changedValues); // send requests to backend and reload the page
      } else {
        syncList(changedValues); // only update the list in the sidebar
      }
    },
  });
}
