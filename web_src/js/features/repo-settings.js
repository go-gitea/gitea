import $ from 'jquery';
import {createMonaco} from './codeeditor.js';
import {initRepoCommonFilterSearchDropdown} from './repo-common.js';

const {appSubUrl, csrfToken, pageData} = window.config;

export function initRepoSettingsCollaboration() {
  // Change collaborator access mode
  $('.access-mode.menu .item').on('click', function () {
    const $menu = $(this).parent();
    $.post($menu.data('url'), {
      _csrf: csrfToken,
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
      url: `${appSubUrl}/api/v1/orgs/${$searchTeamBox.data('org')}/teams/search?q={query}`,
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

export function initRepoSettingsSSHAuthorization() {
  const generateButton = document.querySelector('#generate-ssh-key');
  if (!generateButton) {
    return;
  }
  const deleteSSHButton = document.querySelector('#delete-ssh-key');
  const generateButtonForm = generateButton.closest('form');

  generateButton.addEventListener('click', async () => {
    const resp = await fetch(pageData.GenerateSSHKey, {
      'method': 'GET',
      'cache': 'no-cache',
      'headers': {'X-Csrf-Token': csrfToken},
    });
    const bodyJson = await resp.json();
    if (bodyJson['error']) {
      return;
    }
    document.querySelector('.password-auth').setAttribute('hidden', '');
    document.querySelector('.ssh-auth').removeAttribute('hidden');
    deleteSSHButton.style.display = '';

    document.querySelector('#public-ssh-key-content').textContent = bodyJson['public_ssh_key'];
  });

  deleteSSHButton.addEventListener('click', async () => {
    const resp = await fetch(pageData.GenerateSSHKey.replace('generate_ssh', 'delete_ssh'), {
      'method': 'GET',
      'cache': 'no-cache',
      'headers': {'X-Csrf-Token': csrfToken},
    });
    const bodyJson = await resp.json();
    if (bodyJson['error']) {
      return;
    }
    document.querySelector('.password-auth').removeAttribute('hidden');
    document.querySelector('.ssh-auth').setAttribute('hidden', '');
    deleteSSHButton.style.display = 'none';
  });

  // Avoid that the SSH buttons causes the form to submit.
  generateButtonForm.addEventListener('submit', (ev) => {
    if (ev.submitter.id === 'generate-ssh-key' || ev.submitter.id === 'delete-ssh-key') {
      ev.preventDefault();
      return false;
    }
  });
}
