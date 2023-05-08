import $ from 'jquery';
import {createMonaco} from './codeeditor.js';

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
  if (!$('.repository.settings.branches').length) return;
  $('.toggle-target-enabled').on('change', function () {
    const $target = $($(this).attr('data-target'));
    $target.toggleClass('disabled', !this.checked);
  });
  $('.toggle-target-disabled').on('change', function () {
    const $target = $($(this).attr('data-target'));
    if (this.checked) $target.addClass('disabled'); // only disable, do not auto enable
  });
}

export function initRepoSettingVariables() {
  $('.show-variable-edit-modal').on('click', function () {
    const target = $(this).attr('data-modal');
    const $modal = $(target);
    // clear input/textarea value
    $modal.find('input[name=name]').val('');
    $modal.find('textarea[name=data]').val('');
    // set dialog header
    const $header = $modal.find('#variable-edit-header');
    $header.text($(this).attr('data-modal-header'));

    if ($(this).attr('data-is-new') === 'false') {
      // edit variable dialog
      const oldName = $(this).attr('data-old-name');
      const oldValue = $(this).attr('data-old-value');
      $modal.find('input[name=name]').val(oldName);
      $modal.find('textarea[name=data]').val(oldValue);
    }

    const url = $(this).attr('data-base-action')
    const commitButton = $modal.find('.actions > .ok.button');
    $(commitButton).on('click', (e) => {
      e.preventDefault();
      $.ajax({
        method: 'POST',
        url: url,
        headers: {
          'X-Csrf-Token': csrfToken,
        },
        data: JSON.stringify({
          name: $modal.find('input[name=name]').val(),
          data: $modal.find('textarea[name=data]').val(),
        }),
        contentType: 'application/json',
      }).done((data) => {
        if (data.redirect) {
          window.location.href = data.redirect;
        } else if (redirect) {
          window.location.href = redirect;
        } else {
          window.location.reload();
        }
      });
    });
  });
}
