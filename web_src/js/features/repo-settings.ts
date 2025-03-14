import {minimatch} from 'minimatch';
import {createMonaco} from './codeeditor.ts';
import {onInputDebounce, queryElems, toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {initAvatarUploaderWithCropper} from './comp/Cropper.ts';
import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl, csrfToken} = window.config;

function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  for (const dropdownEl of queryElems(document, '.page-content.repository .ui.dropdown.access-mode')) {
    const textEl = dropdownEl.querySelector(':scope > .text');
    const $dropdown = fomanticQuery(dropdownEl);
    $dropdown.dropdown({
      async action(text: string, value: string) {
        dropdownEl.classList.add('is-loading', 'loading-icon-2px');
        const lastValue = dropdownEl.getAttribute('data-last-value');
        $dropdown.dropdown('hide');
        try {
          const uid = dropdownEl.getAttribute('data-uid');
          await POST(dropdownEl.getAttribute('data-url'), {data: new URLSearchParams({uid, 'mode': value})});
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

function initRepoSettingsSearchTeamBox() {
  const searchTeamBox = document.querySelector('#search-team-box');
  if (!searchTeamBox) return;

  fomanticQuery(searchTeamBox).search({
    minCharacters: 2,
    searchFields: ['name', 'description'],
    showNoResults: false,
    rawResponse: true,
    apiSettings: {
      url: `${appSubUrl}/org/${searchTeamBox.getAttribute('data-org-name')}/teams/-/search?q={query}`,
      headers: {'X-Csrf-Token': csrfToken},
      onResponse(response: any) {
        const items: Array<Record<string, any>> = [];
        for (const item of response.data) {
          items.push({
            title: item.name,
            description: `${item.permission} access`, // TODO: translate this string
          });
        }
        return {results: items};
      },
    },
  });
}

function initRepoSettingsGitHook() {
  if (!document.querySelector('.page-content.repository.settings.edit.githook')) return;
  const filename = document.querySelector('.hook-filename').textContent;
  createMonaco(document.querySelector<HTMLTextAreaElement>('#content'), filename, {language: 'shell'});
}

function initRepoSettingsBranches() {
  if (!document.querySelector('.repository.settings.branches')) return;

  for (const el of document.querySelectorAll<HTMLInputElement>('.toggle-target-enabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target'));
      target?.classList.toggle('disabled', !this.checked);
    });
  }

  for (const el of document.querySelectorAll<HTMLInputElement>('.toggle-target-disabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target'));
      if (this.checked) target?.classList.add('disabled'); // only disable, do not auto enable
    });
  }

  document.querySelector<HTMLInputElement>('#dismiss_stale_approvals')?.addEventListener('change', function () {
    document.querySelector('#ignore_stale_approvals_box')?.classList.toggle('disabled', this.checked);
  });

  // show the `Matched` mark for the status checks that match the pattern
  const markMatchedStatusChecks = () => {
    const patterns = (document.querySelector<HTMLTextAreaElement>('#status_check_contexts').value || '').split(/[\r\n]+/);
    const validPatterns = patterns.map((item) => item.trim()).filter(Boolean);
    const marks = document.querySelectorAll('.status-check-matched-mark');

    for (const el of marks) {
      let matched = false;
      const statusCheck = el.getAttribute('data-status-check');
      for (const pattern of validPatterns) {
        if (minimatch(statusCheck, pattern, {noext: true})) { // https://github.com/go-gitea/gitea/issues/33121 disable extended glob syntax
          matched = true;
          break;
        }
      }
      toggleElem(el, matched);
    }
  };
  markMatchedStatusChecks();
  document.querySelector('#status_check_contexts').addEventListener('input', onInputDebounce(markMatchedStatusChecks));
}

function initRepoSettingsOptions() {
  const pageContent = document.querySelector('.page-content.repository.settings.options');
  if (!pageContent) return;

  const toggleClass = (elems: NodeListOf<Element>, className: string, value: boolean) => {
    for (const el of elems) el.classList.toggle(className, value);
  };

  // Enable or select internal/external wiki system and issue tracker.
  queryElems<HTMLInputElement>(pageContent, '.enable-system', (el) => el.addEventListener('change', () => {
    const elTargets = document.querySelectorAll(el.getAttribute('data-target'));
    const elContexts = document.querySelectorAll(el.getAttribute('data-context'));
    toggleClass(elTargets, 'disabled', !el.checked);
    toggleClass(elContexts, 'disabled', el.checked);
  }));
  queryElems<HTMLInputElement>(pageContent, '.enable-system-radio', (el) => el.addEventListener('change', () => {
    const elTargets = document.querySelectorAll(el.getAttribute('data-target'));
    const elContexts = document.querySelectorAll(el.getAttribute('data-context'));
    toggleClass(elTargets, 'disabled', el.value === 'false');
    toggleClass(elContexts, 'disabled', el.value === 'true');
  }));

  queryElems<HTMLInputElement>(pageContent, '.js-tracker-issue-style', (el) => el.addEventListener('change', () => {
    const checkedVal = el.value;
    pageContent.querySelector('#tracker-issue-style-regex-box').classList.toggle('disabled', checkedVal !== 'regexp');
  }));
}

export function initRepoSettings() {
  if (!document.querySelector('.page-content.repository.settings')) return;
  initRepoSettingsOptions();
  initRepoSettingsBranches();
  initRepoSettingsCollaboration();
  initRepoSettingsSearchTeamBox();
  initRepoSettingsGitHook();
  initRepoSettingsBranchesDrag();

  queryElems(document, '.avatar-file-with-cropper', initAvatarUploaderWithCropper);
}
