import {createCodeEditor} from '../modules/codeeditor/main.ts';
import {addDelegatedEventListener, onInputDebounce, queryElems, toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {initDropdown} from '../modules/dropdown.ts';
import {attachSearchBox} from '../modules/search.ts';
import {globMatch} from '../utils/glob.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

const {appSubUrl} = window.config;

function initRepoSettingsCollaboration() {
  for (const dropdownEl of queryElems(document, '.page-content.repository .dropdown-container.access-mode')) {
    const dropdown = initDropdown(dropdownEl);
    const textEl = dropdown.trigger.querySelector('.dropdown-text')!;
    addDelegatedEventListener(dropdown.popup, 'click', '.dropdown-item', async (item) => {
      dropdown.trigger.disabled = true;
      dropdown.trigger.classList.add('is-loading', 'loading-icon-2px');
      let respOk = false;
      try {
        const uid = dropdownEl.getAttribute('data-uid')!;
        const value = item.getAttribute('data-value')!;
        const resp = await POST(dropdownEl.getAttribute('data-url')!, {data: new URLSearchParams({uid, mode: value})});
        respOk = resp.ok;
        if (respOk) dropdown.setActiveItem(item);
      } finally {
        dropdown.trigger.disabled = false;
        dropdown.trigger.classList.remove('is-loading', 'loading-icon-2px');
        if (!respOk) textEl.textContent = '(error)';
      }
    });
  }
}

type TeamSearchResponse = {data: Array<{name: string; permission: string}>};

function initRepoSettingsSearchTeamBox() {
  const box = document.querySelector<HTMLElement>('#search-team-box');
  if (!box) return;

  const url = `${appSubUrl}/org/${box.getAttribute('data-org-name')}/teams/-/search?q={query}`;
  attachSearchBox(box, url, (response: TeamSearchResponse) => response.data.map((item) => ({
    title: item.name,
    description: `${item.permission} access`, // TODO: translate this string
  })));
}

function initRepoSettingsGitHook(form: HTMLFormElement) {
  createCodeEditor(form.querySelector<HTMLTextAreaElement>('textarea[name=content]')!);
}

function initRepoSettingsBranches() {
  if (!document.querySelector('.repository.settings.branches')) return;

  for (const el of document.querySelectorAll<HTMLInputElement>('.toggle-target-enabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target')!);
      target?.classList.toggle('disabled', !this.checked);
    });
  }

  for (const el of document.querySelectorAll<HTMLInputElement>('.toggle-target-disabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target')!);
      if (this.checked) target?.classList.add('disabled'); // only disable, do not auto enable
    });
  }

  document.querySelector<HTMLInputElement>('#dismiss_stale_approvals')?.addEventListener('change', function () {
    document.querySelector('#ignore_stale_approvals_box')?.classList.toggle('disabled', this.checked);
  });

  // show the `Matched` mark for the status checks that match the pattern
  const markMatchedStatusChecks = () => {
    const patterns = (document.querySelector<HTMLTextAreaElement>('#status_check_contexts')!.value || '').split(/[\r\n]+/);
    const validPatterns = patterns.map((item) => item.trim()).filter(Boolean as unknown as <T>(x: T | boolean) => x is T);
    const marks = document.querySelectorAll('.status-check-matched-mark');

    for (const el of marks) {
      let matched = false;
      const statusCheck = el.getAttribute('data-status-check')!;
      for (const pattern of validPatterns) {
        if (globMatch(statusCheck, pattern, '/')) {
          matched = true;
          break;
        }
      }
      toggleElem(el, matched);
    }
  };
  markMatchedStatusChecks();
  document.querySelector('#status_check_contexts')!.addEventListener('input', onInputDebounce(markMatchedStatusChecks));
}

function initRepoSettingsOptions() {
  const pageContent = document.querySelector('.page-content.repository.settings.options');
  if (!pageContent) return;

  // toggle related panels for the checkbox/radio inputs, the "selector" may not exist
  const toggleTargetContextPanel = (selector: string, enabled: boolean) => {
    if (!selector) return;
    queryElems(document, selector, (el) => el.classList.toggle('disabled', !enabled));
  };
  queryElems<HTMLInputElement>(pageContent, '.enable-system', (el) => el.addEventListener('change', () => {
    toggleTargetContextPanel(el.getAttribute('data-target')!, el.checked);
    toggleTargetContextPanel(el.getAttribute('data-context')!, !el.checked);
  }));
  queryElems<HTMLInputElement>(pageContent, '.enable-system-radio', (el) => el.addEventListener('change', () => {
    toggleTargetContextPanel(el.getAttribute('data-target')!, el.value === 'true');
    toggleTargetContextPanel(el.getAttribute('data-context')!, el.value === 'false');
  }));

  queryElems<HTMLInputElement>(pageContent, '.js-tracker-issue-style', (el) => el.addEventListener('change', () => {
    const checkedVal = el.value;
    pageContent.querySelector('#tracker-issue-style-regex-box')!.classList.toggle('disabled', checkedVal !== 'regexp');
  }));
}

export function initRepoSettings() {
  registerGlobalInitFunc('initRepoSettingsGitHook', initRepoSettingsGitHook);
  if (!document.querySelector('.page-content.repository.settings')) return;
  initRepoSettingsOptions();
  initRepoSettingsBranches();
  initRepoSettingsCollaboration();
  initRepoSettingsSearchTeamBox();
  initRepoSettingsBranchesDrag();
}
