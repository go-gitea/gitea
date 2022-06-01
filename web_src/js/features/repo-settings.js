import $ from 'jquery';
import {createMonaco} from './codeeditor.js';
import {initRepoCommonFilterSearchDropdown} from './repo-common.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  const $dropdown = $('.page-content.repository .ui.dropdown.access-mode');
  const $text = $dropdown.find('> .text');
  $dropdown.dropdown({
    action (_text, value) {
      $.post($dropdown.attr('data-url'), {
        _csrf: csrfToken,
        uid: $dropdown.attr('data-uid'),
        mode: value,
      });
      $dropdown.attr('data-last-value', value);
      $dropdown.dropdown('hide');
    },
    onChange (_value, text, _$choice) {
      $text.text(text); // update the text when using keyboard navigating
    },
    onHide () {
      setTimeout(() => {
        // restore the really selected value, defer to next tick to make sure `action` has finished its work
        $dropdown.dropdown('set selected', $dropdown.attr('data-last-value'));
      }, 0);
    }
  });
}

export function initRepoSettingSearchTeamBox() {
  const $searchTeamBox = $('#search-team-box');
  $searchTeamBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/org/${$searchTeamBox.data('org')}/teams/-/search?q={query}`,
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
  // Branches
  if ($('.repository.settings.branches').length > 0) {
    initRepoCommonFilterSearchDropdown('.protected-branches .dropdown');
    $('.enable-protection, .enable-whitelist, .enable-statuscheck').on('change', function () {
      if (this.checked) {
        $($(this).data('target')).removeClass('disabled');
      } else {
        $($(this).data('target')).addClass('disabled');
      }
    });
    $('.disable-whitelist').on('change', function () {
      if (this.checked) {
        $($(this).data('target')).addClass('disabled');
      }
    });
  }
}
