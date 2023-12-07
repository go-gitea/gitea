import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';

const {csrfToken} = window.config;

function getArchive($target, url, first) {
  $.ajax({
    url,
    type: 'POST',
    data: {
      _csrf: csrfToken,
    },
    complete(xhr) {
      if (xhr.status === 200) {
        if (!xhr.responseJSON) {
          // XXX Shouldn't happen?
          $target.closest('.dropdown').children('i').removeClass('loading');
          return;
        }

        if (!xhr.responseJSON.complete) {
          $target.closest('.dropdown').children('i').addClass('loading');
          // Wait for only three quarters of a second initially, in case it's
          // quickly archived.
          setTimeout(() => {
            getArchive($target, url, false);
          }, first ? 750 : 2000);
        } else {
          // We don't need to continue checking.
          $target.closest('.dropdown').children('i').removeClass('loading');
          window.location.href = url;
        }
      }
    },
  });
}

export function initRepoArchiveLinks() {
  $('.archive-link').on('click', function (event) {
    event.preventDefault();
    const url = $(this).attr('href');
    if (!url) return;
    getArchive($(event.target), url, true);
  });
}

export function initRepoCloneLink() {
  const $repoCloneSsh = $('#repo-clone-ssh');
  const $repoCloneHttps = $('#repo-clone-https');
  const $inputLink = $('#repo-clone-url');

  if ((!$repoCloneSsh.length && !$repoCloneHttps.length) || !$inputLink.length) {
    return;
  }

  $repoCloneSsh.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'ssh');
    window.updateCloneStates();
  });
  $repoCloneHttps.on('click', () => {
    localStorage.setItem('repo-clone-protocol', 'https');
    window.updateCloneStates();
  });

  $inputLink.on('focus', () => {
    $inputLink.trigger('select');
  });
}

export function initRepoCommonBranchOrTagDropdown(selector) {
  $(selector).each(function () {
    const $dropdown = $(this);
    $dropdown.find('.reference.column').on('click', function () {
      hideElem($dropdown.find('.scrolling.reference-list-menu'));
      showElem($($(this).data('target')));
      return false;
    });
  });
}

export function initRepoCommonFilterSearchDropdown(selector) {
  const $dropdown = $(selector);
  $dropdown.dropdown({
    fullTextSearch: 'exact',
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice.attr('data-url')) {
        window.location.href = $choice.attr('data-url');
      }
    },
    message: {noResults: $dropdown.attr('data-no-results')},
  });
}

const {appSubUrl} = window.config;

export function initRepoCommonForksRepoSearchDropdown(selector) {
  const $dropdown = $(selector);
  $dropdown.find('input').on('input', function() {
    const $root = $(this).closest(selector).find('.reference-list-menu');
    const $query = $(this).val().trim();
    if ($query.length === 0) {
      return;
    }

    $.get(`${appSubUrl}/repo/search?q=${$query}`).done((data) => {
      if (data.ok !== true) {
        return;
      }

      const $linkTmpl = $root.data('url-tmpl');

      for (let i = 0; i < data.data.length; i++) {
        const {id, full_name, link} = data.data[i].repository;

        const found = $root.find('.item').filter(function() {
          return $(this).data('id') === id;
        });

        if (found.length !== 0) {
          continue;
        }

        const compareLink = $linkTmpl.replace('{REPO_LINK}', link).replace('{REOP_FULL_NAME}', full_name);
        $root.append($(`<div class="item" data-id="${id}" data-url="${compareLink}">${full_name}</div>`));
      }
    }).always(() => {
      $root.find('.item').each((_, e) => {
        if (!$(e).html().includes($query)) {
          $(e).addClass('filtered');
        }
      });
    });

    return false;
  });

  $dropdown.dropdown({
    fullTextSearch: 'exact',
    selectOnKeydown: false,
    onChange(_text, _value, $choice) {
      if ($choice.attr('data-url')) {
        window.location.href = $choice.attr('data-url');
      }
    },
    message: {noResults: $dropdown.attr('data-no-results')},
  });

  const $acrossServiceCompareBtn = $('.choose.branch .compare-across-server-btn');
  const $acrossServiceCompareInput = $('.choose.branch .compare-across-server-input');

  if ($acrossServiceCompareBtn.length === 0 || $acrossServiceCompareInput.length === 0) {
    return;
  }

  $acrossServiceCompareBtn.on('click', function(e) {
    e.preventDefault();
    e.stopPropagation();

    window.location.href = $(this).data('compare-url') + encodeURIComponent($acrossServiceCompareInput.val());
  });
}

export function initRepoCommonLanguageStats() {
  // Language stats
  if ($('.language-stats').length > 0) {
    $('.language-stats').on('click', (e) => {
      e.preventDefault();
      toggleElem($('.language-stats-details, .repository-menu'));
    });
  }
}
