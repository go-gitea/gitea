import {POST} from '../modules/fetch.ts';
import {queryElems, toggleElem} from '../utils/dom.ts';
import {initIssueSidebarComboList} from './repo-issue-sidebar-combolist.ts';

function initBranchSelector() {
  // TODO: RemoveIssueRef: see "repo/issue/branch_selector_field.tmpl"
  const elSelectBranch = document.querySelector('.ui.dropdown.select-branch.branch-selector-dropdown');
  if (!elSelectBranch) return;

  const urlUpdateIssueRef = elSelectBranch.getAttribute('data-url-update-issueref');
  const elBranchMenu = elSelectBranch.querySelector('.reference-list-menu');
  queryElems(elBranchMenu, '.item:not(.no-select)', (el) => el.addEventListener('click', async function (e) {
    e.preventDefault();
    const selectedValue = this.getAttribute('data-id'); // eg: "refs/heads/my-branch"
    const selectedText = this.getAttribute('data-name'); // eg: "my-branch"
    if (urlUpdateIssueRef) {
      // for existing issue, send request to update issue ref, and reload page
      try {
        await POST(urlUpdateIssueRef, {data: new URLSearchParams({ref: selectedValue})});
        window.location.reload();
      } catch (error) {
        console.error(error);
      }
    } else {
      // for new issue, only update UI&form, do not send request/reload
      const selectedHiddenSelector = this.getAttribute('data-id-selector');
      document.querySelector<HTMLInputElement>(selectedHiddenSelector).value = selectedValue;
      elSelectBranch.querySelector('.text-branch-name').textContent = selectedText;
    }
  }));
}

function initRepoIssueDue() {
  const form = document.querySelector<HTMLFormElement>('.issue-due-form');
  if (!form) return;
  const deadline = form.querySelector<HTMLInputElement>('input[name=deadline]');
  document.querySelector('.issue-due-edit')?.addEventListener('click', () => {
    toggleElem(form);
  });
  document.querySelector('.issue-due-remove')?.addEventListener('click', () => {
    deadline.value = '';
    form.dispatchEvent(new Event('submit', {cancelable: true, bubbles: true}));
  });
}

export function initRepoIssueSidebar() {
  initBranchSelector();
  initRepoIssueDue();

  // init the combo list: a dropdown for selecting items, and a list for showing selected items and related actions
  queryElems<HTMLElement>(document, '.issue-sidebar-combo', (el) => initIssueSidebarComboList(el));
}
