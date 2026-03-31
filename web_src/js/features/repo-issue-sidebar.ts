import {POST} from '../modules/fetch.ts';
import {queryElems, toggleElem} from '../utils/dom.ts';
import {IssueSidebarComboList} from './repo-issue-sidebar-combolist.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {parseIssuePageInfo} from '../utils.ts';
import {html} from '../utils/html.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {showTemporaryTooltip} from '../modules/tippy.ts';

const {appSubUrl} = window.config;

function initRepoIssueBranchSelector(elSidebar: HTMLElement) {
  // TODO: RemoveIssueRef: see "repo/issue/branch_selector_field.tmpl"
  const elSelectBranch = elSidebar.querySelector('.ui.dropdown.select-branch.branch-selector-dropdown');
  if (!elSelectBranch) return;
  const urlUpdateIssueRef = elSelectBranch.getAttribute('data-url-update-issueref');
  const elBranchMenu = elSelectBranch.querySelector('.reference-list-menu')!;
  queryElems(elBranchMenu, '.item:not(.no-select)', (el) => el.addEventListener('click', async function (e) {
    e.preventDefault();
    const selectedValue = this.getAttribute('data-id')!; // eg: "refs/heads/my-branch"
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
      const selectedHiddenSelector = this.getAttribute('data-id-selector')!;
      document.querySelector<HTMLInputElement>(selectedHiddenSelector)!.value = selectedValue;
      elSelectBranch.querySelector('.text-branch-name')!.textContent = selectedText;
    }
  }));
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
  const elDropdown = elSidebar.querySelector('#new-dependency-drop-list');
  if (!elDropdown) return;

  const issuePageInfo = parseIssuePageInfo();
  const crossRepoSearch = elDropdown.getAttribute('data-issue-cross-repo-search');
  let issueSearchUrl = `${issuePageInfo.repoLink}/issues/search?q={query}&type=${issuePageInfo.issueDependencySearchType}`;
  if (crossRepoSearch === 'true') {
    issueSearchUrl = `${appSubUrl}/issues/search?q={query}&priority_repo_id=${issuePageInfo.repoId}&type=${issuePageInfo.issueDependencySearchType}`;
  }
  fomanticQuery(elDropdown).dropdown({
    fullTextSearch: true,
    apiSettings: {
      cache: false,
      rawResponse: true,
      url: issueSearchUrl,
      onResponse(response: any) {
        const filteredResponse = {success: true, results: [] as Array<Record<string, any>>};
        const currIssueId = elDropdown.getAttribute('data-issue-id');
        // Parse the response from the api to work with our dropdown
        for (const issue of response) {
          // Don't list current issue in the dependency list.
          if (String(issue.id) === currIssueId) continue;
          filteredResponse.results.push({
            value: issue.id,
            name: html`<div class="gt-ellipsis">#${issue.number} ${issue.title}</div><div class="text small tw-break-anywhere">${issue.repository.full_name}</div>`,
          });
        }
        return filteredResponse;
      },
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
