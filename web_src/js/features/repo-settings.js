import {createMonaco} from './codeeditor.js';
import {initRepoCommonFilterSearchDropdown} from './repo-common.js';

const {AppSubUrl, csrf} = window.config;

export function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  $('.access-mode.menu .item').on('click', function () {
    const $menu = $(this).parent();
    $.post($menu.data('url'), {
      _csrf: csrf,
      uid: $menu.data('uid'),
      mode: $(this).data('value')
    });
  });
}

export function initRepoSettingSearchTeamBox() {
  const $searchTeamBox = $('#search-team-box');
  $searchTeamBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${AppSubUrl}/api/v1/orgs/${$searchTeamBox.data('org')}/teams/search?q={query}`,
      headers: {'X-Csrf-Token': csrf},
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


export async function initRepoSettingGitHook() {
  if ($('.edit.githook').length === 0) return;
  const filename = document.querySelector('.hook-filename').textContent;
  await createMonaco($('#content')[0], filename, {language: 'shell'});
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
