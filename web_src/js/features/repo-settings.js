import $ from 'jquery';
import {minimatch} from 'minimatch';
import {createMonaco} from './codeeditor.js';
import {onInputDebounce, toggleElem} from '../utils/dom.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  $('.page-content.repository .ui.dropdown.access-mode').each((_, e) => {
    const $dropdown = $(e);
    const $text = $dropdown.find('> .text');
    $dropdown.dropdown({
      action(_text, value) {
        const lastValue = $dropdown.attr('data-last-value');
        $.post($dropdown.attr('data-url'), {
          _csrf: csrfToken,
          uid: $dropdown.attr('data-uid'),
          mode: value,
        }).fail(() => {
          $text.text('(error)'); // prevent from misleading users when error occurs
          $dropdown.attr('data-last-value', lastValue);
        });
        $dropdown.attr('data-last-value', value);
        $dropdown.dropdown('hide');
      },
      onChange(_value, text, _$choice) {
        $text.text(text); // update the text when using keyboard navigating
      },
      onHide() {
        // set to the really selected value, defer to next tick to make sure `action` has finished its work because the calling order might be onHide -> action
        setTimeout(() => {
          const $item = $dropdown.dropdown('get item', $dropdown.attr('data-last-value'));
          if ($item) {
            $dropdown.dropdown('set selected', $dropdown.attr('data-last-value'));
          } else {
            $text.text('(none)'); // prevent from misleading users when the access mode is undefined
          }
        }, 0);
      }
    });
  });
}

export function initRepoSettingSearchTeamBox() {
  const $searchTeamBox = $('#search-team-box');
  $searchTeamBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/org/${$searchTeamBox.attr('data-org-name')}/teams/-/search?q={query}`,
      headers: {'X-Csrf-Token': csrfToken},
      onResponse(response) {
        const items = [];
        $.each(response.data, (_i, item) => {
          const title = `${item.name} (${item.permission} access)`;
          items.push({
            title,
          });
        });

        return {results: items};
      }
    },
    searchFields: ['name', 'description'],
    showNoResults: false
  });
}

export function initRepoSettingGitHook() {
  if ($('.edit.githook').length === 0) return;
  const filename = document.querySelector('.hook-filename').textContent;
  const _promise = createMonaco($('#content')[0], filename, {language: 'shell'});
}

export function initRepoSettingBranches() {
  if (!$('.repository.settings.branches').length) return;
  $('.toggle-target-enabled').on('change', function () {
    const $target = $($(this).attr('data-target'));
    $target.toggleClass('disabled', !this.checked);
  });
  $('.toggle-target-disabled').on('change', function () {
    const $target = $($(this).attr('data-target'));
    if (this.checked) $target.addClass('disabled'); // only disable, do not auto enable
  });

  // show the `Matched` mark for the status checks that match the pattern
  const markMatchedStatusChecks = () => {
    const patterns = (document.getElementById('status_check_contexts').value || '').split(/[\r\n]+/);
    const validPatterns = patterns.map((item) => item.trim()).filter(Boolean);
    const marks = document.getElementsByClassName('status-check-matched-mark');

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
  document.getElementById('status_check_contexts').addEventListener('input', onInputDebounce(markMatchedStatusChecks));
}
