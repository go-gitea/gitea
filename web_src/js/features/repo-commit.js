import $ from 'jquery';
import {createTippy} from '../modules/tippy.js';
import {toggleElem} from '../utils/dom.js';

const {csrfToken} = window.config;

export function initRepoEllipsisButton() {
  $('.js-toggle-commit-body').on('click', function (e) {
    e.preventDefault();
    const expanded = $(this).attr('aria-expanded') === 'true';
    toggleElem($(this).parent().find('.commit-body'));
    $(this).attr('aria-expanded', String(!expanded));
  });
}

export function initRepoCommitLastCommitLoader() {
  const entryMap = {};

  const entries = $('table#repo-files-table tr.notready')
    .map((_, v) => {
      entryMap[$(v).attr('data-entryname')] = $(v);
      return $(v).attr('data-entryname');
    })
    .get();

  if (entries.length === 0) {
    return;
  }

  const lastCommitLoaderURL = $('table#repo-files-table').data('lastCommitLoaderUrl');

  if (entries.length > 200) {
    $.post(lastCommitLoaderURL, {
      _csrf: csrfToken,
    }, (data) => {
      $('table#repo-files-table').replaceWith(data);
    });
    return;
  }

  $.post(lastCommitLoaderURL, {
    _csrf: csrfToken,
    'f': entries,
  }, (data) => {
    $(data).find('tr').each((_, row) => {
      if (row.className === 'commit-list') {
        $('table#repo-files-table .commit-list').replaceWith(row);
        return;
      }
      // there are other <tr> rows in response (eg: <tr class="has-parent">)
      // at the moment only the "data-entryname" rows should be processed
      const entryName = $(row).attr('data-entryname');
      if (entryName) {
        entryMap[entryName].replaceWith(row);
      }
    });
  });
}

export function initCommitStatuses() {
  $('[data-tippy="commit-statuses"]').each(function () {
    const top = $('.repository.file.list').length > 0 || $('.repository.diff').length > 0;

    createTippy(this, {
      content: this.nextElementSibling,
      placement: top ? 'top-start' : 'bottom-start',
      interactive: true,
      role: 'dialog',
      theme: 'box-with-header',
    });
  });
}
