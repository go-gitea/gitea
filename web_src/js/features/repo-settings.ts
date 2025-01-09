import $ from 'jquery';
import {minimatch} from 'minimatch';
import {createMonaco} from './codeeditor.ts';
import {onInputDebounce, queryElems, toggleElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {initRepoSettingsBranchesDrag} from './repo-settings-branches.ts';

const {appSubUrl, csrfToken} = window.config;

function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  for (const dropdownEl of queryElems(document, '.page-content.repository .ui.dropdown.access-mode')) {
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

function initRepoSettingsSearchTeamBox() {
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

function initRepoSettingsGitHook() {
  if (!$('.edit.githook').length) return;
  const filename = document.querySelector('.hook-filename').textContent;
  createMonaco($('#content')[0] as HTMLTextAreaElement, filename, {language: 'shell'});
}

function initRepoSettingsBranches() {
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
  if ($('.repository.settings.options').length > 0) {
    // Enable or select internal/external wiki system and issue tracker.
    $('.enable-system').on('change', function (this: HTMLInputElement) { // eslint-disable-line @typescript-eslint/no-deprecated
      if (this.checked) {
        $($(this).data('target')).removeClass('disabled');
        if (!$(this).data('context')) $($(this).data('context')).addClass('disabled');
      } else {
        $($(this).data('target')).addClass('disabled');
        if (!$(this).data('context')) $($(this).data('context')).removeClass('disabled');
      }
    });
    $('.enable-system-radio').on('change', function (this: HTMLInputElement) { // eslint-disable-line @typescript-eslint/no-deprecated
      if (this.value === 'false') {
        $($(this).data('target')).addClass('disabled');
        if ($(this).data('context') !== undefined) $($(this).data('context')).removeClass('disabled');
      } else if (this.value === 'true') {
        $($(this).data('target')).removeClass('disabled');
        if ($(this).data('context') !== undefined) $($(this).data('context')).addClass('disabled');
      }
    });
    const $trackerIssueStyleRadios = $('.js-tracker-issue-style');
    $trackerIssueStyleRadios.on('change input', () => {
      const checkedVal = $trackerIssueStyleRadios.filter(':checked').val();
      $('#tracker-issue-style-regex-box').toggleClass('disabled', checkedVal !== 'regexp');
    });
  }
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
