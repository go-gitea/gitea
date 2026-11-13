import {GET, POST} from '../modules/fetch.ts';
import {createElementFromHTML, queryElems, toggleElem} from '../utils/dom.ts';
import {IssueSidebarComboList} from './repo-issue-sidebar-combolist.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {html} from '../utils/html.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {showTemporaryTooltip} from '../modules/tippy.ts';
import type {Issue} from '../types.ts';

const {appSubUrl} = window.config;

function initRepoIssueBranchSelector(elSidebar: HTMLElement) {
  const elSelectBranch = elSidebar.querySelector<HTMLElement>('.dropdown-container.select-branch');
  if (!elSelectBranch) return;
  const dropdown = initDropdown(elSelectBranch);
  const urlUpdateIssueRef = elSelectBranch.getAttribute('data-url-update-issueref');
  if (!urlUpdateIssueRef) return;
  dropdown.valueInput!.addEventListener('change', async () => {
    try {
      await POST(urlUpdateIssueRef, {data: new URLSearchParams({ref: dropdown.value})});
      window.location.reload();
    } catch (error) {
      console.error(error);
    }
  });
}

function initRepoIssueDue(elSidebar: HTMLElement) {
  const form = elSidebar.querySelector<HTMLFormElement>('.issue-due-form');
  if (!form) return;
  const deadline = form.querySelector<HTMLInputElement>('input[name=deadline]')!;
  elSidebar.querySelector('.issue-due-edit')?.addEventListener('click', () => {
    toggleElem(form);
  });
  elSidebar.querySelector('.issue-due-remove')?.addEventListener('click', () => {
    deadline.value = '';
    form.dispatchEvent(new Event('submit', {cancelable: true, bubbles: true}));
  });
}

export function initRepoIssueSidebarDependency(elSidebar: HTMLElement) {
  const elDropdown = elSidebar.querySelector<HTMLElement>('#new-dependency-drop-list');
  if (!elDropdown) return;

  const issuePageInfo = parseIssuePageInfo();
  const issueSearchUrl = elDropdown.getAttribute('data-issue-cross-repo-search') === 'true' ?
    `${appSubUrl}/issues/search?priority_repo_id=${issuePageInfo.repoId}&type=${issuePageInfo.issueDependencySearchType}` :
    `${issuePageInfo.repoLink}/issues/search?type=${issuePageInfo.issueDependencySearchType}`;
  initDropdown(elDropdown, {
    async loadItems(query, signal) {
      const response = await GET(`${issueSearchUrl}&q=${encodeURIComponent(query)}`, {signal});
      const issues = await response.json() as Issue[];
      return issues.filter((issue) => String(issue.id) !== elDropdown.getAttribute('data-issue-id')).map((issue) => createElementFromHTML(html`
        <div class="dropdown-item tw-flex-col tw-items-start" data-value="${issue.id}" data-text="#${issue.number} ${issue.title}">
          <div class="gt-ellipsis tw-w-full">#${issue.number} ${issue.title}</div>
          <div class="text small tw-break-anywhere">${issue.repository.full_name}</div>
        </div>
      `));
    },
  });
}

export function initRepoPullRequestAllowMaintainerEdit(elSidebar: HTMLElement) {
  const wrapper = elSidebar.querySelector('#allow-edits-from-maintainers')!;
  if (!wrapper) return;
  const checkbox = wrapper.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
  checkbox.addEventListener('input', async () => {
    const url = `${wrapper.getAttribute('data-url')}/set_allow_maintainer_edit`;
    wrapper.classList.add('is-loading');
    try {
      const resp = await POST(url, {data: new URLSearchParams({allow_maintainer_edit: String(checkbox.checked)})});
      if (!resp.ok) {
        throw new Error('Failed to update maintainer edit permission');
      }
      const data = await resp.json();
      checkbox.checked = data.allow_maintainer_edit;
    } catch (error) {
      checkbox.checked = !checkbox.checked;
      console.error(error);
      showTemporaryTooltip(wrapper, wrapper.getAttribute('data-prompt-error')!);
    } finally {
      wrapper.classList.remove('is-loading');
    }
  });
}

export function initRepoIssueSidebar() {
  registerGlobalInitFunc('initRepoIssueSidebar', (elSidebar) => {
    initRepoIssueBranchSelector(elSidebar);
    initRepoIssueDue(elSidebar);
    initRepoIssueSidebarDependency(elSidebar);
    initRepoPullRequestAllowMaintainerEdit(elSidebar);
    // init the combo list: a dropdown for selecting items, and a list for showing selected items and related actions
    queryElems(elSidebar, '.issue-sidebar-combo', (el) => new IssueSidebarComboList(el).init());
  });
}
