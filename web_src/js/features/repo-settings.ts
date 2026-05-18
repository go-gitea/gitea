import {createCodeEditor} from '../modules/codeeditor/main.ts';
import {onInputDebounce, queryElems, toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {attachSearchBox} from '../modules/search.ts';
import {globMatch} from '../utils/glob.ts';

const {appSubUrl} = window.config;

const projectsModeAllowedScopes: Record<string, Set<string>> = {
  repo: new Set(['repo']),
  owner: new Set(['owner']),
  all: new Set(['repo', 'owner']),
  none: new Set<string>(),
};

function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  for (const dropdownEl of queryElems(document, '.page-content.repository .ui.dropdown.access-mode')) {
    const textEl = dropdownEl.querySelector(':scope > .text')!;
    const $dropdown = fomanticQuery(dropdownEl);
    $dropdown.dropdown({
      async action(text: string, value: string) {
        dropdownEl.classList.add('is-loading', 'loading-icon-2px');
        const lastValue = dropdownEl.getAttribute('data-last-value')!;
        $dropdown.dropdown('hide');
        try {
          const uid = dropdownEl.getAttribute('data-uid')!;
          await POST(dropdownEl.getAttribute('data-url')!, {data: new URLSearchParams({uid, 'mode': value})});
          textEl.textContent = text;
          dropdownEl.setAttribute('data-last-value', value);
        } catch {
          textEl.textContent = '(error)'; // prevent from misleading users when error occurs
          dropdownEl.setAttribute('data-last-value', lastValue);
        } finally {
          dropdownEl.classList.remove('is-loading');
        }
      },
      onHide() {
        // set to the really selected value, defer to next tick to make sure `action` has finished
        // its work because the calling order might be onHide -> action
        setTimeout(() => {
          const $item = $dropdown.dropdown('get item', dropdownEl.getAttribute('data-last-value'));
          if ($item) {
            $dropdown.dropdown('set selected', dropdownEl.getAttribute('data-last-value'));
          } else {
            textEl.textContent = '(none)'; // prevent from misleading users when the access mode is undefined
          }
        }, 0);
      },
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

function initRepoSettingsGitHook() {
  if (!document.querySelector('.page-content.repository.settings.edit.githook')) return;
  createCodeEditor(document.querySelector<HTMLTextAreaElement>('#content')!);
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

  initDefaultProjectScopeFilter(pageContent);
}

export function initDefaultProjectScopeFilter(pageContent: Element) {
  const modeSelect = pageContent.querySelector<HTMLSelectElement>('select[name="projects_mode"]');
  if (!modeSelect) return;

  const dropdowns = pageContent.querySelectorAll<HTMLSelectElement>(
    'select[name="default_project_id_for_issues"], select[name="default_project_id_for_pull_requests"]',
  );

  const applyFilter = () => {
    // Unknown mode (shouldn't happen) degrades to "show everything" rather than hiding all options.
    const allowed = projectsModeAllowedScopes[modeSelect.value] ?? new Set(['repo', 'owner']);
    for (const select of dropdowns) {
      const fieldEl = select.closest('.field')!;
      let selectedHidden = false;
      for (const opt of select.querySelectorAll<HTMLOptionElement>('option')) {
        const scope = opt.getAttribute('data-project-scope');
        // value-0 ("Don't auto-assign") has no scope and is always shown
        const visible = !scope || allowed.has(scope);
        opt.hidden = !visible;
        const item = fieldEl.querySelector<HTMLElement>(`.menu .item[data-value="${CSS.escape(opt.value)}"]`);
        if (item) item.classList.toggle('tw-hidden', !visible);
        if (!visible && opt.selected) selectedHidden = true;
      }
      if (selectedHidden) {
        select.value = '0';
        const $dropdown = fomanticQuery(fieldEl.querySelector('.ui.dropdown')!);
        $dropdown.dropdown('set selected', '0');
      }
    }
  };

  modeSelect.addEventListener('change', applyFilter);
  // Defer the initial pass so Fomantic has initialized the dropdown modules
  // (set up after initRepoSettings runs); the change listener is immediate.
  setTimeout(applyFilter, 0);
}

export function initRepoSettings() {
  if (!document.querySelector('.page-content.repository.settings')) return;
  initRepoSettingsOptions();
  initRepoSettingsBranches();
  initRepoSettingsCollaboration();
  initRepoSettingsSearchTeamBox();
  initRepoSettingsGitHook();
  initRepoSettingsBranchesDrag();
}
