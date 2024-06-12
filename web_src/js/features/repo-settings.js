import $ from 'jquery';
import {minimatch} from 'minimatch';
import {createMonaco} from './codeeditor.js';
import {onInputDebounce, queryElems, toggleElem} from '../utils/dom.js';
import {POST} from '../modules/fetch.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  for (const dropdownEl of queryElems('.page-content.repository .ui.dropdown.access-mode')) {
    const textEl = dropdownEl.querySelector(':scope > .text');
    $(dropdownEl).dropdown({
      async action(text, value) {
        dropdownEl.classList.add('is-loading', 'loading-icon-2px');
        const lastValue = dropdownEl.getAttribute('data-last-value');
        $(dropdownEl).dropdown('hide');
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
          const $item = $(dropdownEl).dropdown('get item', dropdownEl.getAttribute('data-last-value'));
          if ($item) {
            $(dropdownEl).dropdown('set selected', dropdownEl.getAttribute('data-last-value'));
          } else {
            textEl.textContent = '(none)'; // prevent from misleading users when the access mode is undefined
          }
        }, 0);
      },
    });
  }
}

export function initRepoSettingSearchTeamBox() {
  const searchTeamBox = document.querySelector('#search-team-box');
  if (!searchTeamBox) return;

  $(searchTeamBox).search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/org/${searchTeamBox.getAttribute('data-org-name')}/teams/-/search?q={query}`,
      headers: {'X-Csrf-Token': csrfToken},
      onResponse(response) {
        const items = [];
        $.each(response.data, (_i, item) => {
          items.push({
            title: item.name,
            description: `${item.permission} access`, // TODO: translate this string
          });
        });

        return {results: items};
      },
    },
    searchFields: ['name', 'description'],
    showNoResults: false,
  });
}

export function initRepoSettingGitHook() {
  if (!$('.edit.githook').length) return;
  const filename = document.querySelector('.hook-filename').textContent;
  const _promise = createMonaco($('#content')[0], filename, {language: 'shell'});
}

export function initRepoSettingBranches() {
  if (!document.querySelector('.repository.settings.branches')) return;

  for (const el of document.querySelectorAll('.toggle-target-enabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target'));
      target?.classList.toggle('disabled', !this.checked);
    });
  }

  for (const el of document.querySelectorAll('.toggle-target-disabled')) {
    el.addEventListener('change', function () {
      const target = document.querySelector(this.getAttribute('data-target'));
      if (this.checked) target?.classList.add('disabled'); // only disable, do not auto enable
    });
  }

  document.querySelector('#dismiss_stale_approvals')?.addEventListener('change', function () {
    document.querySelector('#ignore_stale_approvals_box')?.classList.toggle('disabled', this.checked);
  });

  // show the `Matched` mark for the status checks that match the pattern
  const markMatchedStatusChecks = () => {
    const patterns = (document.querySelector('#status_check_contexts').value || '').split(/[\r\n]+/);
    const validPatterns = patterns.map((item) => item.trim()).filter(Boolean);
    const marks = document.querySelectorAll('.status-check-matched-mark');

    for (const el of marks) {
      let matched = false;
      const statusCheck = el.getAttribute('data-status-check');
      for (const pattern of validPatterns) {
        if (minimatch(statusCheck, pattern)) {
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
